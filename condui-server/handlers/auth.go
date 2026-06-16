package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"condui-server/config"
	"condui-server/crypto"
	"condui-server/db/queries"
	"condui-server/middleware"
)

type AuthHandler struct {
	users  *queries.UserStore
	tokens *queries.TokenStore
	cfg    config.Config
}

func NewAuthHandler(users *queries.UserStore, tokens *queries.TokenStore, cfg config.Config) *AuthHandler {
	return &AuthHandler{users: users, tokens: tokens, cfg: cfg}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))
	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	userID := uuid.NewString()
	if err := h.users.Create(userID, req.Email, string(hash)); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create user")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"userId": userID})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Email      string `json:"email"`
		Password   string `json:"password"`
		DeviceName string `json:"deviceName"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	user, err := h.users.GetByEmail(req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	effectiveTier := user.EffectiveTier()

	// Check device limit for free tier
	if effectiveTier == "free" {
		count, _ := h.tokens.CountByUser(user.ID)
		if count >= 2 { // free tier: 2 devices
			writeError(w, http.StatusPaymentRequired, "device limit reached: upgrade to pro for unlimited devices")
			return
		}
	}

	accessToken, err := crypto.GenerateAccessToken(
		h.cfg.JWTSecret, user.ID, user.Email, effectiveTier, h.cfg.AccessTokenExpiry,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	rawRefresh, refreshHash, err := crypto.GenerateRefreshToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate refresh token")
		return
	}

	deviceName := req.DeviceName
	if deviceName == "" {
		deviceName = "Unknown device"
	}

	tokenID := uuid.NewString()
	expiresAt := time.Now().Add(h.cfg.RefreshTokenExpiry)
	if err := h.tokens.Create(tokenID, user.ID, refreshHash, deviceName, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to store refresh token")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]any{
		"accessToken":  accessToken,
		"refreshToken": rawRefresh,
		"user": map[string]string{
			"id":        user.ID,
			"email":     user.Email,
			"tier":      effectiveTier,
			"publicKey": user.PublicKey,
		},
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	hash := crypto.HashRefreshToken(req.RefreshToken)
	token, err := h.tokens.GetByHash(hash)
	if err != nil || time.Now().After(token.ExpiresAt) {
		writeError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	user, err := h.users.GetByID(token.UserID)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "user not found")
		return
	}

	accessToken, err := crypto.GenerateAccessToken(
		h.cfg.JWTSecret, user.ID, user.Email, user.EffectiveTier(), h.cfg.AccessTokenExpiry,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate token")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"accessToken": accessToken})
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	if req.RefreshToken != "" {
		hash := crypto.HashRefreshToken(req.RefreshToken)
		_ = h.tokens.Delete(hash)
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	user, err := h.users.GetByID(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, "user not found")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":            user.ID,
		"email":         user.Email,
		"tier":          user.EffectiveTier(),
		"tierExpiresAt": user.TierExpiresAt,
		"publicKey":     user.PublicKey,
		"identityBlob":  user.IdentityBlob,
	})
}

func (h *AuthHandler) UpdateIdentity(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	var req struct {
		PublicKey    string `json:"publicKey"`
		IdentityBlob string `json:"identityBlob"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.PublicKey == "" {
		writeError(w, http.StatusBadRequest, "publicKey is required")
		return
	}
	if err := h.users.UpdateIdentity(userID, req.PublicKey, req.IdentityBlob); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update identity")
		return
	}
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *AuthHandler) GetPublicKeyByEmail(w http.ResponseWriter, r *http.Request) {
	email := strings.ToLower(r.URL.Query().Get("email"))
	if email == "" {
		writeError(w, http.StatusBadRequest, "email query param required")
		return
	}
	pk, err := h.users.GetPublicKeyByEmail(email)
	if err != nil || pk == "" {
		writeError(w, http.StatusNotFound, "user not found or has no public key")
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]string{"publicKey": pk})
}
