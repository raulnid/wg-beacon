package main

import (
	"log"

	"wg-beacon/internal/config"
)

func main() {
	cfg := config.New()

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
