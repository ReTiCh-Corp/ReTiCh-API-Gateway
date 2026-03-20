package proxy

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNewServiceProxy(t *testing.T) {
	proxy := NewServiceProxy("http://user:8083")
	if proxy.serviceURL != "http://user:8083" {
		t.Errorf("Expected serviceURL to be http://user:8083, got %s", proxy.serviceURL)
	}
}

func TestServiceProxyRequest_Success(t *testing.T) {
	// Mock du service
	mockService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vérifier que les headers sont transmis
		if r.Header.Get("X-User-ID") != "user-123" {
			t.Errorf("Expected X-User-ID to be user-123, got %s", r.Header.Get("X-User-ID"))
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id":"user-123","name":"Alice"}`))
	}))
	defer mockService.Close()

	proxy := NewServiceProxy(mockService.URL)

	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("X-User-Email", "alice@test.com")
	req.Header.Set("Authorization", "Bearer token123")

	rr := httptest.NewRecorder()

	proxy.ProxyRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if !strings.Contains(rr.Body.String(), "Alice") {
		t.Errorf("Expected response to contain Alice, got %s", rr.Body.String())
	}
}

func TestServiceProxyRequest_PreservesHeaders(t *testing.T) {
	mockService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Vérifier tous les headers
		headers := []string{"X-User-ID", "X-User-Email", "X-User-Role", "Authorization"}
		for _, h := range headers {
			if r.Header.Get(h) == "" {
				t.Errorf("Expected header %s to be present", h)
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockService.Close()

	proxy := NewServiceProxy(mockService.URL)

	req := httptest.NewRequest("GET", "/api/v1/user/profile", nil)
	req.Header.Set("X-User-ID", "user-123")
	req.Header.Set("X-User-Email", "test@test.com")
	req.Header.Set("X-User-Role", "admin")
	req.Header.Set("Authorization", "Bearer xyz")

	rr := httptest.NewRecorder()
	proxy.ProxyRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServiceProxyRequest_WithQueryParams(t *testing.T) {
	mockService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.RawQuery != "page=1&limit=10" {
			t.Errorf("Expected query params page=1&limit=10, got %s", r.URL.RawQuery)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer mockService.Close()

	proxy := NewServiceProxy(mockService.URL)

	req := httptest.NewRequest("GET", "/api/v1/user/list?page=1&limit=10", nil)
	rr := httptest.NewRecorder()

	proxy.ProxyRequest(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestServiceProxyRequest_POST(t *testing.T) {
	mockService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("Expected method POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"created":true}`))
	}))
	defer mockService.Close()

	proxy := NewServiceProxy(mockService.URL)

	req := httptest.NewRequest("POST", "/api/v1/user", strings.NewReader(`{"name":"Bob"}`))
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()

	proxy.ProxyRequest(rr, req)

	if rr.Code != http.StatusCreated {
		t.Errorf("Expected status 201, got %d", rr.Code)
	}
}
