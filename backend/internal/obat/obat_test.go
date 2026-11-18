package obat

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

const (
	nurseID = "22222222-2222-2222-2222-222222222222" // mapped to MED and PRM
	loneID  = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func f(v float64) *float64 { return &v }

// fakeRepo keeps drugs and stock in memory and records what List was asked for.
type fakeRepo struct {
	userBranches map[string][]Branch
	obat         map[int]Obat
	stock        map[int]map[string]Stock
	listedFor    []string
	lastUser     string
}

func newFake() *fakeRepo {
	return &fakeRepo{
		userBranches: map[string][]Branch{nurseID: {{"MED", "KLINIK MEDIKA"}, {"PRM", "KLINIK PRATAMA PRIMA MEDIKA"}}},
		obat:         map[int]Obat{1: {ID: 1, Nama: "PARACETAMOL"}},
		stock: map[int]map[string]Stock{1: {
			"MED": {Cabang: "MED", Stok: f(10), IsActive: true},
			"CTM": {Cabang: "CTM", Stok: f(505), IsActive: true},
		}},
	}
}

func (r *fakeRepo) Options(context.Context) (Options, error) { return Options{}, nil }

func (r *fakeRepo) UserBranches(_ context.Context, userID string) ([]Branch, error) {
	return r.userBranches[userID], nil
}

func (r *fakeRepo) withStock(o Obat, branches []string) Obat {
	o.Stocks = []Stock{}
	for _, b := range branches {
		if s, ok := r.stock[o.ID][b]; ok {
			o.Stocks = append(o.Stocks, s)
		}
	}
	return o
}

func (r *fakeRepo) List(_ context.Context, branches []string) ([]Obat, error) {
	r.listedFor = branches
	out := []Obat{}
	for id := 1; id <= len(r.obat); id++ {
		out = append(out, r.withStock(r.obat[id], branches))
	}
	return out, nil
}

func (r *fakeRepo) Get(_ context.Context, id int, branches []string) (Obat, error) {
	o, ok := r.obat[id]
	if !ok {
		return Obat{}, ErrNotFound
	}
	return r.withStock(o, branches), nil
}

func (r *fakeRepo) Create(_ context.Context, in Input, user string) (int, error) {
	if in.IDJenis != nil && *in.IDJenis == 999 {
		return 0, InvalidRefError{Field: "idJenis"}
	}
	id := len(r.obat) + 1
	r.obat[id] = Obat{ID: id, Nama: in.Nama}
	r.lastUser = user
	return id, nil
}

func (r *fakeRepo) Update(_ context.Context, id int, in Input, user string) error {
	if _, ok := r.obat[id]; !ok {
		return ErrNotFound
	}
	r.obat[id] = Obat{ID: id, Nama: in.Nama}
	r.lastUser = user
	return nil
}

func (r *fakeRepo) UpsertStock(_ context.Context, id int, cabang string, s StockInput) error {
	if _, ok := r.obat[id]; !ok {
		return ErrNotFound
	}
	if r.stock[id] == nil {
		r.stock[id] = map[string]Stock{}
	}
	r.stock[id][cabang] = Stock{Cabang: cabang, Stok: s.Stok, HJual: s.HJual, IsActive: s.IsActive}
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

func (s *server) do(t *testing.T, userID, email, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	tok, _, err := s.tokens.Generate(userID, email)
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
	code, body := s.do(t, nurseID, "perawat@medika.com", "GET", "/api/obat", nil)
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	if got := strings.Join(s.repo.listedFor, ","); got != "MED,PRM" {
		t.Errorf("listed for %q, want MED,PRM", got)
	}
	stocks := body["obat"].([]any)[0].(map[string]any)["stocks"].([]any)
	if len(stocks) != 1 || stocks[0].(map[string]any)["cabang"] != "MED" {
		t.Errorf("stocks = %v (CTM must not leak)", stocks)
	}
	if len(body["branches"].([]any)) != 2 {
		t.Errorf("branches = %v", body["branches"])
	}

	// A user without branches sees the drugs but no stock, and an empty branch list.
	_, body = s.do(t, loneID, "x@medika.com", "GET", "/api/obat", nil)
	if len(body["branches"].([]any)) != 0 || len(body["obat"].([]any)[0].(map[string]any)["stocks"].([]any)) != 0 {
		t.Errorf("unassigned user: %v", body)
	}
}

func TestSetStockRequiresBranch(t *testing.T) {
	s := newServer(t)
	body := map[string]any{"stok": 25, "hjual": 175000, "isActive": true}

	code, out := s.do(t, nurseID, "perawat@medika.com", "PUT", "/api/obat/1/stock/prm", body)
	if code != http.StatusOK {
		t.Fatalf("own branch: got %d %v", code, out)
	}
	if st := s.repo.stock[1]["PRM"]; st.Stok == nil || *st.Stok != 25 {
		t.Errorf("stock not saved: %+v", st)
	}
	if len(out["stocks"].([]any)) != 2 {
		t.Errorf("response stocks = %v", out["stocks"])
	}

	if code, _ := s.do(t, nurseID, "perawat@medika.com", "PUT", "/api/obat/1/stock/CTM", body); code != http.StatusForbidden {
		t.Errorf("other branch: got %d, want 403", code)
	}
	if code, _ := s.do(t, loneID, "x@medika.com", "PUT", "/api/obat/1/stock/MED", body); code != http.StatusForbidden {
		t.Errorf("unassigned user: got %d, want 403", code)
	}
	if code, out := s.do(t, nurseID, "perawat@medika.com", "PUT", "/api/obat/1/stock/MED", map[string]any{"stok": -1}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("negative stock: got %d %v", code, out)
	}
	if code, _ := s.do(t, nurseID, "perawat@medika.com", "PUT", "/api/obat/99/stock/MED", body); code != http.StatusNotFound {
		t.Errorf("unknown obat: got %d", code)
	}
}

func TestCreateUpdate(t *testing.T) {
	s := newServer(t)
	email := "a-very-long-email-address@medika.com"
	code, out := s.do(t, nurseID, email, "POST", "/api/obat", map[string]any{"nama": " AMOXICILLIN ", "hjual": 5000})
	if code != http.StatusCreated || out["nama"] != "AMOXICILLIN" {
		t.Fatalf("create: got %d %v", code, out)
	}
	if s.repo.lastUser != email[:20] {
		t.Errorf("create_user = %q, want the email cut to 20 characters", s.repo.lastUser)
	}
	if code, out := s.do(t, nurseID, email, "POST", "/api/obat", map[string]any{"nama": ""}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty nama: got %d %v", code, out)
	}
	if code, out := s.do(t, nurseID, email, "POST", "/api/obat", map[string]any{"nama": "X", "idJenis": 999}); code != http.StatusBadRequest ||
		out["errors"].(map[string]any)["idJenis"] == nil {
		t.Errorf("unknown jenis: got %d %v", code, out)
	}
	if code, out := s.do(t, nurseID, email, "POST", "/api/obat", map[string]any{"nama": strings.Repeat("n", 101)}); code != http.StatusBadRequest {
		t.Errorf("long nama: got %d %v", code, out)
	}
	if code, _ := s.do(t, nurseID, email, "PUT", "/api/obat/1", map[string]any{"nama": "PARACETAMOL 500"}); code != http.StatusOK {
		t.Errorf("update: got %d", code)
	}
	if code, _ := s.do(t, nurseID, email, "PUT", "/api/obat/99", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: got %d", code)
	}
}
