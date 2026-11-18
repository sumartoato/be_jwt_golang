package pasien

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

func ptr[T any](v T) *T { return &v }

var today = time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)

func TestFormatNoRM(t *testing.T) {
	for n, want := range map[int]string{890117: "89-01-17", 4700: "00-47-00", 1: "00-00-01", 999999: "99-99-99"} {
		if got := FormatNoRM(n); got != want {
			t.Errorf("FormatNoRM(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	in := Input{Nama: " SENO AJI ", TmptLahir: "CILACAP", TglLahir: ptr("2006-02-02"), Alamat: "PAMUNUNTAN", IDGender: ptr(1), Cabang: " ctm ", TglKun: ptr(" ")}
	if errs := in.Normalize(today); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if in.Nama != "SENO AJI" || in.Cabang != "CTM" || in.TglKun != nil {
		t.Fatalf("not normalized: %+v", in)
	}

	cases := []struct {
		mod   func(*Input)
		field string
	}{
		{func(i *Input) { i.Nama = "" }, "nama"},
		{func(i *Input) { i.Nama = strings.Repeat("n", 51) }, "nama"},
		{func(i *Input) { i.TglLahir = nil }, "tglLahir"},
		{func(i *Input) { i.TglLahir = ptr("02/02/2006") }, "tglLahir"},
		{func(i *Input) { i.TglLahir = ptr("2027-01-01") }, "tglLahir"},
		{func(i *Input) { i.IDGender = nil }, "idGender"},
		{func(i *Input) { i.Cabang = "" }, "cabang"},
		{func(i *Input) { i.RT = "12345" }, "rt"},
		{func(i *Input) { i.NIK = strings.Repeat("1", 31) }, "nik"},
	}
	for i, c := range cases {
		x := Input{Nama: "A", TmptLahir: "B", TglLahir: ptr("2000-01-01"), Alamat: "C", IDGender: ptr(1), Cabang: "MED"}
		c.mod(&x)
		if errs := x.Normalize(today); errs[c.field] == "" {
			t.Errorf("case %d: want error on %s, got %v", i, c.field, errs)
		}
	}
}

func TestSearchHelpers(t *testing.T) {
	if got := escapeLike(`50%_a\b`); got != `50\%\_a\\b` {
		t.Errorf("escapeLike = %q", got)
	}
	if id, err := parseID("939858"); err != nil || id != 939858 {
		t.Errorf("parseID(939858) = %d, %v", id, err)
	}
	for _, s := range []string{"89-01-17", "12a", "0", "-5"} {
		if _, err := parseID(s); err == nil {
			t.Errorf("parseID(%q) should fail", s)
		}
	}
	sql, args := where(ListParams{Branches: []string{"MED"}, Search: "123"}, true)
	if !strings.Contains(sql, "id = $3") || len(args) != 3 {
		t.Errorf("numeric search should also match the id: %s %v", sql, args)
	}
}

// fakeRepo keeps patients in memory.
type fakeRepo struct {
	userBranches map[string][]branchauth.Branch
	items        map[int]Pasien
	lastList     ListParams
	created      []Input
}

func newFake() *fakeRepo {
	return &fakeRepo{
		userBranches: map[string][]branchauth.Branch{csID: {{Kode: "MED", Nama: "KLINIK MEDIKA"}, {Kode: "MIT", Nama: "KLINIK MITRA CAKUNG"}}},
		items: map[int]Pasien{
			1: {Row: Row{ID: 1, NoRM: "01-00-01", Nama: "PASIEN MED", Cabang: "MED"}},
			2: {Row: Row{ID: 2, NoRM: "89-01-17", Nama: "PASIEN CTM", Cabang: "CTM"}},
		},
	}
}

func (r *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return r.userBranches[userID], nil
}

func (r *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	r.lastList = p
	page := Page{Rows: []Row{}}
	for id := 1; id <= len(r.items); id++ {
		x := r.items[id]
		if contains(p.Branches, x.Cabang) {
			page.Rows = append(page.Rows, x.Row)
		}
	}
	page.Total, page.UnfilteredTotal = len(page.Rows), len(page.Rows)
	return page, nil
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func (r *fakeRepo) Get(_ context.Context, id int) (Pasien, error) {
	x, ok := r.items[id]
	if !ok {
		return Pasien{}, ErrNotFound
	}
	return x, nil
}

func (r *fakeRepo) Create(_ context.Context, in Input, _ string) (int, error) {
	if in.IDJamkes != nil && *in.IDJamkes == 999 {
		return 0, InvalidRefError{Field: "idJamkes"}
	}
	r.created = append(r.created, in)
	id := len(r.items) + 1
	r.items[id] = Pasien{Row: Row{ID: id, NoRM: "01-00-02", Nama: in.Nama, Cabang: in.Cabang}}
	return id, nil
}

func (r *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	x := r.items[id]
	x.Nama, x.Cabang = in.Nama, in.Cabang
	r.items[id] = x
	return nil
}

func (r *fakeRepo) Options(context.Context) (Options, error) {
	return Options{Gender: []Option{{ID: 1, Name: "Laki-laki", IsActive: true}}}, nil
}

func (r *fakeRepo) SearchKelurahan(_ context.Context, q string, _ int) ([]Kelurahan, error) {
	return []Kelurahan{{ID: 1, Kelurahan: strings.ToUpper(q)}}, nil
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
	h := NewHandler(repo)
	h.now = func() time.Time { return today }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 6 {
		t.Fatalf("%d routes guarded, want 6", guarded)
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

func TestListIsLimitedToBranches(t *testing.T) {
	s := newServer(t)
	code, body := s.do(t, csID, "GET", "/api/pasien?size=99999&page=0", nil)
	if code != http.StatusOK {
		t.Fatalf("got %d", code)
	}
	lp := s.repo.lastList
	if strings.Join(lp.Branches, ",") != "MED,MIT" || lp.Limit != MaxPageSize || lp.Offset != 0 || lp.Sort != "id" || !lp.Desc {
		t.Errorf("list params = %+v (want the user's branches, size capped, newest first)", lp)
	}
	if rows := body["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["nama"] != "PASIEN MED" {
		t.Errorf("rows = %v (CTM must not leak)", rows)
	}

	s.do(t, csID, "GET", "/api/pasien?cabang=mit&sort=nama&dir=asc&size=10&page=3&q=+seno+", nil)
	lp = s.repo.lastList
	if strings.Join(lp.Branches, ",") != "MIT" || lp.Sort != "nama" || lp.Desc || lp.Offset != 20 || lp.Search != "seno" {
		t.Errorf("filtered list params = %+v", lp)
	}
	s.do(t, csID, "GET", "/api/pasien?sort=password;drop", nil)
	if s.repo.lastList.Sort != "id" {
		t.Errorf("unknown sort key must fall back to id, got %q", s.repo.lastList.Sort)
	}

	if code, _ := s.do(t, csID, "GET", "/api/pasien?cabang=CTM", nil); code != http.StatusForbidden {
		t.Errorf("other branch filter: got %d, want 403", code)
	}
	code, body = s.do(t, loneID, "GET", "/api/pasien", nil)
	if code != http.StatusOK || body["total"] != 0.0 || len(body["branches"].([]any)) != 0 {
		t.Errorf("unassigned user: %d %v", code, body)
	}
}

func TestGetAndUpdateRespectBranches(t *testing.T) {
	s := newServer(t)
	if code, _ := s.do(t, csID, "GET", "/api/pasien/1", nil); code != http.StatusOK {
		t.Errorf("own branch: got %d", code)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pasien/2", nil); code != http.StatusNotFound {
		t.Errorf("other branch must look like not found: got %d", code)
	}

	valid := map[string]any{"nama": "PASIEN MED 2", "tmptLahir": "JAKARTA", "tglLahir": "1990-05-01", "alamat": "JL. A", "idGender": 1, "cabang": "MED"}
	if code, out := s.do(t, csID, "PUT", "/api/pasien/1", valid); code != http.StatusOK || out["nama"] != "PASIEN MED 2" {
		t.Errorf("update: got %d %v", code, out)
	}
	moved := map[string]any{"nama": "X", "tmptLahir": "J", "tglLahir": "1990-05-01", "alamat": "A", "idGender": 1, "cabang": "CTM"}
	if code, _ := s.do(t, csID, "PUT", "/api/pasien/1", moved); code != http.StatusForbidden {
		t.Errorf("move to another user's branch: got %d, want 403", code)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/pasien/2", valid); code != http.StatusNotFound {
		t.Errorf("update other branch: got %d, want 404", code)
	}
}

func TestCreate(t *testing.T) {
	s := newServer(t)
	body := map[string]any{"nama": "BARU", "tmptLahir": "BOGOR", "tglLahir": "2001-01-25", "alamat": "JL. B", "idGender": 2, "cabang": "mit"}
	code, out := s.do(t, csID, "POST", "/api/pasien", body)
	if code != http.StatusCreated || out["noRm"] == "" || out["cabang"] != "MIT" {
		t.Fatalf("create: got %d %v", code, out)
	}
	if code, out := s.do(t, csID, "POST", "/api/pasien", map[string]any{"cabang": "MED"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("missing fields: got %d %v", code, out)
	}
	body["cabang"] = "CTM"
	if code, _ := s.do(t, csID, "POST", "/api/pasien", body); code != http.StatusForbidden {
		t.Errorf("other branch: got %d, want 403", code)
	}
	body["cabang"], body["idJamkes"] = "MED", 999
	if code, out := s.do(t, csID, "POST", "/api/pasien", body); code != http.StatusBadRequest || out["errors"].(map[string]any)["idJamkes"] == nil {
		t.Errorf("unknown jamkes: got %d %v", code, out)
	}
}

func TestKelurahanAndOptions(t *testing.T) {
	s := newServer(t)
	if _, out := s.do(t, csID, "GET", "/api/pasien/kelurahan?q=a", nil); len(out["kelurahan"].([]any)) != 0 {
		t.Errorf("one-letter search should return nothing: %v", out)
	}
	if _, out := s.do(t, csID, "GET", "/api/pasien/kelurahan?q=kebon", nil); len(out["kelurahan"].([]any)) != 1 {
		t.Errorf("search: %v", out)
	}
	if code, out := s.do(t, csID, "GET", "/api/pasien/options", nil); code != http.StatusOK || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}
