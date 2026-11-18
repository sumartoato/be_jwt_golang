package penunjang

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
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

const (
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to MED and MIT
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func f(v float64) *float64 { return &v }

type fakeRepo struct {
	userBranches map[string][]branchauth.Branch
	items        map[int]Penunjang
	prices       map[int]map[string]Price
	listedFor    []string
}

func newFake() *fakeRepo {
	return &fakeRepo{
		userBranches: map[string][]branchauth.Branch{csID: {{Kode: "MED", Nama: "KLINIK MEDIKA"}, {Kode: "MIT", Nama: "KLINIK MITRA CAKUNG"}}},
		items:        map[int]Penunjang{1: {ID: 1, Nama: "ASAM URAT"}},
		prices: map[int]map[string]Price{1: {
			"MED": {Cabang: "MED", HJual: f(77000), HJualJamkes: f(108000), IsActive: true},
			"CTM": {Cabang: "CTM", HJual: f(49500), IsActive: true},
		}},
	}
}

func (r *fakeRepo) Jenis(context.Context) ([]Option, error) {
	return []Option{{ID: 15, Name: "LAB MOHC"}}, nil
}

func (r *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return r.userBranches[userID], nil
}

func (r *fakeRepo) withPrices(x Penunjang, branches []string) Penunjang {
	x.Prices = []Price{}
	for _, b := range branches {
		if p, ok := r.prices[x.ID][b]; ok {
			x.Prices = append(x.Prices, p)
		}
	}
	return x
}

func (r *fakeRepo) List(_ context.Context, branches []string) ([]Penunjang, error) {
	r.listedFor = branches
	out := []Penunjang{}
	for id := 1; id <= len(r.items); id++ {
		out = append(out, r.withPrices(r.items[id], branches))
	}
	return out, nil
}

func (r *fakeRepo) Get(_ context.Context, id int, branches []string) (Penunjang, error) {
	x, ok := r.items[id]
	if !ok {
		return Penunjang{}, ErrNotFound
	}
	return r.withPrices(x, branches), nil
}

func (r *fakeRepo) Create(_ context.Context, in Input, _ string) (int, error) {
	if in.IDJenis != nil && *in.IDJenis == 999 {
		return 0, ErrUnknownJenis
	}
	id := len(r.items) + 1
	r.items[id] = Penunjang{ID: id, Nama: in.Nama, IsActive: in.IsActive}
	return id, nil
}

func (r *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	r.items[id] = Penunjang{ID: id, Nama: in.Nama}
	return nil
}

func (r *fakeRepo) UpsertPrice(_ context.Context, id int, cabang string, p PriceInput) error {
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	if r.prices[id] == nil {
		r.prices[id] = map[string]Price{}
	}
	r.prices[id][cabang] = Price{Cabang: cabang, HJual: p.HJual, HJualJamkes: p.HJualJamkes, IsActive: p.IsActive}
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
	if guarded != 5 {
		t.Fatalf("%d routes guarded, want 5", guarded)
	}
	return &server{mux: mux, tokens: tokens, repo: repo}
}

func (s *server) do(t *testing.T, userID, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	tok, _, err := s.tokens.Generate(userID, "user@medika.com")
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestListShowsOnlyAuthorizedBranches(t *testing.T) {
	s := newServer(t)
	code, body := s.do(t, csID, "GET", "/api/penunjang", nil)
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	if got := strings.Join(s.repo.listedFor, ","); got != "MED,MIT" {
		t.Errorf("listed for %q, want MED,MIT", got)
	}
	prices := body["penunjang"].([]any)[0].(map[string]any)["prices"].([]any)
	if len(prices) != 1 || prices[0].(map[string]any)["cabang"] != "MED" || prices[0].(map[string]any)["hjualJamkes"] != 108000.0 {
		t.Errorf("prices = %v (CTM must not leak)", prices)
	}

	_, body = s.do(t, loneID, "GET", "/api/penunjang", nil)
	branches, ok := body["branches"].([]any)
	if !ok || len(branches) != 0 || len(body["penunjang"].([]any)[0].(map[string]any)["prices"].([]any)) != 0 {
		t.Errorf("unassigned user: %v (branches must be [], not null)", body)
	}
}

func TestSetPriceRequiresBranch(t *testing.T) {
	s := newServer(t)
	body := map[string]any{"hjual": 77000, "hjualJamkes": 110000, "isActive": true}

	if code, out := s.do(t, csID, "PUT", "/api/penunjang/1/price/mit", body); code != http.StatusOK {
		t.Fatalf("own branch: got %d %v", code, out)
	}
	if p := s.repo.prices[1]["MIT"]; p.HJualJamkes == nil || *p.HJualJamkes != 110000 {
		t.Errorf("price not saved: %+v", p)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/penunjang/1/price/CTM", body); code != http.StatusForbidden {
		t.Errorf("other branch: got %d, want 403", code)
	}
	if code, _ := s.do(t, loneID, "PUT", "/api/penunjang/1/price/MED", body); code != http.StatusForbidden {
		t.Errorf("unassigned user: got %d, want 403", code)
	}
	if code, out := s.do(t, csID, "PUT", "/api/penunjang/1/price/MED", map[string]any{"hjual": -1}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("negative price: got %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/penunjang/99/price/MED", body); code != http.StatusNotFound {
		t.Errorf("unknown penunjang: got %d", code)
	}
}

func TestCreateUpdate(t *testing.T) {
	s := newServer(t)
	if code, out := s.do(t, csID, "POST", "/api/penunjang", map[string]any{"nama": " HBA1C ", "isActive": true}); code != http.StatusCreated || out["nama"] != "HBA1C" {
		t.Fatalf("create: got %d %v", code, out)
	}
	if code, out := s.do(t, csID, "POST", "/api/penunjang", map[string]any{"nama": ""}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty nama: got %d %v", code, out)
	}
	if code, out := s.do(t, csID, "POST", "/api/penunjang", map[string]any{"nama": "X", "idJenis": 999}); code != http.StatusBadRequest ||
		out["errors"].(map[string]any)["idJenis"] == nil {
		t.Errorf("unknown jenis: got %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/penunjang/1", map[string]any{"nama": "ASAM URAT DARAH"}); code != http.StatusOK {
		t.Errorf("update: got %d", code)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/penunjang/99", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: got %d", code)
	}
	if code, out := s.do(t, csID, "GET", "/api/penunjang/options", nil); code != http.StatusOK || len(out["jenis"].([]any)) != 1 {
		t.Errorf("options: got %d %v", code, out)
	}
}
