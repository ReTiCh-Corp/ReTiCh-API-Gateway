package proxy

import (
	"net/http"
)

// ServiceProxy gère le proxy vers un microservice générique.
type ServiceProxy struct {
	serviceURL string
}

// NewServiceProxy crée un nouveau ServiceProxy pointant vers serviceURL.
func NewServiceProxy(serviceURL string) *ServiceProxy {
	return &ServiceProxy{serviceURL: serviceURL}
}

// ProxyRequest transmet la requête vers le microservice cible.
func (sp *ServiceProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	target := sp.serviceURL + r.URL.Path
	doReverseProxy(target, w, r)
}
