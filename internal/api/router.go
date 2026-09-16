package api

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// NewRouter sets up the HTTP router and middleware
func NewRouter(store cache.Store, dispatcher *provider.Dispatcher) http.Handler {
	mux := http.NewServeMux()

	getHandler := GetHandler(store, dispatcher)
	searchHandler := SearchHandler(store, dispatcher)

	// LRCLIB endpoints
	mux.HandleFunc("/api/get", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "Method not allowed"})
			return
		}
		getHandler(w, r)
	})

	// Optional ID-based endpoint: /api/get/{id}
	mux.HandleFunc("/api/get/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "Method not allowed"})
			return
		}

		idStr := strings.TrimPrefix(r.URL.Path, "/api/get/")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			getHandler(w, r)
			return
		}

		cached, err := store.GetByID(id)
		if err != nil || cached == nil {
			writeJSON(w, http.StatusNotFound, model.ErrorResponse{Error: "Lyrics not found"})
			return
		}

		writeJSON(w, http.StatusOK, cached)
	})

	mux.HandleFunc("/api/search", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			return
		}
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, model.ErrorResponse{Error: "Method not allowed"})
			return
		}
		searchHandler(w, r)
	})

	// Health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Root path with helpful status info
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "x-cyber-lrc-hub",
			"status":  "running",
			"version": "1.0.0",
		})
	})

	return recoveryMiddleware(corsMiddleware(loggingMiddleware(mux)))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		log.Printf("%s %s %s", r.Method, r.URL.RequestURI(), duration)
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				log.Printf("[PANIC] %v", rec)
				writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
					Error: "Internal server error",
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
