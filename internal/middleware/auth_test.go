package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidateJWT_ValidToken(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"
	am := NewAuthMiddleware(secret, issuer)

	// Créer un token valide
	token := createTestToken(secret, issuer, "user-123", "test@test.com", "admin")

	// Créer une requête avec le token
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	// Handler de test
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		// Vérifier que les headers ont été ajoutés
		if r.Header.Get("X-User-ID") != "user-123" {
			t.Errorf("Expected X-User-ID to be user-123, got %s", r.Header.Get("X-User-ID"))
		}
		if r.Header.Get("X-User-Email") != "test@test.com" {
			t.Errorf("Expected X-User-Email to be test@test.com, got %s", r.Header.Get("X-User-Email"))
		}
		if r.Header.Get("X-User-Role") != "admin" {
			t.Errorf("Expected X-User-Role to be admin, got %s", r.Header.Get("X-User-Role"))
		}
	})

	rr := httptest.NewRecorder()
	handler := am.ValidateJWT(next)
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler was not called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestValidateJWT_NoToken(t *testing.T) {
	am := NewAuthMiddleware("secret", "issuer")

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	handler := am.ValidateJWT(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	am := NewAuthMiddleware("secret", "issuer")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-token")
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	handler := am.ValidateJWT(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestValidateJWT_WrongIssuer(t *testing.T) {
	secret := "test-secret"
	am := NewAuthMiddleware(secret, "expected-issuer")

	// Token avec mauvais issuer
	token := createTestToken(secret, "wrong-issuer", "user-123", "test@test.com", "admin")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	handler := am.ValidateJWT(next)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestOptionalJWT_WithValidToken(t *testing.T) {
	secret := "test-secret"
	issuer := "test-issuer"
	am := NewAuthMiddleware(secret, issuer)

	token := createTestToken(secret, issuer, "user-123", "test@test.com", "admin")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if r.Header.Get("X-User-ID") != "user-123" {
			t.Errorf("Expected X-User-ID to be user-123, got %s", r.Header.Get("X-User-ID"))
		}
	})

	rr := httptest.NewRecorder()
	handler := am.OptionalJWT(next)
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler was not called")
	}
}

func TestOptionalJWT_NoToken(t *testing.T) {
	am := NewAuthMiddleware("secret", "issuer")

	req := httptest.NewRequest("GET", "/test", nil)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	rr := httptest.NewRecorder()
	handler := am.OptionalJWT(next)
	handler.ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler was not called")
	}

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

// Helper pour créer un token de test
func createTestToken(secret, issuer, userID, email, role string) string {
	claims := jwt.MapClaims{
		"user_id": userID,
		"email":   email,
		"role":    role,
		"iss":     issuer,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(15 * time.Minute).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, _ := token.SignedString([]byte(secret))
	return tokenString
}
