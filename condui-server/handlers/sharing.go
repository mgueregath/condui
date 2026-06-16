package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/google/uuid"

	"condui-server/db/queries"
	"condui-server/middleware"
	"condui-server/models"
)

type SharingHandler struct {
	shares *queries.ShareStore
	blobs  *queries.BlobStore
}

func NewSharingHandler(shares *queries.ShareStore, blobs *queries.BlobStore) *SharingHandler {
	return &SharingHandler{shares: shares, blobs: blobs}
}

func (h *SharingHandler) CreateShare(w http.ResponseWriter, r *http.Request) {
	ownerID := middleware.GetUserID(r.Context())

	var req struct {
		BlobID         string `json:"blobId"`
		RecipientEmail string `json:"recipientEmail"`
		Permissions    string `json:"permissions"`
		EncryptedKey   string `json:"encryptedKey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.BlobID == "" || req.RecipientEmail == "" {
		writeError(w, http.StatusBadRequest, "blobId and recipientEmail are required")
		return
	}
	if req.Permissions != "read" && req.Permissions != "write" {
		req.Permissions = "read"
	}

	// Verify owner actually owns the blob
	if _, err := h.blobs.Get(req.BlobID, ownerID); err != nil {
		writeError(w, http.StatusForbidden, "blob not found or access denied")
		return
	}

	share := &models.ShareInvite{
		ID:             uuid.NewString(),
		OwnerID:        ownerID,
		RecipientEmail: req.RecipientEmail,
		BlobID:         req.BlobID,
		EncryptedKey:   req.EncryptedKey,
		Permissions:    req.Permissions,
	}

	if err := h.shares.Create(share); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create share")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(share)
}

func (h *SharingHandler) GetSentShares(w http.ResponseWriter, r *http.Request) {
	ownerID := middleware.GetUserID(r.Context())
	shares, err := h.shares.GetSentByOwner(ownerID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch shares")
		return
	}
	if shares == nil {
		shares = []models.ShareInvite{}
	}
	_ = json.NewEncoder(w).Encode(shares)
}

func (h *SharingHandler) GetReceivedShares(w http.ResponseWriter, r *http.Request) {
	email := middleware.GetUserEmail(r.Context())
	shares, err := h.shares.GetReceivedByEmail(email)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to fetch shares")
		return
	}
	if shares == nil {
		shares = []models.ShareInvite{}
	}
	_ = json.NewEncoder(w).Encode(shares)
}

func (h *SharingHandler) AcceptShare(w http.ResponseWriter, r *http.Request) {
	email := middleware.GetUserEmail(r.Context())
	id := pathParam(r, "id")

	if err := h.shares.Accept(id, email); err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "share not found or already processed")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to accept share")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
}

func (h *SharingHandler) DeleteShare(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	email := middleware.GetUserEmail(r.Context())
	id := pathParam(r, "id")

	if err := h.shares.Delete(id, userID, email); err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "share not found or access denied")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete share")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
