package proxy

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDoReverseProxy_ProxiesRequestAndResponse(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected method POST, got %s", r.Method)
		}
		if r.URL.Query().Get("q") != "1" {
			t.Fatalf("expected query q=1, got %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-Test") != "abc" {
			t.Fatalf("expected header X-Test=abc, got %s", r.Header.Get("X-Test"))
		}
		b, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("failed to read body: %v", err)
		}
		if string(b) != "ping" {
			t.Fatalf("expected body ping, got %s", string(b))
		}

		w.Header().Set("X-From", "upstream")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("pong"))
	}))
	defer target.Close()

	req := httptest.NewRequest(http.MethodPost, "http://gateway.local/test?q=1", bytes.NewBufferString("ping"))
	req.Header.Set("X-Test", "abc")
	rr := httptest.NewRecorder()

	doReverseProxy(target.URL, rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status 201, got %d", rr.Code)
	}
	if rr.Header().Get("X-From") != "upstream" {
		t.Fatalf("expected X-From=upstream, got %s", rr.Header().Get("X-From"))
	}
	if rr.Body.String() != "pong" {
		t.Fatalf("expected body pong, got %s", rr.Body.String())
	}
}
