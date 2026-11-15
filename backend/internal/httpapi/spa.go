package httpapi

import (
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path"
	"strings"
)

// spaHandler serves real files from dir and falls back to index.html for any
// other path, so client-side routes survive a page reload. It returns nil
// when dir has no index.html (for example, the frontend was not built).
func spaHandler(dir string) http.Handler {
	fsys := os.DirFS(dir)
	if _, err := fs.Stat(fsys, "index.html"); err != nil {
		slog.Warn("no index.html in static dir; serving the API only", "static_dir", dir)
		return nil
	}
	files := http.FileServerFS(fsys)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if info, err := fs.Stat(fsys, name); err == nil && !info.IsDir() {
				files.ServeHTTP(w, r)
				return
			}
		}
		// Never cache index.html, so a new deploy is picked up on reload.
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, fsys, "index.html")
	})
}
