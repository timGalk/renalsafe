package httpapi

import (
	"io/fs"
	"net/http"
	"strings"
)

// The application and API share one origin. Only browser routes fall back to
// the React shell; unknown API, health and static-asset paths stay real 404s.
func browserAssets() http.Handler {
	dist, _ := fs.Sub(assets, "web/dist")
	training, _ := fs.Sub(assets, "web")
	files := http.FileServer(http.FS(dist))
	demo := http.FileServer(http.FS(training))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/demo.html" || path == "/demo.js" || path == "/demo.css" {
			demo.ServeHTTP(w, r)
			return
		}
		if strings.HasPrefix(path, "/api/") || path == "/api" || strings.HasPrefix(path, "/healthz") || strings.HasPrefix(path, "/readyz") {
			respond(w, 404, map[string]string{"error": "not_found"})
			return
		}
		if path != "/" {
			if f, e := dist.Open(strings.TrimPrefix(path, "/")); e == nil {
				info, e := f.Stat()
				f.Close()
				if e == nil && !info.IsDir() {
					files.ServeHTTP(w, r)
					return
				}
			}
		}
		if path == "/assets" || strings.HasPrefix(path, "/assets/") || strings.Contains(path, ".") {
			http.NotFound(w, r)
			return
		}
		shell, e := fs.ReadFile(dist, "index.html")
		if e != nil {
			http.Error(w, "Frontend build unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(shell)
	})
}
