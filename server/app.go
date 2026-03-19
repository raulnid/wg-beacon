package main

import (
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"wg-beacon/internal/config"

	"golang.org/x/crypto/nacl/box"
)

type App struct {
	cfg        *config.Config
	publicKey  [32]byte
	privateKey [32]byte
}

func NewApp(cfg *config.Config) (*App, error) {
	log.Printf("listening on %s", cfg.Port)
	return &App{cfg: cfg}, nil
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

func (a *App) handleUpdateIP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	if len(body) < 24 {
		http.Error(w, "Body too short", http.StatusBadRequest)
		return
	}

	var nonce [24]byte
	copy(nonce[:], body[:24])
	encrypted := body[24:]

	decrypted, ok := box.Open(nil, encrypted, &nonce, &a.publicKey, &a.privateKey)
	if !ok {
		http.Error(w, "Failed to decrypt", http.StatusUnauthorized)
		return
	}

	log.Printf("Decrypted:\n %s\n", decrypted)
	if net.ParseIP(string(decrypted)) == nil {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}
	if err := a.changeWireguardConfig(string(decrypted)); err != nil {
		http.Error(w, "Failed to update Wireguard configuration", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (a *App) changeWireguardConfig(ip string) error {
	conf, err := os.ReadFile("/etc/wireguard/wg0.conf")
	if err != nil {
		log.Printf("failed to read configuration: %v", err)
		return err
	}

	lines := strings.Split(string(conf), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "Endpoint =") {
			parts := strings.Split(line, ":")
			port := parts[len(parts)-1]
			lines[i] = fmt.Sprintf("Endpoint = %s:%s", ip, port)
		}
	}
	newConf := strings.Join(lines, "\n")

	if err := os.WriteFile("/etc/wireguard/wg0.conf", []byte(newConf), 0600); err != nil {
		log.Printf("failed to write configuration: %v", err)
		return err
	}

	if err := exec.Command("wg-quick", "down", "wg0").Run(); err != nil {
		log.Printf("failed to sync configuration: %v", err)
		return err
	}

	if err := exec.Command("wg-quick", "up", "wg0").Run(); err != nil {
		log.Printf("failed to sync configuration: %v", err)
		return err
	}

	return nil
}

func (a *App) Run() error {

	var err error
	a.publicKey, err = readKey("pem/client_public")
	if err != nil {
		return err
	}
	a.privateKey, err = readKey("pem/server_private")
	if err != nil {
		return err
	}

	http.HandleFunc("/"+a.cfg.Endpoint, a.handleUpdateIP)

	return http.ListenAndServe(fmt.Sprintf(":%s", a.cfg.Port), nil)
}
