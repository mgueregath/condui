package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeError(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// pathParam extracts a path segment from the URL.
// For routes like /api/v1/blobs/{id}, extracts the last segment.
func pathParam(r *http.Request, name string) string {
	// Go 1.22+ ServeMux supports named path params via r.PathValue
	if v := r.PathValue(name); v != "" {
		return v
	}
	// Fallback: extract last segment from path
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}
