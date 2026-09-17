// Package api implements the LRCLIB-compatible HTTP surface: routing,
// middleware, and the two read endpoints.
//
// Response field sets and error bodies are verified against the live
// lrclib.net service rather than assumed; see the compatibility table in the
// README before changing them.

package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/cache"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/model"
	"github.com/x-cyber-space/x-cyber-lrc-hub/internal/provider"
)

// NewRouter sets up the HTTP router and middleware.
func NewRouter(store cache.Store, dispatcher *provider.Dispatcher, version string) http.Handler {
	mux := http.NewServeMux()

	getHandler := GetHandler(store, dispatcher)
	searchHandler := SearchHandler(store, dispatcher)

	// LRCLIB endpoints.
	mux.HandleFunc("/api/get", getOnly(getHandler))
	mux.HandleFunc("/api/get/", getOnly(func(w http.ResponseWriter, r *http.Request) {
		// /api/get/{id} is a first-class LRCLIB endpoint. A non-numeric
		// suffix is not an error: fall back to the query-parameter form so
		// clients that probe it still work.
		id, err := strconv.Atoi(strings.TrimPrefix(r.URL.Path, "/api/get/"))
		if err != nil {
			getHandler(w, r)
			return
		}

		cached, err := store.GetByID(id)
		if err != nil || cached == nil {
			writeJSON(w, http.StatusNotFound, model.TrackNotFoundError())
			return
		}
		writeJSON(w, http.StatusOK, cached)
	}))
	mux.HandleFunc("/api/search", getOnly(searchHandler))

	// Health check endpoint (not part of LRCLIB).
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Root path with helpful status info.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			// lrclib.net answers an unknown route with an empty 404 body.
			w.WriteHeader(http.StatusNotFound)
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "x-cyber-lrc-hub",
			"status":  "running",
			"version": version,
		})
	})

	return recoveryMiddleware(corsMiddleware(loggingMiddleware(mux)))
}

// getOnly rejects any non-GET method the way lrclib.net does: a bare 405 with
// no body and no Content-Type.
func getOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		next(w, r)
	}
}

// writeJSON writes a JSON body the way lrclib.net does: compact, no trailing
// newline, HTML escaping disabled (lyrics are full of & and <), and
// `application/json` without a charset suffix.
//
// The exact byte layout matters for the error envelope, which clients match
// against the published shape:
//
//	{"message":"Failed to find specified track","name":"TrackNotFound","statusCode":404}
func writeJSON(w http.ResponseWriter, status int, payload any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		slog.Error("json encode failed", "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(bytes.TrimSuffix(buf.Bytes(), []byte("\n")))
}

// writeMissingField reproduces LRCLIB's plain-text 400 for a missing required
// query parameter.
func writeMissingField(w http.ResponseWriter, field string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = io.WriteString(w, model.MissingFieldError(field))
}

// firstNonEmpty returns the first non-empty argument.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		// lrclib.net advertises this so browsers can read a 503's Retry-After.
		w.Header().Set("Access-Control-Expose-Headers", "retry-after")

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
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		slog.Info("request",
			"method", r.Method,
			"path", r.URL.RequestURI(),
			"status", rec.status,
			"duration", time.Since(start).Round(time.Microsecond).String(),
		)
	})
}

// statusRecorder captures the response status so the access log carries it.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wroteHeader {
		r.status = status
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	r.wroteHeader = true
	return r.ResponseWriter.Write(b)
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// The stack is the whole point of recovering; without it the
				// log line says only "something panicked".
				slog.Error("panic recovered",
					"panic", rec,
					"path", r.URL.RequestURI(),
					"stack", string(debug.Stack()),
				)
				writeJSON(w, http.StatusInternalServerError, model.ErrorResponse{
					Message:    "Internal server error",
					Name:       "InternalServerError",
					StatusCode: http.StatusInternalServerError,
				})
			}
		}()
		next.ServeHTTP(w, r)
	})
}
