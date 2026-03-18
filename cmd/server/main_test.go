package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	healthHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "healthy" {
		t.Errorf("expected status 'healthy', got %q", resp.Status)
	}
	if resp.Service != "api-gateway" {
		t.Errorf("expected service 'api-gateway', got %q", resp.Service)
	}
	if resp.Timestamp == "" {
		t.Error("expected non-empty timestamp")
	}
}

func TestReadyHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	readyHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp["status"] != "ready" {
		t.Errorf("expected status 'ready', got %q", resp["status"])
	}
}

func TestCorsMiddleware_PreflightRequest(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("inner handler should not be called on preflight")
	})

	handler := corsMiddleware(inner)
	req := httptest.NewRequest(http.MethodOptions, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", rec.Code)
	}

	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Errorf("expected Access-Control-Allow-Origin '*', got %q", v)
	}
	if v := rec.Header().Get("Access-Control-Allow-Methods"); v == "" {
		t.Error("expected Access-Control-Allow-Methods to be set")
	}
	if v := rec.Header().Get("Access-Control-Allow-Headers"); v == "" {
		t.Error("expected Access-Control-Allow-Headers to be set")
	}
}

func TestCorsMiddleware_RegularRequest(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Origin", "http://localhost:3000")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Errorf("expected CORS headers on regular request, got %q", v)
	}
}

func TestCorsMiddleware_NoOrigin(t *testing.T) {
	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := corsMiddleware(inner)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if !called {
		t.Error("expected inner handler to be called even without Origin")
	}
	if v := rec.Header().Get("Access-Control-Allow-Origin"); v != "*" {
		t.Errorf("expected CORS headers even without Origin, got %q", v)
	}
}

func TestConversationsHandler_SetsUserIDHeader(t *testing.T) {
	// Create a backend server to act as the messaging service
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID := r.Header.Get("X-User-ID")
		if userID != "11111111-1111-1111-1111-111111111111" {
			t.Errorf("expected X-User-ID header, got %q", userID)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	handler := conversationsHandler(proxy)
	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestConversationsHandler_PostRequest(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)

	handler := conversationsHandler(proxy)
	req := httptest.NewRequest(http.MethodPost, "/conversations", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", rec.Code)
	}
}

func TestNewRouter_HealthRoute(t *testing.T) {
	backendURL, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}

	var resp HealthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("expected 'healthy', got %q", resp.Status)
	}
}

func TestNewRouter_ReadyRoute(t *testing.T) {
	backendURL, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodGet, "/ready", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestNewRouter_ConversationsRoute(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodGet, "/conversations", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rec.Code)
	}
}

func TestNewRouter_ConversationsSubpath(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer backend.Close()

	backendURL, _ := url.Parse(backend.URL)
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodGet, "/conversations/123/messages", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 for subpath, got %d", rec.Code)
	}
}

func TestNewRouter_UnknownRoute(t *testing.T) {
	backendURL, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodGet, "/unknown", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 or 405 for unknown route, got %d", rec.Code)
	}
}

func TestHealthHandler_MethodNotAllowed(t *testing.T) {
	backendURL, _ := url.Parse("http://localhost:9999")
	proxy := httputil.NewSingleHostReverseProxy(backendURL)
	router := newRouter(proxy)

	req := httptest.NewRequest(http.MethodPost, "/health", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", rec.Code)
	}
}
