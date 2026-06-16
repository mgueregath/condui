package middleware

import (
	"context"
	"net/http"
	"strings"

	"condui-server/crypto"
)

type contextKey string

const (
	CtxUserID    contextKey = "userID"
	CtxUserEmail contextKey = "userEmail"
	CtxUserTier  contextKey = "userTier"
)

// Auth validates the Bearer token and injects user info into the request context.
func Auth(secret []byte) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if !strings.HasPrefix(authHeader, "Bearer ") {
				http.Error(w, `{"error":"missing or invalid authorization header"}`, http.StatusUnauthorized)
				return
			}
			tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
			claims, err := crypto.VerifyAccessToken(secret, tokenStr)
			if err != nil {
				http.Error(w, `{"error":"invalid or expired token"}`, http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), CtxUserID, claims.UserID)
			ctx = context.WithValue(ctx, CtxUserEmail, claims.Email)
			ctx = context.WithValue(ctx, CtxUserTier, claims.Tier)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetUserID extracts the authenticated user ID from context.
func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserID).(string)
	return v
}

// GetUserEmail extracts the authenticated user email from context.
func GetUserEmail(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserEmail).(string)
	return v
}

// GetUserTier extracts the authenticated user tier from context.
func GetUserTier(ctx context.Context) string {
	v, _ := ctx.Value(CtxUserTier).(string)
	return v
}
