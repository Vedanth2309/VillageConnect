package config

import (
	"fmt"
	"os"
	"strings"
)

// Config contains the application configuration for runtime and data services.
type Config struct {
	Port               string
	MongoURI           string
	MongoDatabase      string
	FrontendURL        string
	JWTSecret          string
	ElasticsearchURL   string
	ElasticsearchIndex string
}

func Load() Config {
	return Config{
		Port:               getEnv("PORT", "8081"),
		MongoURI:           getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase:      getEnv("MONGO_DATABASE", "villageconnect"),
		FrontendURL:        getEnv("FRONTEND_URL", "http://localhost:5173"),
		JWTSecret:          getEnv("JWT_SECRET", "villageconnect-dev-secret-change-me"),
		ElasticsearchURL:   getEnv("ELASTICSEARCH_URL", "http://localhost:9200"),
		ElasticsearchIndex: getEnv("ELASTICSEARCH_INDEX", "villageconnect-products"),
	}
}

func (c Config) Validate(mode string) error {
	if c.Port == "" || c.MongoURI == "" || c.MongoDatabase == "" || c.ElasticsearchIndex == "" {
		return fmt.Errorf("PORT, MONGO_URI, MONGO_DATABASE, and ELASTICSEARCH_INDEX must be configured")
	}
	if mode == "release" {
		secret := strings.TrimSpace(c.JWTSecret)
		lowerSecret := strings.ToLower(secret)
		if len(secret) < 32 || strings.Contains(lowerSecret, "change-me") ||
			strings.Contains(lowerSecret, "replace-this") || strings.Contains(lowerSecret, "dev-secret") {
			return fmt.Errorf("JWT_SECRET must be a deployment-specific secret of at least 32 characters in release mode")
		}
	}
	return nil
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
