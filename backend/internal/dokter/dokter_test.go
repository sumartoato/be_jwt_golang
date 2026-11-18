package dokter

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

const (
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to MED and MGM
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func ptr[T any](v T) *T { return &v }

var today = time.Date(2026, 11, 18, 0, 0, 0, 0, time.UTC)

func TestNormalize(t *testing.T) {
	in := Input{Nama: " DR HAFIZ ", TglLahir: ptr(" "), TglMasuk: ptr("2020-01-02")}
	if errs := in.Normalize(today); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if in.Nama != "DR HAFIZ" || in.TglLahir != nil || *in.TglMasuk != "2020-01-02" {
		t.Fatalf("not normalized: %+v", in)
	}
	for field, in := range map[string]Input{
		"nama":     {Nama: ""},
		"tglLahir": {Nama: "X", TglLahir: ptr("2030-01-01")},
		"tglMasuk": {Nama: "X", TglMasuk: ptr("01/02/2020")},
		"hp":       {Nama: "X", HP: strings.Repeat("0", 21)},
	} {
		if errs := in.Normalize(today); errs[field] == "" {
			t.Errorf("want error on %s, got %v", field, errs)
		}
	}
	if errs := (&Input{Nama: strings.Repeat("n", 31)}).Normalize(today); errs["nama"] == "" {
		t.Error("nama longer than 30 must fail")
	}
}

type fakeRepo struct {
	userBranches map[string][]branchauth.Branch
	dokter       map[int]Dokter
	tindakan     map[int]map[int]bool // dokter -> tindakan ids
}

var allBranches = []branchauth.Branch{{Kode: "CTM", Nama: "KLINIK CITRA MEDIKA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}, {Kode: "MGM", Nama: "KLINIK MGM 24"}}
var allTindakan = []TindakanItem{{1, "KONSULTASI", "Konsultasi"}, {2, "TENSI", "Tindakan Dokter Umum"}, {3, "EKG", "Tindakan Spesialis"}}

func newFake() *fakeRepo {
	return &fakeRepo{
		userBranches: map[string][]branchauth.Branch{csID: {allBranches[1], allBranches[2]}},
		dokter:       map[int]Dokter{7: {ID: 7, Nama: "PUTRI WULANDARI DR", IsActive: true, CabangPraktek: []string{"CTM", "MED"}}},
		tindakan:     map[int]map[int]bool{7: {1: true}},
	}
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.userBranches[userID], nil
}
func (f *fakeRepo) AllBranches(context.Context) ([]branchauth.Branch, error) { return allBranches, nil }
func (f *fakeRepo) Options(context.Context) (Options, error) {
	return Options{Jenis: []Option{{2, "DOKTER UMUM"}}}, nil
}
func (f *fakeRepo) List(context.Context) ([]Dokter, error) {
	out := []Dokter{}
	for _, d := range f.dokter {
		out = append(out, d)
	}
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (Dokter, error) {
	d, ok := f.dokter[id]
	if !ok {
		return Dokter{}, ErrNotFound
	}
	return d, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, _ string) (int, error) {
	if in.IDJenis != nil && *in.IDJenis == 99 {
		return 0, InvalidRefError{Field: "idJenis"}
	}
	id := 100 + len(f.dokter)
	f.dokter[id] = Dokter{ID: id, Nama: in.Nama, IsActive: in.IsActive, CabangPraktek: []string{}}
	return id, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	d, ok := f.dokter[id]
	if !ok {
		return ErrNotFound
	}
	d.Nama, d.IsActive = in.Nama, in.IsActive
	f.dokter[id] = d
	return nil
}
func (f *fakeRepo) Tindakan(_ context.Context, id int) ([]TindakanItem, []TindakanItem, error) {
	a, b := []TindakanItem{}, []TindakanItem{}
	for _, t := range allTindakan {
		if f.tindakan[id][t.ID] {
			a = append(a, t)
		} else {
			b = append(b, t)
		}
	}
	return a, b, nil
}
func (f *fakeRepo) ChangeTindakan(_ context.Context, id int, add, remove []int) error {
	if f.tindakan[id] == nil {
		f.tindakan[id] = map[int]bool{}
	}
	for _, t := range remove {
		delete(f.tindakan[id], t)
	}
	for _, t := range add {
		if t > 3 {
			return InvalidRefError{Field: "tindakan"}
		}
		f.tindakan[id][t] = true
	}
	return nil
}
func (f *fakeRepo) ChangeCabang(_ context.Context, id int, add, remove []string) error {
	d := f.dokter[id]
	set := map[string]bool{}
	for _, c := range d.CabangPraktek {
		set[c] = true
	}
	for _, c := range remove {
		delete(set, c)
	}
	for _, c := range add {
		set[c] = true
	}
	d.CabangPraktek = []string{}
	for c := range set {
		d.CabangPraktek = append(d.CabangPraktek, c)
	}
	sort.Strings(d.CabangPraktek)
	f.dokter[id] = d
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
	h.now = func() time.Time { return today }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 8 {
		t.Fatalf("%d routes guarded, want 8", guarded)
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

func TestDokterCRUD(t *testing.T) {
	s := newServer(t)
	if code, out := s.do(t, csID, "GET", "/api/dokter", nil); code != http.StatusOK || len(out["dokter"].([]any)) != 1 || len(out["branches"].([]any)) != 3 {
		t.Fatalf("list: %d %v", code, out)
	}
	code, out := s.do(t, csID, "POST", "/api/dokter", map[string]any{"nama": " DR BARU ", "isActive": true})
	if code != http.StatusCreated || out["nama"] != "DR BARU" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := s.do(t, csID, "POST", "/api/dokter", map[string]any{"nama": "X", "idJenis": 99}); code != http.StatusBadRequest || out["errors"].(map[string]any)["idJenis"] == nil {
		t.Errorf("unknown jenis: %d %v", code, out)
	}
	if code, out := s.do(t, csID, "PUT", "/api/dokter/7", map[string]any{"nama": "PUTRI W DR", "isActive": false}); code != http.StatusOK || out["isActive"] != false {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "PUT", "/api/dokter/999", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}

func TestTindakan(t *testing.T) {
	s := newServer(t)
	_, out := s.do(t, csID, "GET", "/api/dokter/7/tindakan", nil)
	if len(out["assigned"].([]any)) != 1 || len(out["available"].([]any)) != 2 {
		t.Fatalf("lists: %v", out)
	}
	code, out := s.do(t, csID, "PATCH", "/api/dokter/7/tindakan", map[string]any{"add": []int{2, 3}, "remove": []int{1}})
	if code != http.StatusOK || len(out["assigned"].([]any)) != 2 || len(out["available"].([]any)) != 1 {
		t.Fatalf("change: %d %v", code, out)
	}
	if code, out := s.do(t, csID, "PATCH", "/api/dokter/7/tindakan", map[string]any{"add": []int{42}}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown tindakan: %d %v", code, out)
	}
	if code, _ := s.do(t, csID, "GET", "/api/dokter/999/tindakan", nil); code != http.StatusNotFound {
		t.Errorf("missing dokter: %d", code)
	}
}

func TestCabangPraktekFollowsUserBranches(t *testing.T) {
	s := newServer(t)
	_, out := s.do(t, csID, "GET", "/api/dokter/7/cabang", nil)
	if len(out["assigned"].([]any)) != 2 || len(out["available"].([]any)) != 1 || len(out["editable"].([]any)) != 2 {
		t.Fatalf("lists: %v", out)
	}
	// cs may add MGM and remove MED (their branches)...
	code, out := s.do(t, csID, "PATCH", "/api/dokter/7/cabang", map[string]any{"add": []string{"mgm"}, "remove": []string{"MED"}})
	if code != http.StatusOK {
		t.Fatalf("change: %d %v", code, out)
	}
	if got := strings.Join(s.repo.dokter[7].CabangPraktek, ","); got != "CTM,MGM" {
		t.Errorf("cabang praktek = %s, want CTM,MGM", got)
	}
	// ...but not touch CTM, which is not one of theirs.
	if code, _ := s.do(t, csID, "PATCH", "/api/dokter/7/cabang", map[string]any{"remove": []string{"CTM"}}); code != http.StatusForbidden {
		t.Errorf("remove other branch: %d, want 403", code)
	}
	if code, _ := s.do(t, loneID, "PATCH", "/api/dokter/7/cabang", map[string]any{"add": []string{"MED"}}); code != http.StatusForbidden {
		t.Errorf("unassigned user: %d, want 403", code)
	}
}
