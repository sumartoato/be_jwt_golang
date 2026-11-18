package principal

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
)

func TestNormalize(t *testing.T) {
	in := Input{Nama: "  PT BAYER INDONESIA "}
	if errs := in.Normalize(); errs != nil || in.Nama != "PT BAYER INDONESIA" {
		t.Fatalf("got %v %q", errs, in.Nama)
	}
	if errs := (&Input{Nama: " "}).Normalize(); errs["nama"] == "" {
		t.Error("empty nama must fail")
	}
	if errs := (&Input{Nama: strings.Repeat("n", 301)}).Normalize(); errs["nama"] == "" {
		t.Error("nama longer than 300 must fail")
	}
}

type fakeRepo struct{ items map[int]string }

func (f *fakeRepo) List(context.Context) ([]Principal, error) {
	out := []Principal{}
	for id, n := range f.items {
		out = append(out, Principal{ID: id, Nama: n})
	}
	return out, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input) (Principal, error) {
	id := 300 + len(f.items)
	f.items[id] = in.Nama
	return Principal{ID: id, Nama: in.Nama}, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input) (Principal, error) {
	if _, ok := f.items[id]; !ok {
		return Principal{}, ErrNotFound
	}
	f.items[id] = in.Nama
	return Principal{ID: id, Nama: in.Nama}, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]string{2: "PT ABBOT INDONESIA"}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	pass := func(next http.Handler) http.Handler { return next }
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q", path)
		}
		guarded++
		return pass
	}
	mux := http.NewServeMux()
	NewHandler(repo).Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 3 {
		t.Fatalf("%d routes guarded, want 3", guarded)
	}
	tok, _, _ := tokens.Generate("11111111-1111-1111-1111-111111111111", "admin@medika.com")
	do := func(method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	if code, out := do("GET", "/api/principal", nil); code != http.StatusOK || len(out["principal"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("POST", "/api/principal", map[string]any{"nama": " PT BARU "}); code != http.StatusCreated || out["nama"] != "PT BARU" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/principal", map[string]any{"nama": ""}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty nama: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/principal/2", map[string]any{"nama": "PT ABBOTT INDONESIA"}); code != http.StatusOK || out["nama"] != "PT ABBOTT INDONESIA" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/principal/999", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
