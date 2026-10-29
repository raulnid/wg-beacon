package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	Endpoint string
}

// loadConfig reads the environment, with an optional .env that never overrides real env vars.
func loadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("loading .env: %w", err)
	}
	cfg := &Config{
		Port:     os.Getenv("PORT"),
		Endpoint: os.Getenv("ENDPOINT"),
	}
	for name, v := range map[string]string{"PORT": cfg.Port, "ENDPOINT": cfg.Endpoint} {
		if v == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	return cfg, nil
}
