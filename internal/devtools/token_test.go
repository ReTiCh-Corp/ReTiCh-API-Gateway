package devtools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateToken_Success(t *testing.T) {
	handler := NewDevToolsHandler("test-secret", "test-issuer")

	reqBody := TokenRequest{
		UserID: "user-123",
		Email:  "test@test.com",
		Role:   "admin",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/dev/generate-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.GenerateToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response TokenResponse
	err := json.NewDecoder(rr.Body).Decode(&response)
	if err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.AccessToken == "" {
		t.Error("Expected access_token to be non-empty")
	}

	if response.RefreshToken == "" {
		t.Error("Expected refresh_token to be non-empty")
	}

	if response.ExpiresIn != 900 {
		t.Errorf("Expected expires_in to be 900, got %d", response.ExpiresIn)
	}

	// Vérifier que le token est valide
	token, err := jwt.Parse(response.AccessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})

	if err != nil {
		t.Errorf("Token should be valid: %v", err)
	}

	if !token.Valid {
		t.Error("Token should be valid")
	}

	claims := token.Claims.(jwt.MapClaims)
	if claims["user_id"] != "user-123" {
		t.Errorf("Expected user_id to be user-123, got %v", claims["user_id"])
	}

	if claims["email"] != "test@test.com" {
		t.Errorf("Expected email to be test@test.com, got %v", claims["email"])
	}

	if claims["role"] != "admin" {
		t.Errorf("Expected role to be admin, got %v", claims["role"])
	}

	if claims["iss"] != "test-issuer" {
		t.Errorf("Expected iss to be test-issuer, got %v", claims["iss"])
	}
}

func TestGenerateToken_DefaultValues(t *testing.T) {
	handler := NewDevToolsHandler("test-secret", "test-issuer")

	// Requête vide, devrait utiliser les valeurs par défaut
	req := httptest.NewRequest("POST", "/dev/generate-token", bytes.NewReader([]byte("{}")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.GenerateToken(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	var response TokenResponse
	json.NewDecoder(rr.Body).Decode(&response)

	token, _ := jwt.Parse(response.AccessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})

	claims := token.Claims.(jwt.MapClaims)

	if claims["user_id"] != "test-user-123" {
		t.Errorf("Expected default user_id to be test-user-123, got %v", claims["user_id"])
	}

	if claims["email"] != "test@retich.com" {
		t.Errorf("Expected default email to be test@retich.com, got %v", claims["email"])
	}

	if claims["role"] != "user" {
		t.Errorf("Expected default role to be user, got %v", claims["role"])
	}
}

func TestGenerateToken_WithAudience(t *testing.T) {
	handler := NewDevToolsHandler("test-secret", "test-issuer")

	reqBody := TokenRequest{
		UserID:   "user-123",
		Email:    "test@test.com",
		Role:     "admin",
		Audience: "shop",
	}
	body, _ := json.Marshal(reqBody)

	req := httptest.NewRequest("POST", "/dev/generate-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.GenerateToken(rr, req)

	var response TokenResponse
	json.NewDecoder(rr.Body).Decode(&response)

	token, _ := jwt.Parse(response.AccessToken, func(token *jwt.Token) (interface{}, error) {
		return []byte("test-secret"), nil
	})

	claims := token.Claims.(jwt.MapClaims)

	if claims["aud"] != "shop" {
		t.Errorf("Expected aud to be shop, got %v", claims["aud"])
	}
}

func TestGenerateToken_InvalidJSON(t *testing.T) {
	handler := NewDevToolsHandler("test-secret", "test-issuer")

	req := httptest.NewRequest("POST", "/dev/generate-token", bytes.NewReader([]byte("invalid-json")))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	handler.GenerateToken(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", rr.Code)
	}
}
