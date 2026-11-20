package penerimaan

import (
	"context"
	"encoding/json"
	"fmt"
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

// 23:30 UTC on 18 Nov is already 19 Nov in Jakarta.
var now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)

type fakeRepo struct {
	branches map[string][]branchauth.Branch
	last     ListParams
	calls    int
	saved    []SaveInput
	savedBy  string
	saveErr  error
	obatFor  string
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.branches[userID], nil
}
func (f *fakeRepo) Suppliers(context.Context) ([]Option, error) {
	return []Option{{ID: 3, Name: "CV MEDIKA GROUP MANDIRI"}}, nil
}
func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.last, f.calls = p, f.calls+1
	tgl := "2026-11-20"
	return Page{Rows: []Row{{ID: 13908, NoGR: "PM/11/20/2026", TglTrans: &tgl, TglGR: &tgl, Total: 732000, Cabang: "KMJ"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) Distributors(context.Context) ([]Distributor, error) {
	return []Distributor{{ID: 1, Nama: "-", Alamat: "-"}, {ID: 2, Nama: "APOTEK MEDICASTORE", Alamat: "-"}}, nil
}
func (f *fakeRepo) Obat(_ context.Context, branch string) ([]Obat, error) {
	f.obatFor = branch
	return []Obat{{ID: 1, Nama: "ACESSORIES TINDAKAN > 25 SIMPUL", Jenis: "ETIKAL", Satuan: "SIMPUL", Harga: 1, Stok: 505}}, nil
}
func (f *fakeRepo) Save(_ context.Context, in SaveInput, user string) (SaveResult, error) {
	if f.saveErr != nil {
		return SaveResult{}, f.saveErr
	}
	f.saved, f.savedBy = append(f.saved, in), user
	nc := 261113910
	return SaveResult{ID: 13910, Total: 1, NoClose: &nc}, nil
}

func (f *fakeRepo) Detail(_ context.Context, id int) (Detail, error) {
	tgl, exp := "2026-11-08", "2027-11-08"
	switch id {
	case 13809:
		return Detail{ID: id, NoGR: "MJ/ETC/08112026", TglGR: &tgl, Cabang: "KMJ", Distributor: "-", Total: 875000,
			Obat: []DetailLine{{IDObat: 1317, Nama: "EFLIN TABLET", Harga: 125000, NoBatch: "0", Byk: 7, TglExp: &exp}}}, nil
	case 13808:
		return Detail{ID: id, NoGR: "ctm", Cabang: "CTM", Obat: []DetailLine{}}, nil
	}
	return Detail{}, ErrNotFound
}

type server struct {
	mux  *http.ServeMux
	repo *fakeRepo
	tok  *auth.TokenManager
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := &fakeRepo{branches: map[string][]branchauth.Branch{
		csID: {{Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}},
	}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q, want %q", path, PagePath)
		}
		guarded++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 6 {
		t.Fatalf("%d routes guarded, want 6", guarded)
	}
	return &server{mux: mux, repo: repo, tok: tokens}
}

func (s *server) get(t *testing.T, userID, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if userID != "" {
		tok, _, _ := s.tok.Generate(userID, "cs@medika.com")
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestList(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/penerimaan-barang")
	p := s.repo.last
	if code != http.StatusOK || p.From != "2026-11-19" || p.To != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" ||
		p.Sort != "id" || !p.Desc || p.Limit != 10 || p.IDSupplier != nil {
		t.Fatalf("default (today in Jakarta, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["noGr"] != "PM/11/20/2026" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/penerimaan-barang?from=2026-10-22&to=2026-11-20&cabang=kmj&noGr=+CTM+&supplier=0&q=x&sort=total&dir=asc&size=20&page=3")
	p = s.repo.last
	if p.From != "2026-10-22" || p.To != "2026-11-20" || strings.Join(p.Branches, ",") != "KMJ" || p.NoGR != "CTM" ||
		p.IDSupplier == nil || *p.IDSupplier != 0 || p.Search != "x" || p.Sort != "total" || p.Desc || p.Offset != 40 {
		t.Errorf("filtered (supplier 0 = none): %+v", p)
	}
	s.get(t, csID, "/api/penerimaan-barang?sort=evil&size=999999&supplier=-1")
	if p = s.repo.last; p.Sort != "id" || !p.Desc || p.Limit != MaxPageSize || p.IDSupplier != nil {
		t.Errorf("unknown sort / huge size / bad supplier: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/penerimaan-barang?cabang=CTM":                    http.StatusForbidden,
		"/api/penerimaan-barang?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
		"/api/penerimaan-barang?from=bad":                      http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.calls != before {
		t.Errorf("rejected requests reached the repository")
	}
	if code, out := s.get(t, loneID, "/api/penerimaan-barang"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("user without branches: %d %v", code, out)
	}
	if code, _ := s.get(t, "", "/api/penerimaan-barang"); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestOptions(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/penerimaan-barang/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["suppliers"].([]any)) != 1 || len(out["branches"].([]any)) != 2 {
		t.Fatalf("options: %d %v", code, out)
	}
}

func (s *server) post(t *testing.T, userID, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	tok, _, _ := s.tok.Generate(userID, "a-very-long-email-address-for-goods@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestLookups(t *testing.T) {
	s := newServer(t)
	if code, out := s.get(t, csID, "/api/penerimaan-barang/distributor"); code != http.StatusOK || len(out["rows"].([]any)) != 2 {
		t.Errorf("distributor: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/penerimaan-barang/obat?cabang=kmj"); code != http.StatusOK || s.repo.obatFor != "KMJ" || len(out["rows"].([]any)) != 1 {
		t.Errorf("obat: %d %v", code, out)
	}
	for path, want := range map[string]int{
		"/api/penerimaan-barang/obat":            http.StatusBadRequest,
		"/api/penerimaan-barang/obat?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
}

func TestSave(t *testing.T) {
	s := newServer(t)
	valid := `{"cabang":"kmj","idDistributor":2,"noGr":" GR/01 ","tglGr":"2026-11-19",
		"obat":[{"idObat":1,"noBatch":" ","byk":3,"tglExp":"2027-11-19","disc":10}]}`
	code, out := s.post(t, csID, "/api/penerimaan-barang", valid)
	if code != http.StatusCreated || out["id"] != 13910.0 || out["noClose"] != 261113910.0 {
		t.Fatalf("save: %d %v", code, out)
	}
	in := s.repo.saved[0]
	// Trimmed, upper-cased branch, default batch "0"; the user without its domain, cut to 20.
	if in.Cabang != "KMJ" || in.NoGR != "GR/01" || in.Obat[0].NoBatch != "0" || s.repo.savedBy != "a-very-long-email-ad" {
		t.Errorf("normalized input: %+v by %q", in, s.repo.savedBy)
	}
	for name, body := range map[string]string{
		"no cabang":         `{"idDistributor":2,"noGr":"X","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"tglExp":"2027-01-01"}]}`,
		"no distributor":    `{"cabang":"KMJ","noGr":"X","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"tglExp":"2027-01-01"}]}`,
		"no GR":             `{"cabang":"KMJ","idDistributor":2,"noGr":"  ","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"tglExp":"2027-01-01"}]}`,
		"GR too long":       `{"cabang":"KMJ","idDistributor":2,"noGr":"1234567890123456","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"tglExp":"2027-01-01"}]}`,
		"future date":       `{"cabang":"KMJ","idDistributor":2,"noGr":"X","tglGr":"2026-11-20","obat":[{"idObat":1,"byk":1,"tglExp":"2027-01-01"}]}`,
		"no lines":          `{"cabang":"KMJ","idDistributor":2,"noGr":"X","tglGr":"2026-11-19","obat":[]}`,
		"zero qty":          `{"cabang":"KMJ","idDistributor":2,"noGr":"X","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":0,"tglExp":"2027-01-01"}]}`,
		"discount over 100": `{"cabang":"KMJ","idDistributor":2,"noGr":"X","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"disc":101,"tglExp":"2027-01-01"}]}`,
		"bad expiry":        `{"cabang":"KMJ","idDistributor":2,"noGr":"X","tglGr":"2026-11-19","obat":[{"idObat":1,"byk":1,"tglExp":"soon"}]}`,
		"not json":          `{`,
	} {
		if code, _ := s.post(t, csID, "/api/penerimaan-barang", body); code != http.StatusBadRequest {
			t.Errorf("%s: %d, want 400", name, code)
		}
	}
	if code, _ := s.post(t, csID, "/api/penerimaan-barang", strings.Replace(valid, `"kmj"`, `"CTM"`, 1)); code != http.StatusForbidden {
		t.Errorf("another branch: %d", code)
	}
	if len(s.repo.saved) != 1 {
		t.Fatalf("rejected input was saved: %d saves", len(s.repo.saved))
	}
	s.repo.saveErr = &ItemError{IDObat: 99, Msg: "tidak ada di stok cabang KMJ"}
	if code, out := s.post(t, csID, "/api/penerimaan-barang", valid); code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(out), "obat 99") {
		t.Errorf("drug not stocked: %d %v", code, out)
	}
	s.repo.saveErr = ErrUnknownDistributor
	if code, _ := s.post(t, csID, "/api/penerimaan-barang", valid); code != http.StatusBadRequest {
		t.Errorf("unknown distributor: %d", code)
	}
}

func TestNoClose(t *testing.T) {
	tgl := time.Date(2026, 11, 20, 10, 0, 0, 0, time.UTC)
	if n := noClose(tgl, 13809); n == nil || *n != 261113809 {
		t.Errorf("noClose(13809) = %v", n)
	}
	// YYMM + a 6-digit id no longer fits an integer column.
	if n := noClose(tgl, 999999); n != nil {
		t.Errorf("noClose(999999) = %v, want nil", *n)
	}
}

func TestDetail(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/penerimaan-barang/detail/13809")
	if code != http.StatusOK || out["noGr"] != "MJ/ETC/08112026" || len(out["obat"].([]any)) != 1 {
		t.Fatalf("detail: %d %v", code, out)
	}
	for path, want := range map[string]int{
		"/api/penerimaan-barang/detail/13808": http.StatusNotFound, // another branch's
		"/api/penerimaan-barang/detail/1":     http.StatusNotFound,
		"/api/penerimaan-barang/detail/abc":   http.StatusNotFound,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
}
