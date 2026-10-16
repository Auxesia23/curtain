// Package curtain embeds the SvelteKit SPA assets from `ui/` in the Go server binary.
package curtain

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// `make ui` generates ui/build with the static adapter and index.html fallback.
//
//go:embed all:ui/build
var uiBuild embed.FS

// SPAHandler serves the built SvelteKit application:
//   - hashed assets under /_app/immutable/ are cached for one year
//   - other static files are served directly
//   - client-side routes fall back to index.html
func SPAHandler() http.Handler {
	sub, err := fs.Sub(uiBuild, "ui/build")
	if err != nil {
		panic(err)
	}
	fileServer := http.FileServerFS(sub)
	index, indexErr := fs.ReadFile(sub, "index.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cleanPath := strings.TrimPrefix(path.Clean(r.URL.Path), "/")

		if strings.HasPrefix(cleanPath, "api/") {
			http.NotFound(w, r)
			return
		}
		if indexErr != nil {
			http.Error(w, "UI has not been built. Run `make ui` and rebuild the server binary.", http.StatusServiceUnavailable)
			return
		}
		if cleanPath == "" || cleanPath == "." {
			writeIndex(w, index)
			return
		}

		if info, err := fs.Stat(sub, cleanPath); err == nil && !info.IsDir() {
			if strings.HasPrefix(cleanPath, "_app/immutable/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, r)
			return
		}

		writeIndex(w, index)
	})
}

func writeIndex(w http.ResponseWriter, index []byte) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(index)
}
