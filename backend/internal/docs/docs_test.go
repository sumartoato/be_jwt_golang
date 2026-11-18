package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRegisterServesUIAndSpec(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	if rec := get(t, mux, "/swagger"); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/swagger/" {
		t.Fatalf("/swagger: got %d Location=%q", rec.Code, rec.Header().Get("Location"))
	}

	rec := get(t, mux, "/swagger/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("/swagger/: got %d", rec.Code)
	}

	rec = get(t, mux, "/swagger/openapi.yaml")
	if rec.Code != http.StatusOK {
		t.Fatalf("/swagger/openapi.yaml: got %d", rec.Code)
	}
	// Every route registered in main.go (auth, menu, role, branch) must be documented.
	for _, p := range []string{
		"/api/auth/register:", "/api/auth/login:", "/api/auth/refresh:", "/api/me:", "/api/menus:",
		"/api/roles:", "/api/roles/{id}:", "/api/users:", "/api/users/{id}/role:", "/api/users/{id}/branches:", "/api/users/{id}/menus:", "/api/branches:", "/api/branches/{kode}:", "/healthz:",
	} {
		if !strings.Contains(rec.Body.String(), p) {
			t.Errorf("spec is missing path %s", strings.TrimSuffix(p, ":"))
		}
	}
}

func TestNotMountedMeansNotFound(t *testing.T) {
	mux := http.NewServeMux() // SWAGGER_ENABLED=false: Register is never called
	if rec := get(t, mux, "/swagger/"); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}
