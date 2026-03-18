package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewAuthProxy(t *testing.T) {
	proxy := NewAuthProxy("http://auth:8081")
	if proxy.authServiceURL != "http://auth:8081" {
		t.Errorf("Expected authServiceURL to be http://auth:8081, got %s", proxy.authServiceURL)
	}
}

func TestProxyRequest_Success(t *testing.T) {
	// Mock du service auth
	mockAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/auth/login" {
			t.Errorf("Expected path /api/v1/auth/login, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"token":"test-token"}`))
	}))
	defer mockAuth.Close()

	proxy := NewAuthProxy(mockAuth.URL)

	req := httptest.NewRequest("POST", "/api/v1/auth/login", strings.NewReader(`{"email":"test@test.com"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	proxy.ProxyRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "test-token") {
		t.Errorf("Expected response to contain test-token, got %s", rr.Body.String())
	}
}

func TestHandleLogin(t *testing.T) {
	mockAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"access_token":"xyz"}`))
	}))
	defer mockAuth.Close()

	proxy := NewAuthProxy(mockAuth.URL)
	req := httptest.NewRequest("POST", "/api/v1/auth/login", nil)
	rr := httptest.NewRecorder()

	proxy.HandleLogin(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHandleRefresh(t *testing.T) {
	mockAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockAuth.Close()

	proxy := NewAuthProxy(mockAuth.URL)
	req := httptest.NewRequest("POST", "/api/v1/auth/refresh", nil)
	rr := httptest.NewRecorder()

	proxy.HandleRefresh(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHandleLogout(t *testing.T) {
	mockAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockAuth.Close()

	proxy := NewAuthProxy(mockAuth.URL)
	req := httptest.NewRequest("POST", "/api/v1/auth/logout", nil)
	rr := httptest.NewRecorder()

	proxy.HandleLogout(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestHandleRegister(t *testing.T) {
	mockAuth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))
	defer mockAuth.Close()

	proxy := NewAuthProxy(mockAuth.URL)
	req := httptest.NewRequest("POST", "/api/v1/auth/register", nil)
	rr := httptest.NewRecorder()

	proxy.HandleRegister(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
}

func TestMatchesAuthPath(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/api/v1/auth/login", true},
		{"/api/v1/auth/register", true},
		{"/api/v1/auth/refresh", true},
		{"/api/v1/user/profile", false},
		{"/health", false},
		{"/api/v1/messaging/chat", false},
	}

	for _, tt := range tests {
		result := MatchesAuthPath(tt.path)
		if result != tt.expected {
			t.Errorf("MatchesAuthPath(%s) = %v, expected %v", tt.path, result, tt.expected)
		}
	}
}
