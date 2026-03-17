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

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[CORS] %s %s | Origin: %q | Headers: %v", r.Method, r.URL.Path, r.Header.Get("Origin"), r.Header)

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			log.Printf("[CORS] Preflight handled for %s", r.URL.Path)
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
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

	r := mux.NewRouter()
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/ready", readyHandler).Methods("GET")
	r.PathPrefix("/conversations").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO: replace with real authentication
		r.Header.Set("X-User-ID", "11111111-1111-1111-1111-111111111111")
		messagingProxy.ServeHTTP(w, r)
	})

	// CORS configuration
	appEnv := os.Getenv("APP_ENV")
	log.Printf("[ENV] APP_ENV=%q", appEnv)
	var handler http.Handler = r
	if appEnv == "development" {
		log.Println("[ENV] Development mode: CORS accepting all origins")
		handler = corsMiddleware(r)
	} else {
		log.Printf("[ENV] CORS middleware NOT active (APP_ENV=%q, expected \"development\")", appEnv)
	}

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
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
