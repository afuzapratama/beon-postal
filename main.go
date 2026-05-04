package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	if err := loadPostalDB(); err != nil {
		log.Fatalf("load postal data: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /postal/{code}", postalHandler)
	mux.HandleFunc("GET /health", healthHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	n, _ := countEntries()
	log.Printf("Postal API ready — %d entries in SQLite, listening on :%s", n, port)
	if err := http.ListenAndServe(":"+port, corsMiddleware(mux)); err != nil {
		log.Fatal(err)
	}
}

// corsMiddleware adds CORS headers to every response and handles preflight
// OPTIONS requests. The allowed origin defaults to "*" but can be overridden
// by setting the CORS_ORIGIN environment variable.
func corsMiddleware(next http.Handler) http.Handler {
	allowedOrigin := os.Getenv("CORS_ORIGIN")
	if allowedOrigin == "" {
		allowedOrigin = "*"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", allowedOrigin)
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Preflight request — respond immediately without hitting handlers.
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// apiResponse is the standard envelope returned by every endpoint.
type apiResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Meta    meta   `json:"meta"`
}

type meta struct {
	PoweredBy string `json:"powered_by"`
	Timestamp string `json:"timestamp"`
}

func newMeta() meta {
	return meta{
		PoweredBy: "BEON API",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// writeJSON sends a pretty-printed JSON response.
func writeJSON(w http.ResponseWriter, status int, payload apiResponse) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Powered-By", "BEON API")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(payload); err != nil {
		log.Printf("writeJSON encode: %v", err)
	}
}

// postalHandler handles GET /postal/{code}
// Accepts 7-digit code with or without hyphen, e.g. 1130021 or 113-0021.
func postalHandler(w http.ResponseWriter, r *http.Request) {
	code := strings.ReplaceAll(r.PathValue("code"), "-", "")
	code = strings.TrimSpace(code)

	if len(code) != 7 {
		writeJSON(w, http.StatusBadRequest, apiResponse{
			Success: false,
			Error:   "postal code must be 7 digits",
			Meta:    newMeta(),
		})
		return
	}

	entries, err := queryByCode(code)
	if err != nil {
		log.Printf("queryByCode %s: %v", code, err)
		writeJSON(w, http.StatusInternalServerError, apiResponse{
			Success: false,
			Error:   "database error",
			Meta:    newMeta(),
		})
		return
	}
	if len(entries) == 0 {
		writeJSON(w, http.StatusNotFound, apiResponse{
			Success: false,
			Error:   "postal code not found",
			Meta:    newMeta(),
		})
		return
	}

	// Return a single object when there is only one match so the caller
	// can do data.prefecture directly instead of data[0].prefecture.
	// Return an array when there are multiple matches (some codes cover
	// several cities/towns).
	var payload any
	if len(entries) == 1 {
		payload = entries[0]
	} else {
		payload = entries
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data:    payload,
		Meta:    newMeta(),
	})
}

// healthHandler handles GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	n, _ := countEntries()
	writeJSON(w, http.StatusOK, apiResponse{
		Success: true,
		Data: map[string]any{
			"status":  "ok",
			"records": n,
		},
		Meta: newMeta(),
	})
}

// jsonError is kept for internal use by writeJSON error cases (not exported).
func jsonError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{
		Success: false,
		Error:   msg,
		Meta:    newMeta(),
	})
}
