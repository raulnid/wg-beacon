package main

import (
	"log"
	"time"

	"wg-beacon/internal/config"
)

func main() {
	cfg := config.New()

	app, err := NewApp(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize app: %v", err)
	}
	for {
		if err := app.Run(); err != nil {
			log.Println(err)
		}
		time.Sleep(300 * time.Second)
	}
}
