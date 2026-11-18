package jamkes

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
	in := Input{Nama: " MANDIRI IN HEALTH ", HasLimit: false}
	if errs := in.Normalize(); len(errs) != 0 || in.Nama != "MANDIRI IN HEALTH" {
		t.Fatalf("got %v %+v", errs, in)
	}
	for field, in := range map[string]Input{
		"nama":  {Nama: " "},
		"telp":  {Nama: "X", Telp: strings.Repeat("1", 16)},
		"limit": {Nama: "X", HasLimit: true},
	} {
		if errs := in.Normalize(); errs[field] == "" {
			t.Errorf("want error on %s, got %v", field, errs)
		}
	}
	if errs := (&Input{Nama: "X", Limit: ptr(-1.0)}).Normalize(); errs["limit"] == "" {
		t.Error("negative limit must fail")
	}
	if errs := (&Input{Nama: "X", HasLimit: true, Limit: ptr(5_000_000.0)}).Normalize(); len(errs) != 0 {
		t.Errorf("valid limit: %v", errs)
	}
}

type fakeRepo struct {
	items map[int]Jamkes
	terms map[int]map[int]TermInput
	lists map[int][]ItemInput
}

var masterTerms = []Term{{DetailID: 1, Description: "Masa berlaku PKS"}, {DetailID: 2, Description: "Tarif"}}

func newFake() *fakeRepo {
	return &fakeRepo{
		items: map[int]Jamkes{3: {ID: 3, Nama: "MANDIRI IN HEALTH", IDType: ptr(6), IsActive: true}},
		terms: map[int]map[int]TermInput{3: {1: {DetailID: 1, Value: "30 hari setelah pembayaran", Shown: true}}},
		lists: map[int][]ItemInput{},
	}
}

func (f *fakeRepo) List(context.Context) ([]Jamkes, error) {
	out := []Jamkes{}
	for _, j := range f.items {
		out = append(out, j)
	}
	return out, nil
}
func (f *fakeRepo) Types(context.Context) ([]Option, error) {
	return []Option{{ID: 6, Name: "PT & CV"}}, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (Jamkes, error) {
	j, ok := f.items[id]
	if !ok {
		return Jamkes{}, ErrNotFound
	}
	return j, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, _ string) (int, error) {
	if in.IDType != nil && *in.IDType == 99 {
		return 0, ErrUnknownType
	}
	id := 200 + len(f.items)
	f.items[id] = Jamkes{ID: id, Nama: in.Nama, IsActive: in.IsActive, HasLimit: in.HasLimit, Limit: in.Limit}
	return id, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	if _, ok := f.items[id]; !ok {
		return ErrNotFound
	}
	f.items[id] = Jamkes{ID: id, Nama: in.Nama, IsActive: in.IsActive}
	return nil
}
func (f *fakeRepo) Terms(_ context.Context, id int) ([]Term, error) {
	out := []Term{}
	for _, m := range masterTerms {
		t := m
		if s, ok := f.terms[id][m.DetailID]; ok {
			t.Value, t.Shown, t.Saved = s.Value, s.Shown, true
		}
		out = append(out, t)
	}
	return out, nil
}
func (f *fakeRepo) SaveTerms(_ context.Context, id int, terms []TermInput) error {
	f.terms[id] = map[int]TermInput{}
	for _, t := range terms {
		if t.DetailID > 2 {
			return ErrUnknownTerm
		}
		if t.Value != "" {
			f.terms[id][t.DetailID] = t
		}
	}
	return nil
}
func (f *fakeRepo) Items(_ context.Context, id int) ([]Item, error) {
	out := []Item{}
	for i, it := range f.lists[id] {
		out = append(out, Item{No: i + 1, Value: it.Value, Shown: it.Shown})
	}
	return out, nil
}
func (f *fakeRepo) SaveItems(_ context.Context, id int, items []ItemInput) error {
	f.lists[id] = items
	return nil
}

type server struct {
	mux    *http.ServeMux
	tokens *auth.TokenManager
	repo   *fakeRepo
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := newFake()
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	pass := func(next http.Handler) http.Handler { return next }
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q, want %q", path, PagePath)
		}
		guarded++
		return pass
	}
	mux := http.NewServeMux()
	NewHandler(repo).Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 7 {
		t.Fatalf("%d routes guarded, want 7", guarded)
	}
	return &server{mux: mux, tokens: tokens, repo: repo}
}

func (s *server) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	tok, _, _ := s.tokens.Generate("11111111-1111-1111-1111-111111111111", "admin@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestJamkesCRUD(t *testing.T) {
	s := newServer(t)
	if code, out := s.do(t, "GET", "/api/jamkes", nil); code != http.StatusOK || len(out["jamkes"].([]any)) != 1 || len(out["types"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	code, out := s.do(t, "POST", "/api/jamkes", map[string]any{"nama": " BPJS ", "isActive": true, "hasLimit": true, "limit": 1000000})
	if code != http.StatusCreated || out["nama"] != "BPJS" || out["limit"] != 1000000.0 {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := s.do(t, "POST", "/api/jamkes", map[string]any{"nama": "X", "idType": 99}); code != http.StatusBadRequest || out["errors"].(map[string]any)["idType"] == nil {
		t.Errorf("unknown type: %d %v", code, out)
	}
	if code, out := s.do(t, "POST", "/api/jamkes", map[string]any{"nama": "X", "hasLimit": true}); code != http.StatusBadRequest || out["errors"].(map[string]any)["limit"] == nil {
		t.Errorf("limit without amount: %d %v", code, out)
	}
	if code, _ := s.do(t, "PUT", "/api/jamkes/3", map[string]any{"nama": "MANDIRI", "isActive": false}); code != http.StatusOK {
		t.Errorf("update: %d", code)
	}
	if code, _ := s.do(t, "PUT", "/api/jamkes/999", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}

func TestTermsAndItems(t *testing.T) {
	s := newServer(t)
	_, out := s.do(t, "GET", "/api/jamkes/3/pks", nil)
	terms := out["terms"].([]any)
	if len(terms) != 2 || terms[0].(map[string]any)["saved"] != true || terms[1].(map[string]any)["saved"] != false {
		t.Fatalf("terms: %v", terms)
	}
	code, out := s.do(t, "PUT", "/api/jamkes/3/pks", map[string]any{"terms": []map[string]any{
		{"detailId": 1, "value": " ", "shown": true}, {"detailId": 2, "value": " Fee For Service ", "shown": true},
	}})
	if code != http.StatusOK || s.repo.terms[3][2].Value != "Fee For Service" || len(s.repo.terms[3]) != 1 {
		t.Fatalf("save terms: %d %v %v", code, out, s.repo.terms[3])
	}
	if code, _ := s.do(t, "PUT", "/api/jamkes/3/pks", map[string]any{"terms": []map[string]any{{"detailId": 1, "value": "a"}, {"detailId": 1, "value": "b"}}}); code != http.StatusBadRequest {
		t.Errorf("repeated term: %d", code)
	}
	if code, _ := s.do(t, "GET", "/api/jamkes/999/pks", nil); code != http.StatusNotFound {
		t.Errorf("missing insurer: %d", code)
	}

	code, out = s.do(t, "PUT", "/api/jamkes/3/items", map[string]any{"items": []map[string]any{
		{"value": "Penyakit akibat alkohol", "shown": true}, {"value": "  "}, {"value": "General Check-Up", "shown": false},
	}})
	items := out["items"].([]any)
	if code != http.StatusOK || len(items) != 2 || items[1].(map[string]any)["no"] != 2.0 {
		t.Fatalf("save items: %d %v", code, out)
	}
}
