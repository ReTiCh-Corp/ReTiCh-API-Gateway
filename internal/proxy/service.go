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
// en retirant le préfixe gateway du chemin.
func (sp *ServiceProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, sp.stripPrefix)
	target := sp.serviceURL + path
	doReverseProxy(target, w, r)
}
