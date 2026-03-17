package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

type AuthProxy struct {
	authServiceURL string
	client         *http.Client
}

func NewAuthProxy(authServiceURL string) *AuthProxy {
	return &AuthProxy{
		authServiceURL: authServiceURL,
		client:         &http.Client{},
	}
}

// ProxyRequest proxie les requêtes vers le service auth
func (ap *AuthProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	// Construire l'URL cible
	targetURL, err := url.Parse(ap.authServiceURL)
	if err != nil {
		log.Printf("Error parsing auth service URL: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Garder le path original (ex: /api/v1/auth/login)
	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	// Créer la nouvelle requête
	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Copier les headers
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	// Ajouter l'IP originale
	if clientIP := r.RemoteAddr; clientIP != "" {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	// Envoyer la requête
	resp, err := ap.client.Do(proxyReq)
	if err != nil {
		log.Printf("Error proxying to auth service: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Copier les headers de réponse
	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	// Copier le status code
	w.WriteHeader(resp.StatusCode)

	// Copier le body
	io.Copy(w, resp.Body)
}

// HandleLogin proxie la requête de login
func (ap *AuthProxy) HandleLogin(w http.ResponseWriter, r *http.Request) {
	ap.ProxyRequest(w, r)
}

// HandleRefresh proxie la requête de refresh token
func (ap *AuthProxy) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	ap.ProxyRequest(w, r)
}

// HandleLogout proxie la requête de logout
func (ap *AuthProxy) HandleLogout(w http.ResponseWriter, r *http.Request) {
	ap.ProxyRequest(w, r)
}

// HandleRegister proxie la requête d'inscription
func (ap *AuthProxy) HandleRegister(w http.ResponseWriter, r *http.Request) {
	ap.ProxyRequest(w, r)
}

// MatchesAuthPath vérifie si le path correspond aux routes auth
func MatchesAuthPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth/")
}
