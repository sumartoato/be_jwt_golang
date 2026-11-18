package distributor

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
	in := Input{Nama: " PT ANARA TRI SAKTI ", CP: " Budi "}
	if errs := in.Normalize(); len(errs) != 0 || in.Nama != "PT ANARA TRI SAKTI" || in.CP != "Budi" {
		t.Fatalf("got %v %+v", errs, in)
	}
	for field, in := range map[string]Input{
		"nama":   {Nama: " "},
		"telp":   {Nama: "X", Telp: strings.Repeat("1", 16)},
		"fax":    {Nama: "X", Fax: strings.Repeat("1", 26)},
		"cp":     {Nama: "X", CP: strings.Repeat("c", 41)},
		"telpCp": {Nama: "X", TelpCP: strings.Repeat("1", 26)},
	} {
		if errs := in.Normalize(); errs[field] == "" {
			t.Errorf("want error on %s, got %v", field, errs)
		}
	}
}

type fakeRepo struct{ items map[int]Distributor }

func (f *fakeRepo) List(context.Context) ([]Distributor, error) {
	out := []Distributor{}
	for _, d := range f.items {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (Distributor, error) {
	d, ok := f.items[id]
	if !ok {
		return Distributor{}, ErrNotFound
	}
	return d, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, user string) (int, error) {
	id := 200 + len(f.items)
	f.items[id] = Distributor{ID: id, Nama: in.Nama, CP: in.CP, UpdatedBy: user}
	return id, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, user string) error {
	if _, ok := f.items[id]; !ok {
		return ErrNotFound
	}
	f.items[id] = Distributor{ID: id, Nama: in.Nama, UpdatedBy: user}
	return nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Distributor{2: {ID: 2, Nama: "APOTEK MEDICASTORE"}}}
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
	tok, _, _ := tokens.Generate("11111111-1111-1111-1111-111111111111", "a-rather-long-email-address@medika.com")
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

	if code, out := do("GET", "/api/distributor", nil); code != http.StatusOK || len(out["distributor"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	code, out := do("POST", "/api/distributor", map[string]any{"nama": " PT BARU ", "cp": "Ani"})
	if code != http.StatusCreated || out["nama"] != "PT BARU" || out["updatedBy"] != "a-rather-long-email-" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/distributor", map[string]any{"nama": ""}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty nama: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/distributor/2", map[string]any{"nama": "APOTEK MEDICASTORE 2"}); code != http.StatusOK {
		t.Errorf("update: %d", code)
	}
	if code, _ := do("PUT", "/api/distributor/999", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
