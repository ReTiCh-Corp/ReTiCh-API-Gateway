package proxy

import (
	"log"
	"net/http"
	"net/url"
	"strings"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // CORS is handled by the gateway middleware
	},
}

// WebSocketProxy proxies WebSocket connections to a backend service.
type WebSocketProxy struct {
	backendURL string
}

// NewWebSocketProxy creates a new WebSocket proxy targeting backendURL.
func NewWebSocketProxy(backendURL string) *WebSocketProxy {
	return &WebSocketProxy{backendURL: backendURL}
}

// ProxyWS upgrades the client connection and dials the backend,
// then pipes frames bidirectionally.
// The X-User-ID header must already be set by the auth middleware.
func (wsp *WebSocketProxy) ProxyWS(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Build backend WS URL
	backendWS := strings.Replace(wsp.backendURL, "http://", "ws://", 1)
	backendWS = strings.Replace(backendWS, "https://", "wss://", 1)

	backendURL, err := url.Parse(backendWS + "/ws")
	if err != nil {
		log.Printf("WS proxy: invalid backend URL: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Forward query params
	backendURL.RawQuery = r.URL.RawQuery

	// Dial backend with X-User-ID header
	backendHeaders := http.Header{}
	backendHeaders.Set("X-User-ID", userID)

	backendConn, resp, err := websocket.DefaultDialer.Dial(backendURL.String(), backendHeaders)
	if err != nil {
		if resp != nil {
			log.Printf("WS proxy: backend dial failed (status %d): %v", resp.StatusCode, err)
		} else {
			log.Printf("WS proxy: backend dial failed: %v", err)
		}
		http.Error(w, "Backend unavailable", http.StatusServiceUnavailable)
		return
	}
	defer backendConn.Close()

	// Upgrade client connection
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WS proxy: client upgrade failed: %v", err)
		return
	}
	defer clientConn.Close()

	log.Printf("WS proxy: connection established for user %s", userID)

	errc := make(chan error, 2)

	// Client → Backend
	go func() {
		for {
			msgType, msg, err := clientConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := backendConn.WriteMessage(msgType, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	// Backend → Client
	go func() {
		for {
			msgType, msg, err := backendConn.ReadMessage()
			if err != nil {
				errc <- err
				return
			}
			if err := clientConn.WriteMessage(msgType, msg); err != nil {
				errc <- err
				return
			}
		}
	}()

	// Wait for either direction to fail
	<-errc
	log.Printf("WS proxy: connection closed for user %s", userID)
}
