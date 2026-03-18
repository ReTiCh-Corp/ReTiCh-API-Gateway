package middleware

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type jwkKey struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use,omitempty"`
	Alg string `json:"alg,omitempty"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type jwkSet struct {
	Keys []jwkKey `json:"keys"`
}

func base64URLUInt(b []byte) string {
	return base64.RawURLEncoding.EncodeToString(b)
}

func newTestJWKS(t *testing.T, kid string) (*rsa.PrivateKey, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate rsa key: %v", err)
	}

	n := base64URLUInt(priv.PublicKey.N.Bytes())
	// exponent: typically 65537
	eBytes := []byte{0x01, 0x00, 0x01}
	e := base64URLUInt(eBytes)

	set := jwkSet{
		Keys: []jwkKey{{
			Kty: "RSA",
			Kid: kid,
			Use: "sig",
			Alg: "RS256",
			N:   n,
			E:   e,
		}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)

	return priv, srv.URL
}

func signRS256Token(t *testing.T, priv *rsa.PrivateKey, kid, issuer, sub, email, role string) string {
	t.Helper()

	claims := jwt.MapClaims{
		"sub":   sub,
		"email": email,
		"role":  role,
		"iss":   issuer,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(15 * time.Minute).Unix(),
	}

	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tok.Header["kid"] = kid

	s, err := tok.SignedString(priv)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return s
}

func TestNewAuthMiddlewareJWKS_InvalidURL(t *testing.T) {
	_, err := NewAuthMiddlewareJWKS("http://127.0.0.1:0/invalid", "issuer")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateJWTJWKS_ValidToken(t *testing.T) {
	kid := "kid-1"
	priv, jwksURL := newTestJWKS(t, kid)

	issuer := "test-issuer"
	am, err := NewAuthMiddlewareJWKS(jwksURL, issuer)
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	token := signRS256Token(t, priv, kid, issuer, "user-123", "test@test.com", "admin")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true

		if r.Header.Get("X-User-ID") != "user-123" {
			t.Errorf("Expected X-User-ID to be user-123, got %s", r.Header.Get("X-User-ID"))
		}
		if r.Header.Get("X-User-Email") != "test@test.com" {
			t.Errorf("Expected X-User-Email to be test@test.com, got %s", r.Header.Get("X-User-Email"))
		}
		if r.Header.Get("X-User-Role") != "admin" {
			t.Errorf("Expected X-User-Role to be admin, got %s", r.Header.Get("X-User-Role"))
		}

		if got := r.Context().Value(UserIDKey); got != "user-123" {
			t.Errorf("Expected UserIDKey to be user-123, got %v", got)
		}
	})

	rr := httptest.NewRecorder()
	am.ValidateJWT(next).ServeHTTP(rr, req)

	if !nextCalled {
		t.Error("Next handler was not called")
	}
	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestValidateJWTJWKS_MissingHeader(t *testing.T) {
	kid := "kid-1"
	_, jwksURL := newTestJWKS(t, kid)

	am, err := NewAuthMiddlewareJWKS(jwksURL, "")
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	am.ValidateJWT(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestValidateJWTJWKS_InvalidHeaderFormat(t *testing.T) {
	kid := "kid-1"
	_, jwksURL := newTestJWKS(t, kid)

	am, err := NewAuthMiddlewareJWKS(jwksURL, "")
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Token abc")
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	am.ValidateJWT(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestValidateJWTJWKS_WrongIssuer(t *testing.T) {
	kid := "kid-1"
	priv, jwksURL := newTestJWKS(t, kid)

	am, err := NewAuthMiddlewareJWKS(jwksURL, "expected")
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	token := signRS256Token(t, priv, kid, "wrong", "user-123", "test@test.com", "admin")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("Next handler should not be called")
	})

	am.ValidateJWT(next).ServeHTTP(rr, req)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", rr.Code)
	}
}

func TestOptionalJWTJWKS_NoHeader_PassesThrough(t *testing.T) {
	kid := "kid-1"
	_, jwksURL := newTestJWKS(t, kid)

	am, err := NewAuthMiddlewareJWKS(jwksURL, "")
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	am.OptionalJWT(next).ServeHTTP(rr, req)
	if !nextCalled {
		t.Error("Next handler was not called")
	}
}

func TestOptionalJWTJWKS_ValidToken_EnrichesRequest(t *testing.T) {
	kid := "kid-1"
	priv, jwksURL := newTestJWKS(t, kid)

	issuer := "test-issuer"
	am, err := NewAuthMiddlewareJWKS(jwksURL, issuer)
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	token := signRS256Token(t, priv, kid, issuer, "user-123", "test@test.com", "admin")

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		if r.Header.Get("X-User-ID") != "user-123" {
			t.Fatalf("expected X-User-ID user-123, got %s", r.Header.Get("X-User-ID"))
		}
		if got := r.Context().Value(UserEmailKey); got != "test@test.com" {
			t.Fatalf("expected context email, got %v", got)
		}
	})

	am.OptionalJWT(next).ServeHTTP(rr, req)
	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
}

func TestGetStringClaim(t *testing.T) {
	claims := jwt.MapClaims{
		"sub":  "user-123",
		"role": 123,
	}

	if got := getStringClaim(claims, "sub"); got != "user-123" {
		t.Fatalf("expected user-123, got %s", got)
	}
	if got := getStringClaim(claims, "role"); got != "" {
		t.Fatalf("expected empty for non-string claim, got %s", got)
	}
	if got := getStringClaim(claims, "missing"); got != "" {
		t.Fatalf("expected empty for missing claim, got %s", got)
	}
}

func TestOptionalJWTJWKS_InvalidHeaderFormat_PassesThrough(t *testing.T) {
	kid := "kid-1"
	_, jwksURL := newTestJWKS(t, kid)

	am, err := NewAuthMiddlewareJWKS(jwksURL, "")
	if err != nil {
		t.Fatalf("failed to init middleware: %v", err)
	}
	defer am.Close()

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Token abc")
	rr := httptest.NewRecorder()

	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
	})

	am.OptionalJWT(next).ServeHTTP(rr, req)
	if !nextCalled {
		t.Error("Next handler was not called")
	}
}
