package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/imgproxy/imgproxy/v3/ierrors"
	log "github.com/sirupsen/logrus"
)

const masterRefreshTimeout = 60 * time.Second

type refreshRequest struct {
	Path string `json:"path"`
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.WithError(err).Error("failed to encode JSON response")
	}
}

// POST /master/refresh with payload {"path":"<original-object-key-or-imgproxy-path>"}
func handleRefreshMaster(reqID string, rw http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()

	var payload refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		if errors.Is(err, io.EOF) {
			log.Warn("master refresh payload missing 'path'")
		} else {
			log.WithError(err).Warn("failed to decode master refresh payload")
		}
		writeJSON(rw, http.StatusBadRequest, false)
		return
	}

	path := strings.TrimSpace(payload.Path)
	if path == "" {
		log.Warn("master refresh request missing 'path'")
		writeJSON(rw, http.StatusBadRequest, false)
		return
	}

	normalized := path
	if !strings.HasPrefix(path, "0x0/") {
		normalized = "0x0/" + strings.TrimLeft(path, "/")
	}

	ctx := r.Context()
	if err := processingSem.Acquire(ctx, 1); err != nil {
		log.WithError(err).Warn("failed to acquire master refresh worker")
		writeJSON(rw, http.StatusGatewayTimeout, false)
		return
	}
	defer processingSem.Release(1)

	// Optional: hard deadline for the refresh itself
	cctx, cancel := context.WithTimeout(ctx, masterRefreshTimeout)
	defer cancel()

	resultData, err := getAndCreateMasterImageData(cctx, normalized, http.Header{})
	if err != nil {
		var ierr *ierrors.Error
		if errors.As(err, &ierr) {
			code := ierr.StatusCode()
			if code == 0 {
				code = http.StatusInternalServerError
			}
			// ierrors carry more specific HTTP codes (e.g., 404) when available.
			writeJSON(rw, code, false)
			return
		}
		log.WithError(err).Error("master refresh failed")
		writeJSON(rw, http.StatusInternalServerError, false)
		return
	}

	resultData.Close()

	writeJSON(rw, http.StatusOK, true)

}
