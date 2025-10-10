package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/imgproxy/imgproxy/v3/ierrors"
)

type refreshResp struct {
	Status      string `json:"status"`       // "updated"
	Path        string `json:"path"`         // input path as you passed it
	MasterKey   string `json:"master_key"`   // key used under the master bucket
	Format      string `json:"format"`       // e.g. "avif", "jpeg"
	Width       string `json:"width"`        // X-Result-Width
	Height      string `json:"height"`       // X-Result-Height
	Bytes       int    `json:"bytes"`        // processed bytes length
	ProcessedAt string `json:"processed_at"` // RFC3339
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// PUT /admin/master/refresh?path=<original-object-key-or-imgproxy-path>
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
		writeJSON(rw, http.StatusGatewayTimeout, map[string]string{"error": "timeout acquiring worker"})
		return
	}
	defer processingSem.Release(1)

	// Optional: hard deadline for the refresh itself
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resultData, err := getAndCreateMasterImageData(cctx, normalized, http.Header{})
	if err != nil {
		var ierr *ierrors.Error
		if errors.As(err, &ierr) {
			code := ierr.StatusCode()
			if code == 0 {
				code = http.StatusInternalServerError
			}
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
		MasterKey:   normalized,
		Format:      format,
		Width:       wStr,
		Height:      hStr,
		Bytes:       bytes,
		ProcessedAt: time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(rw, http.StatusOK, resp)

}
