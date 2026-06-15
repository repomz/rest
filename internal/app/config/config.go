package config

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	HTTPAddr string
	DB_DSN   string
}

func Read() Config {
	_ = godotenv.Load()

	cfg := Config{
		HTTPAddr: ":8080",
		DB_DSN:   os.Getenv("DB_DSN"),
	}
	if httpAddr := os.Getenv("HTTP_ADDR"); httpAddr != "" {
		cfg.HTTPAddr = httpAddr
	}
	return cfg
}
