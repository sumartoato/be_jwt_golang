package pengobatan

import (
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

// 23:30 UTC on 18 Nov is already 19 Nov in Jakarta.
var now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)

func TestWhere(t *testing.T) {
	sql, args := where(ListParams{
		Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-19", Search: "522830", Nama: "obat",
		IDJenisRawat: ptr(2), IDStatusKunjung: ptr(2), IDJenisBayar: ptr(1), Status: StatusCancel,
	}, true)
	for _, want := range []string{
		"j.cabang = ANY($1)", "j.tgl >= $2::date", "j.tgl < $3::date + 1", "p.nama ILIKE $4", "j.id_jenis_rwt = $5",
		"d.id = j.id_kun AND d.id_statustr = $6", "j.id_jenis_byr = $7", "ELSE 'open' END = $8", "j.no_kwi ILIKE $9", "j.id = $10",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("where missing %q: %s", want, sql)
		}
	}
	if len(args) != 10 || args[3] != "%obat%" || args[7] != "cancel" {
		t.Errorf("args = %v", args)
	}
	// The "filtered from" count keeps the panel filters and drops only the search box.
	sql, args = where(ListParams{Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-18", Search: "x", Nama: "a_b"}, false)
	if strings.Contains(sql, "no_kwi") || len(args) != 4 || args[3] != `%a\_b%` {
		t.Errorf("without search: %s %v", sql, args)
	}
}

type fakeRepo struct {
	userBranches map[string][]branchauth.Branch
	lastList     ListParams
	lists        int
	lastReg      RegParams
	regs         int
	lastDokter   string
	lastObat     string
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.userBranches[userID], nil
}
func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.lastList, f.lists = p, f.lists+1
	return Page{Rows: []Row{{ID: 1, Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}
func (f *fakeRepo) JenisRawat(context.Context) ([]Option, error) {
	return []Option{{ID: 2, Name: "POLI UMUM"}}, nil
}
func (f *fakeRepo) StatusKunjungan(context.Context) ([]Option, error) {
	return []Option{{ID: 2, Name: "CLOSE"}}, nil
}
func (f *fakeRepo) JenisBayar(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "TUNAI"}}, nil
}
func (f *fakeRepo) Pendaftaran(_ context.Context, p RegParams) (RegPage, error) {
	f.lastReg, f.regs = p, f.regs+1
	return RegPage{Rows: []RegRow{}}, nil
}
func (f *fakeRepo) Dokter(_ context.Context, branch string) ([]Dokter, error) {
	f.lastDokter = branch
	if branch == "MED" {
		return []Dokter{{ID: 1, Nama: "NON DOKTER"}, {ID: 6, Nama: "dr. A", Jenis: "DOKTER UMUM"}}, nil
	}
	return []Dokter{}, nil
}
func (f *fakeRepo) Obat(_ context.Context, branch string) ([]Obat, error) {
	f.lastObat = branch
	return []Obat{{ID: 1, Nama: "ACESSORIES", Harga: 175000, Stok: 505}}, nil
}
func (f *fakeRepo) ProsesStatus(context.Context) ([]Option, error) {
	return []Option{{ID: 3, Name: "DAFTAR"}, {ID: 4, Name: "PROSES"}, {ID: 5, Name: "PAYMENT"}}, nil
}

type server struct {
	mux    *http.ServeMux
	tokens *auth.TokenManager
	repo   *fakeRepo
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := &fakeRepo{userBranches: map[string][]branchauth.Branch{
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
	if guarded != 5 {
		t.Fatalf("%d routes guarded, want 5", guarded)
	}
	return &server{mux: mux, tokens: tokens, repo: repo}
}

func (s *server) get(t *testing.T, userID, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if userID != "" {
		tok, _, _ := s.tokens.Generate(userID, "cs@medika.com")
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
	code, out := s.get(t, csID, "/api/pengobatan")
	lp := s.repo.lastList
	if code != http.StatusOK || out["from"] != "2026-11-19" || lp.From != "2026-11-19" || lp.To != "2026-11-19" ||
		strings.Join(lp.Branches, ",") != "KMJ,MED" || lp.Sort != "id" || !lp.Desc || lp.Limit != 10 {
		t.Fatalf("default list (today in Jakarta, newest first): %d %v %+v", code, out, lp)
	}
	s.get(t, csID, "/api/pengobatan?from=2026-11-01&to=2026-11-08&cabang=med&nama=+siti+&pelayanan=2&statusKunjungan=2&pembayaran=7&status=Cancel&q=2611&sort=total&dir=asc&size=20&page=3")
	lp = s.repo.lastList
	if lp.From != "2026-11-01" || lp.To != "2026-11-08" || strings.Join(lp.Branches, ",") != "MED" || lp.Nama != "siti" ||
		*lp.IDJenisRawat != 2 || *lp.IDStatusKunjung != 2 || *lp.IDJenisBayar != 7 || lp.Status != StatusCancel ||
		lp.Search != "2611" || lp.Sort != "total" || lp.Desc || lp.Offset != 40 {
		t.Errorf("filtered list: %+v", lp)
	}
	s.get(t, csID, "/api/pengobatan?sort=drop+table&size=999999")
	if lp = s.repo.lastList; lp.Sort != "id" || !lp.Desc || lp.Limit != MaxPageSize {
		t.Errorf("unknown sort / huge size: %+v", lp)
	}

	before := s.repo.lists
	for path, want := range map[string]int{
		"/api/pengobatan?cabang=CTM":                    http.StatusForbidden,
		"/api/pengobatan?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
		"/api/pengobatan?from=18-11-2026":               http.StatusBadRequest,
		"/api/pengobatan?status=paid":                   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lists != before {
		t.Errorf("rejected requests reached the repository")
	}
	if code, out := s.get(t, loneID, "/api/pengobatan"); code != http.StatusOK || out["total"] != 0.0 || s.repo.lists != before {
		t.Errorf("user without branches: %d %v", code, out)
	}
	if code, _ := s.get(t, "", "/api/pengobatan"); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestOptions(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["branches"].([]any)) != 2 ||
		len(out["pelayanan"].([]any)) != 1 || len(out["statusKunjungan"].([]any)) != 1 || len(out["pembayaran"].([]any)) != 1 ||
		len(out["proses"].([]any)) != 3 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := s.get(t, loneID, "/api/pengobatan/options"); code != http.StatusOK || out["branches"] == nil {
		t.Errorf("options without branches must return an empty list: %d %v", code, out)
	}
}

func TestRegWhere(t *testing.T) {
	sql, args := regWhere(RegParams{Branch: "CTM", From: "2026-11-18", To: "2026-11-19", Nama: "lola", IDJenisRawat: ptr(2), Search: "518586"}, true)
	for _, want := range []string{
		"d.cabang = $1", "d.tgl_masuk >= $2::date", "d.tgl_masuk < $3::date + 1", "d.id_statustr = ANY($4)",
		"NOT EXISTS (SELECT 1 FROM trxjual j WHERE j.id_kun = d.id)", "p.nama ILIKE $5", "d.id_jenis_rwt = $6", "d.id = $8",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("regWhere missing %q: %s", want, sql)
		}
	}
	if st, ok := args[3].([]int); !ok || len(st) != 3 {
		t.Errorf("default statuses = %v", args[3])
	}
	_, args = regWhere(RegParams{Branch: "CTM", From: "2026-11-18", To: "2026-11-19", IDStatus: ptr(4)}, true)
	if st := args[3].([]int); len(st) != 1 || st[0] != 4 {
		t.Errorf("status filter = %v", st)
	}
}

func TestPendaftaranLookup(t *testing.T) {
	s := newServer(t)
	// 23:30 UTC on 18 Nov is 19 Nov in Jakarta: the default range is 18-19 Nov.
	code, out := s.get(t, csID, "/api/pengobatan/pendaftaran?cabang=kmj")
	rp := s.repo.lastReg
	if code != http.StatusOK || rp.Branch != "KMJ" || rp.From != "2026-11-18" || rp.To != "2026-11-19" || rp.Sort != "id" || !rp.Desc || rp.Limit != 5 || rp.IDStatus != nil {
		t.Fatalf("default lookup: %d %v %+v", code, out, rp)
	}
	s.get(t, csID, "/api/pengobatan/pendaftaran?cabang=MED&from=2026-11-01&to=2026-11-08&nama=+lola+&pelayanan=2&status=4&q=48-01&sort=nama&dir=asc&size=10&page=2")
	rp = s.repo.lastReg
	if rp.Branch != "MED" || rp.From != "2026-11-01" || rp.Nama != "lola" || *rp.IDJenisRawat != 2 || *rp.IDStatus != 4 || rp.Search != "48-01" || rp.Sort != "nama" || rp.Desc || rp.Offset != 10 {
		t.Errorf("filtered lookup: %+v", rp)
	}
	before := s.repo.regs
	for path, want := range map[string]int{
		"/api/pengobatan/pendaftaran":                                          http.StatusBadRequest, // no branch
		"/api/pengobatan/pendaftaran?cabang=CTM":                               http.StatusForbidden,
		"/api/pengobatan/pendaftaran?cabang=KMJ&status=2":                      http.StatusBadRequest, // CLOSE is not billable
		"/api/pengobatan/pendaftaran?cabang=KMJ&from=2026-11-19&to=2026-11-01": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, _ := s.get(t, loneID, "/api/pengobatan/pendaftaran?cabang=KMJ"); code != http.StatusForbidden {
		t.Errorf("user without branches: %d", code)
	}
	if s.repo.regs != before {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestDokterLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/dokter?cabang=med")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastDokter != "MED" || len(rows) != 2 {
		t.Fatalf("doctors of MED: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/pengobatan/dokter?cabang=KMJ"); code != http.StatusOK || out["rows"] == nil {
		t.Errorf("a branch without doctors must return an empty list: %d %v", code, out)
	}
	s.repo.lastDokter = ""
	for path, want := range map[string]int{
		"/api/pengobatan/dokter":            http.StatusBadRequest,
		"/api/pengobatan/dokter?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastDokter != "" {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestObatLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/obat?cabang=kmj")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastObat != "KMJ" || len(rows) != 1 {
		t.Fatalf("drugs of KMJ: %d %v", code, out)
	}
	s.repo.lastObat = ""
	for path, want := range map[string]int{
		"/api/pengobatan/obat":            http.StatusBadRequest,
		"/api/pengobatan/obat?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastObat != "" {
		t.Errorf("rejected requests reached the repository")
	}
}
