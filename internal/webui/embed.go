// Package webui serves the embedded single-page application. The SPA owner
// replaces dist/index.html and adds assets; the binary does not change.
package webui

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

//go:embed dist
var embedded embed.FS

var (
	indexOnce sync.Once
	indexBody []byte
	indexErr  error
)

// Handler serves files from dist and falls back to index.html for client-side
// routes. API paths never reach this handler.
func Handler() http.Handler {
	sub, err := fs.Sub(embedded, "dist")
	if err != nil {
		panic("webui: embedded dist is missing: " + err.Error())
	}
	files := http.FileServer(http.FS(sub))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		clean := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if clean == "" {
			serveIndex(sub, w, r)
			return
		}
		if _, statErr := fs.Stat(sub, clean); statErr != nil && path.Ext(clean) == "" {
			serveIndex(sub, w, r)
			return
		}
		files.ServeHTTP(w, r)
	})
}

func serveIndex(sub fs.FS, w http.ResponseWriter, r *http.Request) {
	indexOnce.Do(func() {
		indexBody, indexErr = fs.ReadFile(sub, "index.html")
	})
	if indexErr != nil {
		http.Error(w, "index.html is unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write(indexBody)
}
