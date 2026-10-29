package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/nacl/box"
)

const (
	httpTimeout = 10 * time.Second
	ipifyURL    = "https://api.ipify.org"
)

type App struct {
	cfg    *Config
	peer   [32]byte
	priv   [32]byte
	client *http.Client
}

func NewApp(cfg *Config) (*App, error) {
	serverPublic, err := readKey("pem/server_public")
	if err != nil {
		return nil, fmt.Errorf("reading server public key: %w", err)
	}
	clientPrivate, err := readKey("pem/client_private")
	if err != nil {
		return nil, fmt.Errorf("reading client private key: %w", err)
	}
	log.Printf("trying to connect to %s:%s", cfg.Host, cfg.Port)
	return &App{
		cfg:    cfg,
		peer:   serverPublic,
		priv:   clientPrivate,
		client: &http.Client{Timeout: httpTimeout},
	}, nil
}

func readKey(path string) ([32]byte, error) {
	var key [32]byte
	raw, err := os.ReadFile(path)
	if err != nil {
		return key, err
	}
	decoded, err := hex.DecodeString(strings.TrimSpace(string(raw)))
	if err != nil {
		return key, err
	}
	if len(decoded) != 32 {
		return key, fmt.Errorf("key at %s must be 32 bytes, got %d", path, len(decoded))
	}
	copy(key[:], decoded)
	return key, nil
}

func (a *App) Run() error {
	ip, err := a.fetchPublicIP()
	if err != nil {
		return fmt.Errorf("fetching public IP: %w", err)
	}
	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return fmt.Errorf("generating nonce: %w", err)
	}
	msg := box.Seal(nonce[:], []byte(ip), &nonce, &a.peer, &a.priv)
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
