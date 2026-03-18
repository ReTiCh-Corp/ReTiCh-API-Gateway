package config

import (
	"log"
	"os"
)

type Config struct {
	Port                string
	JWKSURL             string
	JWTIssuer           string
	AuthServiceURL      string
	UserServiceURL      string
	MessagingServiceURL string
	NatsURL             string
	RedisURL            string
	LogLevel            string
}

func Load() *Config {
	jwksURL := os.Getenv("JWKS_URL")
	if jwksURL == "" {
		log.Fatal("JWKS_URL environment variable is required")
	}

	authServiceURL := os.Getenv("AUTH_SERVICE_URL")
	if authServiceURL == "" {
		authServiceURL = "http://auth:8081"
	}

	return &Config{
		Port:                getEnv("PORT", "8080"),
		JWKSURL:             jwksURL,
		JWTIssuer:           getEnv("JWT_ISSUER", ""),
		AuthServiceURL:      authServiceURL,
		UserServiceURL:      getEnv("USER_SERVICE_URL", "http://user:8083"),
		MessagingServiceURL: getEnv("MESSAGING_SERVICE_URL", "http://messaging:8082"),
		NatsURL:             getEnv("NATS_URL", "nats://nats:4222"),
		RedisURL:            getEnv("REDIS_URL", "redis:6379"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
