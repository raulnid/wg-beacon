package config

import (
	"log"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string `env:"PORT"`
	Endpoint string `env:"ENDPOINT"`
}

func New() *Config {
	cfg := &Config{}
	env, err := godotenv.Read()
	if err != nil {
		log.Println("No .env file found or error reading it")
		return cfg
	}
	cfg.Port = env["PORT"]
	cfg.Endpoint = env["ENDPOINT"]
	return cfg
}
