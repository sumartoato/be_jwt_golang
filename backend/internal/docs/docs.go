// Package docs serves the OpenAPI spec and a Swagger UI page.
// It is only mounted when SWAGGER_ENABLED=true, so production can hide it.
package docs

import (
	_ "embed"
	"net/http"
)

//go:embed openapi.yaml
var spec []byte

//go:embed index.html
var index []byte

// Register mounts the Swagger UI at /swagger/ and the spec at /swagger/openapi.yaml.
func Register(mux *http.ServeMux) {
	mux.Handle("GET /swagger", http.RedirectHandler("/swagger/", http.StatusMovedPermanently))
	mux.HandleFunc("GET /swagger/{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})
	mux.HandleFunc("GET /swagger/openapi.yaml", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/yaml")
		_, _ = w.Write(spec)
	})
}
