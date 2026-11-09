// Package webui serves the embedded single-page application. The SPA owner
// replaces dist/index.html and assets; the binary does not change.
package webui

import (
	"embed"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"sync"
)

// The all: prefix keeps the committed .gitkeep, so a fresh clone still embeds
// a valid directory before the SPA is built.
//
//go:embed all:dist
var embedded embed.FS

const notBuiltHTML = `<!doctype html>
<html lang="en">
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1" />
    <title>SSO Service</title>
    <style>
      body {
        margin: 0;
        min-height: 100vh;
        display: grid;
        place-items: center;
        background: #f8fafc;
        color: #0f172a;
        font-family: system-ui, -apple-system, "Segoe UI", sans-serif;
      }
      main {
        max-width: 34rem;
        padding: 2rem;
      }
      p {
        color: #475569;
        line-height: 1.65;
      }
      code {
        color: #1d4ed8;
      }
    </style>
  </head>
  <body>
    <main>
      <h1>UI not built</h1>
      <p>The Go service is running, but this binary has no bundled single-page application.</p>
      <p>Run <code>make web-build</code> and rebuild, or start the Vite dev server from <code>web/</code>.</p>
      <p>Health lives at <code>/healthz</code> and readiness at <code>/readyz</code>.</p>
    </main>
  </body>
</html>
`

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
	if errors.Is(indexErr, fs.ErrNotExist) {
		serveNotBuilt(w, r)
		return
	}
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

func serveNotBuilt(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodHead {
		return
	}
	_, _ = w.Write([]byte(notBuiltHTML))
}
