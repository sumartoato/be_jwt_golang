package pendaftaran

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
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to MED and KMJ
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func ptr[T any](v T) *T { return &v }

// 23:30 UTC on 17 Nov is already 18 Nov in Jakarta.
var now = time.Date(2026, 11, 17, 23, 30, 0, 0, time.UTC)

func TestClinicToday(t *testing.T) {
	if got := ClinicToday(now); got != "2026-11-18" {
		t.Errorf("ClinicToday = %s, want 2026-11-18 (Asia/Jakarta)", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("a-very-long-email-address@medika.com", 20); got != "a-very-long-email-ad" {
		t.Errorf("truncate = %q", got)
	}
	if got := truncate("cs@medika.com", 20); got != "cs@medika.com" {
		t.Errorf("short value changed: %q", got)
	}
}

func TestWhere(t *testing.T) {
	sql, args := where(ListParams{Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-18", Search: "518556", IDJenis: ptr(2)}, true)
	for _, want := range []string{"d.cabang = ANY($1)", "d.tgl_masuk >= $2::date", "d.tgl_masuk < $3::date + 1", "d.id_jenis_rwt = $4", "d.id = $6"} {
		if !strings.Contains(sql, want) {
			t.Errorf("where missing %q: %s", want, sql)
		}
	}
	if len(args) != 6 {
		t.Errorf("args = %v", args)
	}
}

type fakeRepo struct {
	userBranches     map[string][]branchauth.Branch
	rows             map[int]Row
	lastList         ListParams
	lastPasienQ      []string
	lastPasienSearch PasienSearch
	created          []NewInput
}

func newFake() *fakeRepo {
	return &fakeRepo{
		userBranches: map[string][]branchauth.Branch{csID: {{Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}}},
		rows: map[int]Row{
			518556: {ID: 518556, Cabang: "KMJ", IDStatus: ptr(StatusDaftar), IDJenis: ptr(2)},
			518543: {ID: 518543, Cabang: "CTM", IDStatus: ptr(StatusDaftar)},
			500000: {ID: 500000, Cabang: "KMJ", IDStatus: ptr(2)}, // CLOSE
		},
	}
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.userBranches[userID], nil
}
func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.lastList = p
	return Page{Rows: []Row{}}, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (Row, error) {
	r, ok := f.rows[id]
	if !ok {
		return Row{}, ErrNotFound
	}
	return r, nil
}
func (f *fakeRepo) Services(context.Context) ([]Service, error) {
	return []Service{{ID: 2, Name: "POLI UMUM", NeedQueue: true, FirstCode: "U"}}, nil
}
func (f *fakeRepo) Statuses(context.Context) ([]Option, error) {
	return []Option{{ID: 3, Name: "DAFTAR"}}, nil
}
func (f *fakeRepo) SearchPasien(_ context.Context, branches []string, s PasienSearch, _, _ int) (PasienPage, error) {
	f.lastPasienQ = append(branches, s.Q)
	f.lastPasienSearch = s
	return PasienPage{Rows: []PasienHit{}}, nil
}
func (f *fakeRepo) Create(_ context.Context, in NewInput, _ string) (int, error) {
	if in.IDPas == 404 {
		return 0, ErrUnknownPasien
	}
	f.created = append(f.created, in)
	id := 600000 + len(f.created)
	f.rows[id] = Row{ID: id, Cabang: in.Cabang, IDStatus: ptr(StatusDaftar), NoAntri: ptr(1), NoAntriPoli: ptr("U001")}
	return id, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in EditInput, _ string) error {
	r := f.rows[id]
	if *r.IDStatus != StatusDaftar {
		return ErrNotEditable
	}
	r.IDJenis, r.PakaiKartu = &in.IDJenis, in.PakaiKartu
	f.rows[id] = r
	return nil
}
func (f *fakeRepo) Cancel(_ context.Context, id int, reason, _ string) error {
	r := f.rows[id]
	if *r.IDStatus != StatusDaftar {
		return ErrNotEditable
	}
	r.IDStatus, r.CancelNote = ptr(StatusCancel), reason
	f.rows[id] = r
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
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
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
	tok, _, _ := s.tokens.Generate(userID, "cs@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestList(t *testing.T) {
	s := newServer(t)
	code, out := s.do(t, csID, "GET", "/api/pendaftaran", nil)
	lp := s.repo.lastList
	if code != http.StatusOK || lp.From != "2026-11-18" || lp.To != "2026-11-18" || strings.Join(lp.Branches, ",") != "KMJ,MED" || lp.Sort != "id" || !lp.Desc {
		t.Fatalf("default list: %d %v %+v", code, out, lp)
	}
	s.do(t, csID, "GET", "/api/pendaftaran?from=2026-11-01&to=2026-11-07&cabang=med&pelayanan=2&status=3&sort=nama&dir=asc&size=20&page=2", nil)
	lp = s.repo.lastList
	if lp.From != "2026-11-01" || lp.To != "2026-11-07" || strings.Join(lp.Branches, ",") != "MED" || *lp.IDJenis != 2 || *lp.IDStatus != 3 || lp.Sort != "nama" || lp.Desc || lp.Offset != 20 {
		t.Errorf("filtered list: %+v", lp)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pendaftaran?cabang=CTM", nil); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pendaftaran?from=2026-11-10&to=2026-11-01", nil); code != http.StatusBadRequest {
		t.Errorf("reversed range: %d", code)
	}
	if code, out := s.do(t, loneID, "GET", "/api/pendaftaran", nil); code != http.StatusOK || out["total"] != 0.0 {
		t.Errorf("unassigned user: %d %v", code, out)
	}
	if code, out := s.do(t, csID, "GET", "/api/pendaftaran/options", nil); code != http.StatusOK || out["today"] != "2026-11-18" || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
	s.do(t, csID, "GET", "/api/pendaftaran/pasien?q=+doni+", nil)
	if strings.Join(s.repo.lastPasienQ, ",") != "KMJ,MED,doni" {
		t.Errorf("patient search: %v", s.repo.lastPasienQ)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pendaftaran/pasien?by=alamat&key=+kp+jembatan+", nil); code != http.StatusOK ||
		s.repo.lastPasienSearch != (PasienSearch{By: "alamat", Key: "kp jembatan"}) {
		t.Errorf("field search: %d %+v", code, s.repo.lastPasienSearch)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pendaftaran/pasien?by=umur&key=13", nil); code != http.StatusOK ||
		s.repo.lastPasienSearch != (PasienSearch{By: "umur", Key: "13", Age: 13}) {
		t.Errorf("age search: %d %+v", code, s.repo.lastPasienSearch)
	}
	if code, _ := s.do(t, csID, "GET", "/api/pendaftaran/pasien?by=umur&key=", nil); code != http.StatusOK || s.repo.lastPasienSearch.By != "" {
		t.Errorf("empty age: %d %+v", code, s.repo.lastPasienSearch)
	}
	for _, bad := range []string{"by=umur&key=tiga", "by=umur&key=-1", "by=nik&key=1"} {
		if code, _ := s.do(t, csID, "GET", "/api/pendaftaran/pasien?"+bad, nil); code != http.StatusBadRequest {
			t.Errorf("%s: %d", bad, code)
		}
	}
}

func TestCreate(t *testing.T) {
	s := newServer(t)
	code, out := s.do(t, csID, "POST", "/api/pendaftaran", map[string]any{"idPas": 1, "idJenis": 2, "cabang": "kmj"})
	if code != http.StatusCreated || out["noAntriPoli"] != "U001" {
		t.Fatalf("create: %d %v", code, out)
	}
	if got := s.repo.created[0]; got.Tanggal != "2026-11-18" || got.Cabang != "KMJ" {
		t.Errorf("created %+v (date defaults to the clinic's today)", got)
	}
	if code, out := s.do(t, csID, "POST", "/api/pendaftaran", map[string]any{"tanggal": "2026-11-19", "idPas": 1, "idJenis": 2, "cabang": "KMJ"}); code != http.StatusBadRequest || out["errors"].(map[string]any)["tanggal"] == nil {
		t.Errorf("future date: %d %v", code, out)
	}
	if code, out := s.do(t, csID, "POST", "/api/pendaftaran", map[string]any{}); code != http.StatusBadRequest || len(out["errors"].(map[string]any)) != 3 {
		t.Errorf("missing fields: %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "POST", "/api/pendaftaran", map[string]any{"idPas": 1, "idJenis": 2, "cabang": "CTM"}); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	if code, out := s.do(t, csID, "POST", "/api/pendaftaran", map[string]any{"idPas": 404, "idJenis": 2, "cabang": "KMJ"}); code != http.StatusBadRequest || out["errors"].(map[string]any)["idPas"] == nil {
		t.Errorf("unknown patient: %d %v", code, out)
	}
}

func TestUpdateAndCancel(t *testing.T) {
	s := newServer(t)
	if code, out := s.do(t, csID, "PUT", "/api/pendaftaran/518556", map[string]any{"idJenis": 3, "pakaiKartu": true}); code != http.StatusOK || out["pakaiKartu"] != true {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/pendaftaran/518543", map[string]any{"idJenis": 3}); code != http.StatusNotFound {
		t.Errorf("other branch must look like not found: %d", code)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/pendaftaran/500000", map[string]any{"idJenis": 3}); code != http.StatusConflict {
		t.Errorf("closed registration: %d", code)
	}
	if code, _ := s.do(t, csID, "POST", "/api/pendaftaran/518556/cancel", map[string]any{"reason": " "}); code != http.StatusBadRequest {
		t.Errorf("cancel without reason: %d", code)
	}
	if code, out := s.do(t, csID, "POST", "/api/pendaftaran/518556/cancel", map[string]any{"reason": "Pasien batal datang"}); code != http.StatusOK || out["cancelReason"] != "Pasien batal datang" {
		t.Errorf("cancel: %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "POST", "/api/pendaftaran/518556/cancel", map[string]any{"reason": "again"}); code != http.StatusConflict {
		t.Errorf("cancel twice: %d", code)
	}
}
