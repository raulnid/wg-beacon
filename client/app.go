package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"wg-beacon/internal/config"

	"golang.org/x/crypto/nacl/box"
)

type App struct {
	cfg *config.Config
}

func NewApp(cfg *config.Config) (*App, error) {
	log.Printf("trying to connect to %s:%s", cfg.Host, cfg.Port)
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

func (a *App) Run() error {

	publicKey, err := readKey("pem/server_public")
	if err != nil {
		return err
	}
	privateKey, err := readKey("pem/client_private")
	if err != nil {
		return err
	}

	resp, err := http.Get("https://api.ipify.org")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	message, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var nonce [24]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return err
	}

	encrypted := box.Seal(nonce[:], message, &nonce, &publicKey, &privateKey)

	resp, err = http.Post(fmt.Sprintf("http://%s:%s/%s", a.cfg.Host, a.cfg.Port, a.cfg.Endpoint), "application/octet-stream", bytes.NewBuffer(encrypted))
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()

	return nil
}
