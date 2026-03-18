package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

func corsMiddleware(allowedOrigin string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("[CORS] %s %s | Origin: %q", r.Method, r.URL.Path, r.Header.Get("Origin"))

			w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			if allowedOrigin != "*" {
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				log.Printf("[CORS] Preflight handled for %s", r.URL.Path)
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func conversationsHandler(proxy *httputil.ReverseProxy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: replace with real authentication
		r.Header.Set("X-User-ID", "11111111-1111-1111-1111-111111111111")
		proxy.ServeHTTP(w, r)
	}
}

func newRouter(messagingProxy *httputil.ReverseProxy, corsOrigin string) *mux.Router {
	r := mux.NewRouter()

	if corsOrigin != "" {
		r.Use(corsMiddleware(corsOrigin))
		log.Printf("[CORS] Middleware active with origin %q", corsOrigin)
	}

	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/ready", readyHandler).Methods("GET")
	r.PathPrefix("/conversations").HandlerFunc(conversationsHandler(messagingProxy))
	return r
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Messaging service reverse proxy
	messagingURL := os.Getenv("MESSAGING_SERVICE_URL")
	if messagingURL == "" {
		messagingURL = "http://localhost:8082"
	}
	messagingTarget, err := url.Parse(messagingURL)
	if err != nil {
		log.Fatalf("Invalid MESSAGING_SERVICE_URL: %v", err)
	}
	messagingProxy := httputil.NewSingleHostReverseProxy(messagingTarget)
	log.Printf("Messaging proxy configured → %s", messagingURL)

	// CORS configuration
	appEnv := os.Getenv("APP_ENV")
	log.Printf("[ENV] APP_ENV=%q", appEnv)
	var corsOrigin string
	if appEnv == "development" {
		log.Println("[ENV] Development mode: CORS accepting all origins")
		corsOrigin = "*"
	} else {
		clientURL := os.Getenv("CLIENT_URL")
		if clientURL != "" {
			log.Printf("[ENV] Production mode: CORS accepting origin %q", clientURL)
			corsOrigin = clientURL
		} else {
			log.Println("[ENV] Production mode: CLIENT_URL not set, CORS middleware not active")
		}
	}

	r := newRouter(messagingProxy, corsOrigin)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("API Gateway starting on port %s", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited gracefully")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Service:   "api-gateway",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
