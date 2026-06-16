package middleware

import (
	"fmt"
	"net/http"
)

// RequireTier rejects the request if the user's tier is below the required tier.
// Tier hierarchy: free < pro
// This is the primary hook for gating pro features.
func RequireTier(required string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userTier := GetUserTier(r.Context())
			if !tierSatisfies(userTier, required) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusPaymentRequired)
				fmt.Fprintf(w, `{"error":"this feature requires the %s plan","upgradeUrl":"/upgrade"}`, required)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func tierSatisfies(userTier, required string) bool {
	rank := map[string]int{"free": 0, "pro": 1}
	return rank[userTier] >= rank[required]
}

// CORS adds permissive CORS headers. In production, restrict AllowedOrigins.
func CORS(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization")
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// JSON sets Content-Type to application/json for all responses.
func JSON(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}
