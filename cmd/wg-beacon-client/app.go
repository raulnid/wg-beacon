package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"wg-beacon/internal/beacon"
)

const (
	httpTimeout = 10 * time.Second
	ipifyURL    = "https://api.ipify.org"
)

type App struct {
	cfg    *Config
	keys   beacon.Keys
	client *http.Client
}

func NewApp(cfg *Config) (*App, error) {
	serverPublic, err := beacon.ReadKey("pem/server_public")
	if err != nil {
		return nil, fmt.Errorf("reading server public key: %w", err)
	}
	clientPrivate, err := beacon.ReadKey("pem/client_private")
	if err != nil {
		return nil, fmt.Errorf("reading client private key: %w", err)
	}
	log.Printf("trying to connect to %s:%s", cfg.Host, cfg.Port)
	return &App{
		cfg:    cfg,
		keys:   beacon.Keys{Peer: serverPublic, Private: clientPrivate},
		client: &http.Client{Timeout: httpTimeout},
	}, nil
}

func (a *App) Run() error {
	ip, err := a.fetchPublicIP()
	if err != nil {
		return fmt.Errorf("fetching public IP: %w", err)
	}
	msg, err := beacon.Seal(ip, time.Now(), a.keys)
	if err != nil {
		return err
	}
	if err := a.postIP(msg); err != nil {
		return fmt.Errorf("reporting IP to server: %w", err)
	}
	return nil
}

func (a *App) fetchPublicIP() (string, error) {
	resp, err := a.client.Get(ipifyURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %s", resp.Status)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err != nil {
		return "", err
	}
	ip := strings.TrimSpace(string(raw))
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("invalid IP %q", ip)
	}
	return ip, nil
}

func (a *App) postIP(msg []byte) error {
	url := fmt.Sprintf("http://%s/%s", net.JoinHostPort(a.cfg.Host, a.cfg.Port), a.cfg.Endpoint)
	resp, err := a.client.Post(url, "application/octet-stream", bytes.NewReader(msg))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %s", resp.Status)
	}
	return nil
}
