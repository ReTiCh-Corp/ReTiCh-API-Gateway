package proxy

import (
	"io"
	"log"
	"net/http"
)

// doReverseProxy est le helper partagé entre AuthProxy et ServiceProxy.
// Il copie la requête entrante vers targetURL et retransmet la réponse.
func doReverseProxy(targetURL string, w http.ResponseWriter, r *http.Request) {
	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("Failed to create proxy request: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Copier les headers de la requête originale
	for key, values := range r.Header {
		for _, value := range values {
			proxyReq.Header.Add(key, value)
		}
	}

	// Copier les query params
	proxyReq.URL.RawQuery = r.URL.RawQuery

	client := &http.Client{}
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("Proxy request failed: %v", err)
		http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()

	// Copier les headers de la réponse
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
