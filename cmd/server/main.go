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
	"github.com/retich-corp/api-gateway/internal/devtools"
	"github.com/retich-corp/api-gateway/internal/middleware"
	"github.com/retich-corp/api-gateway/internal/proxy"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Service   string `json:"service"`
	Timestamp string `json:"timestamp"`
}

func main() {
	// Charger le fichier .env si présent (ignore l'erreur si absent)
	_ = godotenv.Load()

	// Charger la configuration
	cfg := config.Load()

	// Initialiser les middlewares
	authMiddleware := middleware.NewAuthMiddleware(cfg.JWTSecret, cfg.JWTIssuer)

	// Initialiser les proxies
	authProxy := proxy.NewAuthProxy(cfg.AuthServiceURL)
	userProxy := proxy.NewServiceProxy(cfg.UserServiceURL)
	messagingProxy := proxy.NewServiceProxy(cfg.MessagingServiceURL)

	// Initialiser les dev tools
	devToolsHandler := devtools.NewDevToolsHandler(cfg.JWTSecret, cfg.JWTIssuer)

	r := mux.NewRouter()

	// Routes publiques (health checks)
	r.HandleFunc("/health", healthHandler).Methods("GET")
	r.HandleFunc("/ready", readyHandler).Methods("GET")

	// Routes de développement (générer des tokens de test)
	r.HandleFunc("/dev/generate-token", devToolsHandler.GenerateToken).Methods("POST")

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

	// Configuration du serveur
	srv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      loggingMiddleware(r),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

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
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
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
