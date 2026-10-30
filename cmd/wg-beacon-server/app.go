package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"

	"wg-beacon/internal/beacon"
)

const maxBodySize = 256

// runner runs a command and returns its stdout; swapped out in tests.
type runner func(name string, args ...string) ([]byte, error)

func execRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output() //nolint:gosec // args are config values and a validated IP
}

type App struct {
	cfg  *Config
	keys beacon.Keys
	run  runner

	mu   sync.Mutex // guards last and serializes endpoint updates
	last int64      // timestamp of the last accepted message
}

func NewApp(cfg *Config) (*App, error) {
	clientPublic, err := beacon.ReadKey("pem/client_public")
	if err != nil {
		return nil, fmt.Errorf("reading client public key: %w", err)
	}
	serverPrivate, err := beacon.ReadKey("pem/server_private")
	if err != nil {
		return nil, fmt.Errorf("reading server private key: %w", err)
	}
	return &App{
		cfg:  cfg,
		keys: beacon.Keys{Peer: clientPublic, Private: serverPrivate},
		run:  execRunner,
	}, nil
}

func (a *App) routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /"+a.cfg.Endpoint, a.handleUpdateIP)
	return mux
}

func (a *App) handleUpdateIP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodySize))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	ip, ts, err := beacon.Open(body, time.Now(), a.last, a.keys)
	switch {
	case errors.Is(err, beacon.ErrMalformed):
		http.Error(w, "Malformed message", http.StatusBadRequest)
		return
	case err != nil:
		log.Printf("rejected message from %s: %v", r.RemoteAddr, err) //nolint:gosec // RemoteAddr is host:port set by net/http
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	a.last = ts

	if err := a.setEndpoint(ip); err != nil {
		log.Printf("setting endpoint: %v", err)
		http.Error(w, "Failed to update WireGuard endpoint", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// setEndpoint points the configured peer at ip:PEER_PORT, doing nothing if it already is.
func (a *App) setEndpoint(ip string) error {
	ep := net.JoinHostPort(ip, a.cfg.PeerPort)
	out, err := a.run("wg", "show", a.cfg.Interface, "endpoints")
	if err != nil {
		return fmt.Errorf("wg show %s endpoints: %w", a.cfg.Interface, err)
	}

	found := false
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Fields(line)
		if len(f) == 2 && f[0] == a.cfg.PeerPublicKey {
			if f[1] == ep {
				return nil
			}
			found = true
			break
		}
	}
	// wg set would silently create a missing peer.
	if !found {
		return fmt.Errorf("peer %s is not configured on %s", a.cfg.PeerPublicKey, a.cfg.Interface)
	}

	if _, err := a.run("wg", "set", a.cfg.Interface, "peer", a.cfg.PeerPublicKey, "endpoint", ep); err != nil {
		return fmt.Errorf("wg set: %w", err)
	}
	log.Printf("endpoint of %s set to %s", a.cfg.PeerPublicKey, ep)
	return nil
}

func (a *App) Run() error {
	server := &http.Server{
		Addr:              ":" + a.cfg.Port,
		Handler:           a.routes(),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	log.Printf("listening on %s", server.Addr)
	return server.ListenAndServe()
}
