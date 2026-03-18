package proxy

import (
	"net/http"
	"strings"
)

// AuthProxy gère le proxy vers le service d'authentification.
type AuthProxy struct {
	authServiceURL string
}

// NewAuthProxy crée un nouveau AuthProxy pointant vers authServiceURL.
func NewAuthProxy(authServiceURL string) *AuthProxy {
	return &AuthProxy{authServiceURL: authServiceURL}
}

// ProxyRequest transmet la requête telle quelle vers le service auth.
func (ap *AuthProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	target := ap.authServiceURL + r.URL.Path
	doReverseProxy(target, w, r)
}

// HandleLogin proxie vers /api/v1/auth/login.
func (ap *AuthProxy) HandleLogin(w http.ResponseWriter, r *http.Request) {
	target := ap.authServiceURL + "/api/v1/auth/login"
	doReverseProxy(target, w, r)
}

// HandleRegister proxie vers /api/v1/auth/register.
func (ap *AuthProxy) HandleRegister(w http.ResponseWriter, r *http.Request) {
	target := ap.authServiceURL + "/api/v1/auth/register"
	doReverseProxy(target, w, r)
}

// HandleRefresh proxie vers /api/v1/auth/refresh.
func (ap *AuthProxy) HandleRefresh(w http.ResponseWriter, r *http.Request) {
	target := ap.authServiceURL + "/api/v1/auth/refresh"
	doReverseProxy(target, w, r)
}

// HandleLogout proxie vers /api/v1/auth/logout.
func (ap *AuthProxy) HandleLogout(w http.ResponseWriter, r *http.Request) {
	target := ap.authServiceURL + "/api/v1/auth/logout"
	doReverseProxy(target, w, r)
}

// MatchesAuthPath retourne true si le path correspond à une route d'authentification.
func MatchesAuthPath(path string) bool {
	return strings.HasPrefix(path, "/api/v1/auth/")
}
