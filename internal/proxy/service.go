package proxy

import (
	"net/http"
	"strings"
)

// ServiceProxy gère le proxy vers un microservice générique.
type ServiceProxy struct {
	serviceURL  string
	stripPrefix string
}

// NewServiceProxy crée un nouveau ServiceProxy pointant vers serviceURL.
// stripPrefix est le préfixe gateway à retirer avant de transmettre au service.
func NewServiceProxy(serviceURL, stripPrefix string) *ServiceProxy {
	return &ServiceProxy{serviceURL: serviceURL, stripPrefix: stripPrefix}
}

// ProxyRequest transmet la requête vers le microservice cible
// en retirant le préfixe gateway du chemin et en résolvant "/me" en ID réel.
func (sp *ServiceProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, sp.stripPrefix)
	if userID := r.Header.Get("X-User-ID"); userID != "" {
		path = resolveMe(path, userID)
	}
	target := sp.serviceURL + path
	doReverseProxy(target, w, r)
}

// resolveMe remplace le segment de chemin "/me" par "/{userID}".
// Seul le segment exact "/me" est remplacé (pas "/meeting", "/me-too", etc.).
func resolveMe(path, userID string) string {
	// Fin de path : "/users/me" → "/users/{userID}"
	if strings.HasSuffix(path, "/me") {
		rest := path[:len(path)-3] // tout sauf "/me"
		return rest + "/" + userID
	}
	// Milieu de path : "/users/me/avatar" → "/users/{userID}/avatar"
	if idx := strings.Index(path, "/me/"); idx >= 0 {
		return path[:idx] + "/" + userID + "/" + path[idx+4:]
	}
	return path
}
