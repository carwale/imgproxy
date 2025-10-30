package router

import (
	"bytes"
	_ "embed"
	"net/http"
	"time"
)

//go:embed favicon.ico
var faviconContent []byte

var faviconModTime = time.Unix(0, 0)

func serveFavicon(rw http.ResponseWriter, req *http.Request) {
	if len(faviconContent) == 0 {
		http.NotFound(rw, req)
		return
	}

	rw.Header().Set("Content-Type", "image/x-icon")
	rw.Header().Set("Cache-Control", "public, max-age=86400")
	http.ServeContent(rw, req, "favicon.ico", faviconModTime, bytes.NewReader(faviconContent))
}
