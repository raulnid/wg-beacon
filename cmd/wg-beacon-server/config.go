package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port          string
	Endpoint      string
	Interface     string
	PeerPublicKey string
	PeerPort      string
}

// loadConfig reads the environment, with an optional .env that never overrides real env vars.
func loadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("loading .env: %w", err)
	}
	cfg := &Config{
		Port:          os.Getenv("PORT"),
		Endpoint:      os.Getenv("ENDPOINT"),
		Interface:     os.Getenv("WG_INTERFACE"),
		PeerPublicKey: os.Getenv("PEER_PUBLIC_KEY"),
		PeerPort:      os.Getenv("PEER_PORT"),
	}
	if cfg.Interface == "" {
		cfg.Interface = "wg0"
	}
	for name, v := range map[string]string{
		"PORT":            cfg.Port,
		"ENDPOINT":        cfg.Endpoint,
		"PEER_PUBLIC_KEY": cfg.PeerPublicKey,
		"PEER_PORT":       cfg.PeerPort,
	} {
		if v == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	return cfg, nil
}
