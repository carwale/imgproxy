package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/imgproxy/imgproxy/v3/ierrors"
	log "github.com/sirupsen/logrus"
)

const masterRefreshTimeout = 60 * time.Second

type refreshResp struct {
	Status      string `json:"status"`       // "updated"
	Path        string `json:"path"`         // input path as you passed it
	Format      string `json:"format"`       // e.g. "avif", "jpeg"
	Width       string `json:"width"`        // X-Result-Width
	Height      string `json:"height"`       // X-Result-Height
	Bytes       int    `json:"bytes"`        // processed bytes length
	ProcessedAt string `json:"processed_at"` // RFC3339
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.WithError(err).Error("failed to encode JSON response")
	}
}

// GET /master/refresh?path=<original-object-key-or-imgproxy-path>
func handleRefreshMaster(reqID string, rw http.ResponseWriter, r *http.Request) {

	path := strings.TrimSpace(r.URL.Query().Get("path"))
	if path == "" {
		writeJSON(rw, http.StatusBadRequest, map[string]string{"error": "query param 'path' is required"})
		return
	}

	normalized := path
	if !strings.HasPrefix(path, "0x0/") {
		normalized = "0x0/" + strings.TrimLeft(path, "/")
	}

	ctx := r.Context()
	if err := processingSem.Acquire(ctx, 1); err != nil {
		log.WithError(err).Warn("failed to acquire master refresh worker")
		writeJSON(rw, http.StatusGatewayTimeout, map[string]string{"error": "timeout acquiring worker"})
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
			writeJSON(rw, code, map[string]string{"error": ierr.Error()})
			return
		}
		writeJSON(rw, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	bytes := len(resultData.Data)
	format := resultData.Type.String()
	wStr := resultData.Headers["X-Result-Width"]
	hStr := resultData.Headers["X-Result-Height"]
	resultData.Close()

	resp := refreshResp{
		Status:      "updated",
		Path:        path,
		Format:      format,
		Width:       wStr,
		Height:      hStr,
		Bytes:       bytes,
		ProcessedAt: time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(rw, http.StatusOK, resp)

}
