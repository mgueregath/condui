package main

import (
	"log"
	"net/http"

	"condui-server/config"
	"condui-server/db"
	"condui-server/db/queries"
	"condui-server/handlers"
	"condui-server/middleware"
)

func main() {
	cfg := config.Load()

	database, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer database.Close()

	// Stores
	users := queries.NewUserStore(database.DB)
	tokens := queries.NewTokenStore(database.DB)
	blobs := queries.NewBlobStore(database.DB)
	shares := queries.NewShareStore(database.DB)

	// Handlers
	authH := handlers.NewAuthHandler(users, tokens, cfg)
	syncH := handlers.NewSyncHandler(blobs, shares)
	shareH := handlers.NewSharingHandler(shares, blobs)

	// Middleware chain builders
	authMW := middleware.Auth(cfg.JWTSecret)
	proMW := middleware.RequireTier("pro")

	mux := http.NewServeMux()

	// ── Auth ────────────────────────────────────────────────────────────
	mux.HandleFunc("POST /api/v1/auth/register", authH.Register)
	mux.HandleFunc("POST /api/v1/auth/login", authH.Login)
	mux.HandleFunc("POST /api/v1/auth/refresh", authH.Refresh)
	mux.HandleFunc("POST /api/v1/auth/logout", chain(authH.Logout, authMW))
	mux.HandleFunc("GET /api/v1/auth/me", chain(authH.Me, authMW))
	mux.HandleFunc("PUT /api/v1/auth/identity", chain(authH.UpdateIdentity, authMW))

	// ── User lookup (for sharing) ────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/users/public-key", chain(authH.GetPublicKeyByEmail, authMW))

	// ── Sync (blobs) ─────────────────────────────────────────────────────
	mux.HandleFunc("GET /api/v1/blobs", chain(syncH.ListBlobs, authMW))
	mux.HandleFunc("POST /api/v1/blobs", chain(syncH.CreateBlob, authMW))
	mux.HandleFunc("GET /api/v1/blobs/{id}/meta", chain(syncH.GetBlobMeta, authMW))
	mux.HandleFunc("GET /api/v1/blobs/{id}", chain(syncH.GetBlob, authMW))
	mux.HandleFunc("PUT /api/v1/blobs/{id}", chain(syncH.UpdateBlob, authMW))
	mux.HandleFunc("DELETE /api/v1/blobs/{id}", chain(syncH.DeleteBlob, authMW))

	// ── Sharing (pro tier) ───────────────────────────────────────────────
	mux.HandleFunc("POST /api/v1/shares", chain(shareH.CreateShare, authMW, proMW))
	mux.HandleFunc("GET /api/v1/shares/sent", chain(shareH.GetSentShares, authMW))
	mux.HandleFunc("GET /api/v1/shares/received", chain(shareH.GetReceivedShares, authMW))
	mux.HandleFunc("PUT /api/v1/shares/{id}/accept", chain(shareH.AcceptShare, authMW))
	mux.HandleFunc("DELETE /api/v1/shares/{id}", chain(shareH.DeleteShare, authMW))

	// ── Health ───────────────────────────────────────────────────────────
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Apply global middleware: CORS + JSON content type
	handler := middleware.CORS(cfg.AllowedOrigins)(middleware.JSON(mux))

	log.Printf("condui-server starting on :%s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, handler); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

// chain applies middleware in left-to-right order: chain(h, mw1, mw2) = mw1(mw2(h))
func chain(h http.HandlerFunc, mws ...func(http.Handler) http.Handler) http.HandlerFunc {
	handler := http.Handler(h)
	for i := len(mws) - 1; i >= 0; i-- {
		handler = mws[i](handler)
	}
	return handler.ServeHTTP
}
