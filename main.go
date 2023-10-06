package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
)

type ModifyResponseWriter struct {
	http.ResponseWriter
}

//optional :  modify in response part 
func (w *ModifyResponseWriter) Write(b []byte) (int, error) {
	// Add a custom response header
	w.Header().Set("X-Custom-Response-Header", "Modified-Response")

	// Modify the response body for HTML content
	contentType := w.Header().Get("Content-Type")
	if strings.Contains(contentType, "text/html") {
		originalBody := string(b)

		// Inject a JavaScript script to alert a message
		modifiedBody := fmt.Sprintf(`%s<script>alert('Modified Message');</script>`, originalBody)

		// Write the modified response body
		return w.ResponseWriter.Write([]byte(modifiedBody))
	}

	// For non-HTML content, write the original response body
	return w.ResponseWriter.Write(b)
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	targetURL := r.Header.Get("Target-URL")
	if targetURL == "" {
		http.Error(w, "There is no Target-Endpoint header in the request", http.StatusInternalServerError)
		return
	}

	// Create a reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(&url.URL{
		Scheme: "http", // You can modify this based on your needs
		Host:   targetURL,
	})

	// Update headers to allow CORS
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, PUT, PATCH, POST, DELETE")
	w.Header().Set("Access-Control-Allow-Headers", r.Header.Get("access-control-request-headers"))

	// Create a custom response writer for potential modification
	modifyResponseWriter := &ModifyResponseWriter{ResponseWriter: w}

	// Handle the actual proxying of the request
	proxy.ServeHTTP(modifyResponseWriter, r)
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	http.HandleFunc("/", handleProxy)

	server := &http.Server{
		Addr: ":" + port,
	}

	fmt.Printf("Proxy server listening on port %s\n", port)
	log.Fatal(server.ListenAndServe())
}
