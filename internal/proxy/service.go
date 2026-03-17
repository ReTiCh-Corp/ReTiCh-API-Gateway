package proxy

import (
	"io"
	"log"
	"net/http"
	"net/url"
)

type ServiceProxy struct {
	serviceURL string
	client     *http.Client
}

func NewServiceProxy(serviceURL string) *ServiceProxy {
	return &ServiceProxy{
		serviceURL: serviceURL,
		client:     &http.Client{},
	}
}

// ProxyRequest proxie les requêtes vers un microservice
func (sp *ServiceProxy) ProxyRequest(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(sp.serviceURL)
	if err != nil {
		log.Printf("Error parsing service URL: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	targetURL.Path = r.URL.Path
	targetURL.RawQuery = r.URL.RawQuery

	proxyReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		log.Printf("Error creating proxy request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Copier tous les headers (incluant X-User-ID, X-User-Email, etc.)
	for name, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(name, value)
		}
	}

	if clientIP := r.RemoteAddr; clientIP != "" {
		proxyReq.Header.Set("X-Forwarded-For", clientIP)
	}

	resp, err := sp.client.Do(proxyReq)
	if err != nil {
		log.Printf("Error proxying to service: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	for name, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
