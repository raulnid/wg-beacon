package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Host     string
	Port     string
	Endpoint string
	Interval time.Duration
}

// loadConfig reads the environment, with an optional .env that never overrides real env vars.
func loadConfig() (*Config, error) {
	if err := godotenv.Load(); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("loading .env: %w", err)
	}
	cfg := &Config{
		Host:     os.Getenv("HOST"),
		Port:     os.Getenv("PORT"),
		Endpoint: os.Getenv("ENDPOINT"),
		Interval: 5 * time.Minute,
	}
	for name, v := range map[string]string{"HOST": cfg.Host, "PORT": cfg.Port, "ENDPOINT": cfg.Endpoint} {
		if v == "" {
			return nil, fmt.Errorf("%s is required", name)
		}
	}
	if s := os.Getenv("INTERVAL"); s != "" {
		d, err := time.ParseDuration(s)
		if err != nil || d <= 0 {
			return nil, fmt.Errorf("INTERVAL must be a positive duration, got %q", s)
		}
		cfg.Interval = d
	}
	return cfg, nil
}
