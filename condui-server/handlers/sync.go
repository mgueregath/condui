package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"condui-server/db/queries"
	"condui-server/middleware"
	"condui-server/models"
)

type SyncHandler struct {
	blobs  *queries.BlobStore
	shares *queries.ShareStore
}

func NewSyncHandler(blobs *queries.BlobStore, shares *queries.ShareStore) *SyncHandler {
	return &SyncHandler{blobs: blobs, shares: shares}
}

func (h *SyncHandler) ListBlobs(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	metas, err := h.blobs.List(userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list blobs")
		return
	}
	if metas == nil {
		metas = []models.BlobMeta{}
	}
	_ = json.NewEncoder(w).Encode(metas)
}

func (h *SyncHandler) GetBlobMeta(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := pathParam(r, "id")

	meta, err := h.blobs.GetMeta(id, userID)
	if err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "blob not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get blob metadata")
		return
	}
	_ = json.NewEncoder(w).Encode(meta)
}

func (h *SyncHandler) GetBlob(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	userEmail := middleware.GetUserEmail(r.Context())
	id := pathParam(r, "id")

	blob, err := h.blobs.Get(id, userID)
	if err == sql.ErrNoRows {
		if ok, shareErr := h.shares.CanAccessBlob(userEmail, id); shareErr != nil {
			writeError(w, http.StatusInternalServerError, "failed to check share access")
			return
		} else if !ok {
			writeError(w, http.StatusNotFound, "blob not found")
			return
		}

		blob, err = h.blobs.GetForShare(id)
		if err != nil {
			writeError(w, http.StatusNotFound, "blob not found")
			return
		}
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to get blob")
		return
	}
	_ = json.NewEncoder(w).Encode(blob)
}

func (h *SyncHandler) CreateBlob(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	userTier := middleware.GetUserTier(r.Context())

	// Enforce free tier blob limit
	if userTier == "free" {
		limit, _ := h.blobs.GetTierLimit("free", "blobs")
		if limit > 0 {
			count, _ := h.blobs.CountByUser(userID)
			if count >= limit {
				writeError(w, http.StatusPaymentRequired,
					"blob limit reached for free tier: upgrade to pro for unlimited storage")
				return
			}
		}
	}

	var blob models.Blob
	if err := json.NewDecoder(r.Body).Decode(&blob); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if blob.ID == "" || blob.BlobType == "" || blob.CipherText == "" {
		writeError(w, http.StatusBadRequest, "id, blobType, cipherText are required")
		return
	}
	blob.UserID = userID

	if err := h.blobs.Create(&blob); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "blob already exists, use PUT to update")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to create blob")
		return
	}

	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]string{"id": blob.ID})
}

func (h *SyncHandler) UpdateBlob(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := pathParam(r, "id")

	var blob models.Blob
	if err := json.NewDecoder(r.Body).Decode(&blob); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	blob.ID = id
	blob.UserID = userID

	if err := h.blobs.Update(&blob); err == sql.ErrNoRows {
		writeError(w, http.StatusNotFound, "blob not found")
		return
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update blob")
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"id": id})
}

func (h *SyncHandler) DeleteBlob(w http.ResponseWriter, r *http.Request) {
	userID := middleware.GetUserID(r.Context())
	id := pathParam(r, "id")

	if err := h.blobs.Delete(id, userID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete blob")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
