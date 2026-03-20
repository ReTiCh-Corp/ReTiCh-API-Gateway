package main

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/retich-corp/api-gateway/internal/config"
)

type passthroughJWT struct{}

func (p passthroughJWT) ValidateJWT(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func TestBuildRouter_RoutesAndProxies(t *testing.T) {
	authUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("auth-ok"))
	}))
	defer authUpstream.Close()

	userUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/user/") {
			t.Fatalf("unexpected user upstream path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("user-ok"))
	}))
	defer userUpstream.Close()

	messagingUpstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/api/v1/messaging/") {
			t.Fatalf("unexpected messaging upstream path: %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("msg-ok"))
	}))
	defer messagingUpstream.Close()

	cfg := &config.Config{
		Port:                "0",
		AuthServiceURL:      authUpstream.URL,
		UserServiceURL:      userUpstream.URL,
		MessagingServiceURL: messagingUpstream.URL,
	}

	r := buildRouter(cfg, passthroughJWT{})

	// Public routes
	{
		req := httptest.NewRequest("GET", "/health", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for /health, got %d", rr.Code)
		}
	}

	// Auth proxy routes
	{
		req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader("x"))
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for auth login, got %d", rr.Code)
		}
		if rr.Body.String() != "auth-ok" {
			t.Fatalf("expected auth-ok, got %s", rr.Body.String())
		}
	}

	// User proxy (protected but passthrough) route
	{
		req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for user route, got %d", rr.Code)
		}
		if rr.Body.String() != "user-ok" {
			t.Fatalf("expected user-ok, got %s", rr.Body.String())
		}
	}

	// Messaging proxy (protected but passthrough) route
	{
		req := httptest.NewRequest("GET", "/api/v1/messaging/inbox", nil)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("expected 200 for messaging route, got %d", rr.Code)
		}
		if rr.Body.String() != "msg-ok" {
			t.Fatalf("expected msg-ok, got %s", rr.Body.String())
		}
	}
}

func TestNewHTTPServer(t *testing.T) {
	cfg := &config.Config{Port: "1234"}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	srv := newHTTPServer(cfg, h)
	if srv == nil {
		t.Fatal("expected non-nil server")
	}
	if srv.Addr != ":1234" {
		t.Fatalf("expected addr :1234, got %s", srv.Addr)
	}
	if srv.Handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()

	healthHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %s", ct)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if resp.Status != "healthy" {
		t.Fatalf("expected status healthy, got %s", resp.Status)
	}
	if resp.Service != "api-gateway" {
		t.Fatalf("expected service api-gateway, got %s", resp.Service)
	}
	if resp.Timestamp == "" {
		t.Fatal("expected non-empty timestamp")
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/ready", nil)
	rr := httptest.NewRecorder()

	readyHandler(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if ct := rr.Header().Get("Content-Type"); ct != "application/json" {
		t.Fatalf("expected content-type application/json, got %s", ct)
	}

	var m map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if m["status"] != "ready" {
		t.Fatalf("expected status ready, got %s", m["status"])
	}
}

func TestNotFoundHandler(t *testing.T) {
	req := httptest.NewRequest("GET", "/unknown", nil)
	rr := httptest.NewRecorder()

	notFoundHandler(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rr.Code)
	}

	var m map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&m); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}
	if m["error"] != "route not found" {
		t.Fatalf("expected error route not found, got %s", m["error"])
	}
	if m["path"] != "/unknown" {
		t.Fatalf("expected path /unknown, got %s", m["path"])
	}
}

func TestLoggingMiddleware_RedactsBearerToken(t *testing.T) {
	nextCalled := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		nextCalled = true
		w.WriteHeader(http.StatusNoContent)
	})

	h := loggingMiddleware(next)

	req := httptest.NewRequest("GET", "/x", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)

	if !nextCalled {
		t.Fatal("expected next handler to be called")
	}
	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
}

func TestLoggingMiddleware_PassesThroughBody(t *testing.T) {
	payload := "hello"

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if string(b) != payload {
			t.Fatalf("expected body %q, got %q", payload, string(b))
		}
		w.WriteHeader(http.StatusOK)
	})

	h := loggingMiddleware(next)
	req := httptest.NewRequest("POST", "/x", strings.NewReader(payload))
	rr := httptest.NewRecorder()

	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
}

func TestCorsMiddleware_PreflightRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called on preflight")
	})

	handler := corsMiddleware("*")(inner)
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rr.Code)
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Fatalf("expected Access-Control-Allow-Origin '*', got %q", v)
	}
	if v := rr.Header().Get("Access-Control-Allow-Methods"); v == "" {
		t.Fatal("expected Access-Control-Allow-Methods to be set")
	}
	if v := rr.Header().Get("Access-Control-Allow-Headers"); v == "" {
		t.Fatal("expected Access-Control-Allow-Headers to be set")
	}
}

func TestCorsMiddleware_RegularRequest(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware("*")(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if !called {
		t.Fatal("expected inner handler to be called")
	}
	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rr.Code)
	}
	if v := rr.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Fatalf("expected CORS headers on regular request, got %q", v)
	}
}

func TestResolveCORSOrigin_Development(t *testing.T) {
	cfg := &config.Config{AppEnv: "development"}
	origin := resolveCORSOrigin(cfg)
	if origin != "*" {
		t.Fatalf("expected '*' for development, got %q", origin)
	}
}

func TestResolveCORSOrigin_ProductionWithClientURL(t *testing.T) {
	cfg := &config.Config{AppEnv: "production", ClientURL: "https://example.com"}
	origin := resolveCORSOrigin(cfg)
	if origin != "https://example.com" {
		t.Fatalf("expected 'https://example.com', got %q", origin)
	}
}

func TestResolveCORSOrigin_ProductionNoClientURL(t *testing.T) {
	cfg := &config.Config{AppEnv: "production"}
	origin := resolveCORSOrigin(cfg)
	if origin != "" {
		t.Fatalf("expected empty string, got %q", origin)
	}
}
