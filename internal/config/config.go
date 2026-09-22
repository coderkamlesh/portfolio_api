package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	TursoURL   string
	TursoToken string
	ServerPort string
	JWTSecret  string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}

	cfg := &Config{
		TursoURL:   os.Getenv("TURSO_DATABASE_URL"),
		TursoToken: os.Getenv("TURSO_AUTH_TOKEN"),
		ServerPort: getEnv("SERVER_PORT", "8080"),
		JWTSecret:  os.Getenv("JWT_SECRET"),
	}

	if cfg.TursoURL == "" || cfg.TursoToken == "" {
		log.Fatal("TURSO_DATABASE_URL and TURSO_AUTH_TOKEN must be set")
	}
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET must be set")
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
