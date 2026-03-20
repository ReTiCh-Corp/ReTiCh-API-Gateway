package config

import (
	"os"
	"testing"
)

func TestLoad_WithJWTSecret(t *testing.T) {
	// Setup
	os.Setenv("JWT_SECRET", "test-secret")
	os.Setenv("PORT", "9090")
	os.Setenv("JWT_ISSUER", "test-issuer")
	defer func() {
		os.Unsetenv("JWT_SECRET")
		os.Unsetenv("PORT")
		os.Unsetenv("JWT_ISSUER")
	}()

	cfg := Load()

	if cfg.JWTSecret != "test-secret" {
		t.Errorf("Expected JWT_SECRET to be test-secret, got %s", cfg.JWTSecret)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected Port to be 9090, got %s", cfg.Port)
	}

	if cfg.JWTIssuer != "test-issuer" {
		t.Errorf("Expected JWT_ISSUER to be test-issuer, got %s", cfg.JWTIssuer)
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	os.Setenv("JWT_SECRET", "required-secret")
	defer os.Unsetenv("JWT_SECRET")

	cfg := Load()

	if cfg.Port != "8080" {
		t.Errorf("Expected default Port to be 8080, got %s", cfg.Port)
	}

	if cfg.JWTIssuer != "retich-auth" {
		t.Errorf("Expected default JWT_ISSUER to be retich-auth, got %s", cfg.JWTIssuer)
	}

	if cfg.AuthServiceURL != "http://auth:8081" {
		t.Errorf("Expected default AuthServiceURL to be http://auth:8081, got %s", cfg.AuthServiceURL)
	}

	if cfg.UserServiceURL != "http://user:8083" {
		t.Errorf("Expected default UserServiceURL to be http://user:8083, got %s", cfg.UserServiceURL)
	}

	if cfg.MessagingServiceURL != "http://messaging:8082" {
		t.Errorf("Expected default MessagingServiceURL to be http://messaging:8082, got %s", cfg.MessagingServiceURL)
	}
}

func TestGetEnv(t *testing.T) {
	// Test avec variable définie
	os.Setenv("TEST_VAR", "test-value")
	defer os.Unsetenv("TEST_VAR")

	result := getEnv("TEST_VAR", "default")
	if result != "test-value" {
		t.Errorf("Expected test-value, got %s", result)
	}

	// Test avec valeur par défaut
	result = getEnv("NON_EXISTENT_VAR", "default-value")
	if result != "default-value" {
		t.Errorf("Expected default-value, got %s", result)
	}
}
