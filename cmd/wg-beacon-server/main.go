package main

import (
	"log"
)

func main() {
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}

	if err := app.Run(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
