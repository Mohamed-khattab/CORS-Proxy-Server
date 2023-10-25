package main

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type ModifyResponseWriter struct {
	http.ResponseWriter
}

func (w *ModifyResponseWriter) Write(b []byte) (int, error) {
	// Add a custom response header
	w.Header().Set("X-Custom-Response-Header", "Modified-Response")

	// Modify the response body based on a condition
	originalBody := string(b)
	modifiedBody := originalBody

	// Check if the original body contains a specific string
	if strings.Contains(originalBody, "Hello") {
		// If the string is found, replace it with a custom message
		modifiedBody = strings.Replace(originalBody, "Hello", "Modified Hello", -1)
	}

	// Write the modified response body
	return w.ResponseWriter.Write([]byte(modifiedBody))
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Serve the request
		next.ServeHTTP(w, r)

		// Log the request details
		log.Printf(
			"Method: %s\tURL: %s\tHeaders: %v\tStatus: %d\tDuration: %v\n",
			r.Method, r.URL, r.Header, w.(ModifyResponseWriter).Status(), time.Since(start),
		)
	})
}

type rateLimiter struct {
	mu        sync.Mutex
	clients   map[string]int
	rateLimit int
	interval  time.Duration
}

func newRateLimiter(rateLimit int, interval time.Duration) *rateLimiter {
	return &rateLimiter{
		clients:   make(map[string]int),
		rateLimit: rateLimit,
		interval:  interval,
	}
}

func (rl *rateLimiter) isAllowed(clientIP string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	count, exists := rl.clients[clientIP]
	if !exists || now.Sub(time.Unix(int64(count), 0)) > rl.interval {
		// Reset the counter if the interval has passed
		rl.clients[clientIP] = int(now.Unix())
		return true
	}

	// Increment the counter if within the interval
	if count < rl.rateLimit {
		rl.clients[clientIP]++
		return true
	}

	return false
}

func rateLimitingMiddleware(rl *rateLimiter, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientIP := strings.Split(r.RemoteAddr, ":")[0]

		if rl.isAllowed(clientIP) {
			next.ServeHTTP(w, r)
		} else {
			http.Error(w, "Rate Limit Exceeded", http.StatusTooManyRequests)
		}
	})
}

func handleProxy(w http.ResponseWriter, r *http.Request) {
	// Your dynamic target URL configuration logic here
	// For simplicity, let's set the target URL based on a query parameter
	targetQueryParam := r.URL.Query().Get("target")
	if targetQueryParam == "" {
		http.Error(w, "Target query parameter is required", http.StatusBadRequest)
		return
	}

	targetURL := targetQueryParam

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

	// Set up rate limiting with a limit of 10 requests per minute per IP
	rl := newRateLimiter(10, time.Minute)

	// Use the default ServeMux, which is a basic router
	mux := http.NewServeMux()

	// Register middleware
	mux.HandleFunc("/", handleProxy)
	handler := loggingMiddleware(rateLimitingMiddleware(rl, mux))

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	fmt.Printf("Proxy server listening on port %s\n", port)
	log.Fatal(server.ListenAndServe())
}
