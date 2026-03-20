package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/retich-corp/api-gateway/internal/config"
	"github.com/retich-corp/api-gateway/internal/middleware"
	"github.com/retich-corp/api-gateway/internal/proxy"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

const (
	contentTypeHeader = "Content-Type"
	contentTypeJSON   = "application/json"
)

type JWTValidator interface {
	ValidateJWT(next http.Handler) http.Handler
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

func buildRouter(cfg *config.Config, authMiddleware JWTValidator) *mux.Router {
	// Initialiser les proxies
	authProxy := proxy.NewAuthProxy(cfg.AuthServiceURL)
	userProxy := proxy.NewServiceProxy(cfg.UserServiceURL)
	messagingProxy := proxy.NewServiceProxy(cfg.MessagingServiceURL)

	r := mux.NewRouter()

	// CORS middleware
	corsOrigin := resolveCORSOrigin(cfg)
	if corsOrigin != "" {
		r.Use(corsMiddleware(corsOrigin))
		log.Printf("[CORS] Middleware active with origin %q", corsOrigin)
	}

	// Routes publiques (health checks)
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/ready", readyHandler).Methods("GET")

	// Routes auth (publiques, proxy vers auth service)
	authRouter := r.PathPrefix("/api/v1/auth").Subrouter()
	authRouter.HandleFunc("/login", authProxy.HandleLogin).Methods("POST")
	authRouter.HandleFunc("/register", authProxy.HandleRegister).Methods("POST")
	authRouter.HandleFunc("/refresh", authProxy.HandleRefresh).Methods("POST")
	authRouter.HandleFunc("/logout", authProxy.HandleLogout).Methods("POST")

	// Routes user (protégées par JWT)
	userRouter := r.PathPrefix("/api/v1/user").Subrouter()
	userRouter.Use(authMiddleware.ValidateJWT)
	userRouter.PathPrefix("/").HandlerFunc(userProxy.ProxyRequest)

	// Routes messaging (protégées par JWT)
	messagingRouter := r.PathPrefix("/api/v1/messaging").Subrouter()
	messagingRouter.Use(authMiddleware.ValidateJWT)
	messagingRouter.PathPrefix("/").HandlerFunc(messagingProxy.ProxyRequest)

	// Catch-all pour routes inconnues
	r.NotFoundHandler = http.HandlerFunc(notFoundHandler)

	return r
}

func resolveCORSOrigin(cfg *config.Config) string {
	if cfg.AppEnv == "development" {
		log.Println("[ENV] Development mode: CORS accepting all origins")
		return "*"
	}
	if cfg.ClientURL != "" {
		log.Printf("[ENV] Production mode: CORS accepting origin %q", cfg.ClientURL)
		return cfg.ClientURL
	}
	log.Println("[ENV] Production mode: CLIENT_URL not set, CORS middleware not active")
	return ""
}

func newHTTPServer(cfg *config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func main() {
	// Charger le fichier .env si présent (ignore l'erreur si absent)
	_ = godotenv.Load()

	// Charger la configuration
	cfg := config.Load()

	// Initialiser le middleware d'authentification avec JWKS
	authMiddleware, err := middleware.NewAuthMiddlewareJWKS(cfg.JWKSURL, cfg.JWTIssuer)
	if err != nil {
		log.Fatalf("Failed to initialize JWKS: %v", err)
	}
	defer authMiddleware.Close()

	r := buildRouter(cfg, authMiddleware)

	// Configuration du serveur
	srv := newHTTPServer(cfg, loggingMiddleware(r))

	// Démarrage du serveur
	go func() {
		log.Printf("API Gateway starting on port %s", cfg.Port)
		log.Printf("Auth Service: %s", cfg.AuthServiceURL)
		log.Printf("User Service: %s", cfg.UserServiceURL)
		log.Printf("Messaging Service: %s", cfg.MessagingServiceURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// Graceful shutdown
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
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(response)
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set(contentTypeHeader, contentTypeJSON)
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(map[string]string{
		"error": "route not found",
		"path":  r.URL.Path,
	})
}

// Middleware de logging simple
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Masquer les tokens dans les logs
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
			authHeader = "Bearer [REDACTED]"
		}

		log.Printf("[%s] %s %s (Auth: %s)", r.Method, r.URL.Path, r.RemoteAddr, authHeader)

		next.ServeHTTP(w, r)

		log.Printf("[%s] %s completed in %v", r.Method, r.URL.Path, time.Since(start))
	})
}
