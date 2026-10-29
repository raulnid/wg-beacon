package main

import (
	"log"
	"time"
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
	for {
		if err := app.Run(); err != nil {
			log.Println(err)
		}
		time.Sleep(cfg.Interval)
	}
}
