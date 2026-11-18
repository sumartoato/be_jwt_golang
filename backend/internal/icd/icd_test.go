package icd

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

func ptr[T any](v T) *T { return &v }

func TestNormalize(t *testing.T) {
	in := Input{Kode: " k04.3 ", Nama: " ABNORMAL HARD TISSUE FORMATION IN PULP ", IDJenis: ptr(11)}
	if errs := in.Normalize(); len(errs) != 0 || in.Kode != "K04.3" || in.Nama != "ABNORMAL HARD TISSUE FORMATION IN PULP" {
		t.Fatalf("got %v %+v", errs, in)
	}
	for field, in := range map[string]Input{
		"kode":    {Kode: " ", Nama: "X", IDJenis: ptr(1)},
		"nama":    {Kode: "A00", Nama: strings.Repeat("n", 251), IDJenis: ptr(1)},
		"idJenis": {Kode: "A00", Nama: "X"},
	} {
		if errs := in.Normalize(); errs[field] == "" {
			t.Errorf("want error on %s, got %v", field, errs)
		}
	}
	if errs := (&Input{Kode: strings.Repeat("A", 11), Nama: "X", IDJenis: ptr(1)}).Normalize(); errs["kode"] == "" {
		t.Error("kode longer than 10 must fail")
	}
}

type fakeRepo struct {
	items    map[int]ICD
	lastList ListParams
}

func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.lastList = p
	page := Page{Rows: []ICD{}}
	for _, x := range f.items {
		page.Rows = append(page.Rows, x)
	}
	page.Total, page.UnfilteredTotal = len(page.Rows), len(page.Rows)
	return page, nil
}
func (f *fakeRepo) Jenis(context.Context) ([]Option, error) {
	return []Option{{ID: 11, Name: "GIGI"}}, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (ICD, error) {
	x, ok := f.items[id]
	if !ok {
		return ICD{}, ErrNotFound
	}
	return x, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, _ string) (int, error) {
	for _, x := range f.items {
		if x.Kode == in.Kode {
			return 0, ErrDuplicate
		}
	}
	if *in.IDJenis == 99 {
		return 0, ErrUnknownJenis
	}
	id := 20000 + len(f.items)
	f.items[id] = ICD{ID: id, Kode: in.Kode, Nama: in.Nama, IDJenis: *in.IDJenis}
	return id, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	if _, ok := f.items[id]; !ok {
		return ErrNotFound
	}
	f.items[id] = ICD{ID: id, Kode: in.Kode, Nama: in.Nama, IDJenis: *in.IDJenis}
	return nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]ICD{1: {ID: 1, Kode: "K04.3", Nama: "ABNORMAL HARD TISSUE FORMATION IN PULP", IDJenis: 11, Jenis: "GIGI"}}}
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
	if guarded != 4 {
		t.Fatalf("%d routes guarded, want 4", guarded)
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

	if code, out := do("GET", "/api/icd?page=3&size=99999&sort=nama&dir=desc&q=+pulp+", nil); code != http.StatusOK || out["total"] != 1.0 {
		t.Fatalf("list: %d %v", code, out)
	}
	lp := repo.lastList
	if lp.Limit != MaxPageSize || lp.Offset != 2*MaxPageSize || lp.Sort != "nama" || !lp.Desc || lp.Search != "pulp" {
		t.Errorf("list params = %+v", lp)
	}
	do("GET", "/api/icd?sort=drop+table", nil)
	if repo.lastList.Sort != "id" {
		t.Errorf("unknown sort must fall back to id: %q", repo.lastList.Sort)
	}
	if code, out := do("GET", "/api/icd/options", nil); code != http.StatusOK || len(out["jenis"].([]any)) != 1 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := do("POST", "/api/icd", map[string]any{"kode": "a00.1", "nama": "CHOLERA", "idJenis": 11}); code != http.StatusCreated || out["kode"] != "A00.1" {
		t.Errorf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/icd", map[string]any{"kode": "K04.3", "nama": "X", "idJenis": 11}); code != http.StatusConflict || out["errors"] == nil {
		t.Errorf("duplicate kode: %d %v", code, out)
	}
	if code, _ := do("POST", "/api/icd", map[string]any{"kode": "Z99", "nama": "X", "idJenis": 99}); code != http.StatusBadRequest {
		t.Errorf("unknown jenis: %d", code)
	}
	if code, _ := do("PUT", "/api/icd/1", map[string]any{"kode": "K04.3", "nama": "ABNORMAL HARD TISSUE", "idJenis": 11}); code != http.StatusOK {
		t.Errorf("update: %d", code)
	}
	if code, _ := do("PUT", "/api/icd/999", map[string]any{"kode": "X", "nama": "X", "idJenis": 11}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
