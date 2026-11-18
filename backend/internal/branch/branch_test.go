package branch

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNormalize(t *testing.T) {
	in := Input{Kode: "  ctm ", Nama: " KLINIK CITRA MEDIKA ", KodeKwt: " ctm", OrganizationID: " 100468100 "}
	if errs := in.Normalize(true); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if in.Kode != "CTM" || in.KodeKwt != "CTM" || in.Nama != "KLINIK CITRA MEDIKA" || in.OrganizationID != "100468100" {
		t.Fatalf("not normalized: %+v", in)
	}

	cases := []struct {
		in    Input
		field string
	}{
		{Input{Nama: "X"}, "kode"},
		{Input{Kode: "C T", Nama: "X"}, "kode"},
		{Input{Kode: strings.Repeat("A", 21), Nama: "X"}, "kode"},
		{Input{Kode: "CTM", Nama: "   "}, "nama"},
		// Limits follow the varchar sizes of refbranch.
		{Input{Kode: "CTM", Nama: strings.Repeat("n", 51)}, "nama"},
		{Input{Kode: "CTM", Nama: "X", KodeKwt: strings.Repeat("K", 11)}, "kodeKwt"},
		{Input{Kode: "CTM", Nama: "X", Alamat: strings.Repeat("a", 201)}, "alamat"},
		{Input{Kode: "CTM", Nama: "X", SecretKey: strings.Repeat("s", 101)}, "secretKey"},
		{Input{Kode: "CTM", Nama: "X", DBHbee: strings.Repeat("d", 51)}, "dbHbee"},
	}
	for _, c := range cases {
		if errs := c.in.Normalize(true); errs[c.field] == "" {
			t.Errorf("%+v: want error on %s, got %v", c.in, c.field, errs)
		}
	}

	// On update the kode is not validated (it comes from the URL and cannot change).
	upd := Input{Nama: "X"}
	if errs := upd.Normalize(false); len(errs) != 0 {
		t.Errorf("update: unexpected errors %v", errs)
	}
}

// fakeRepo keeps branches in memory and mimics the database's secret handling.
type fakeRepo struct {
	order    []string
	branches map[string]Branch
	secrets  map[string]string
}

func (f *fakeRepo) List(context.Context) ([]Branch, error) {
	out := []Branch{}
	for _, k := range f.order {
		out = append(out, f.branches[k])
	}
	return out, nil
}

func (f *fakeRepo) save(kode string, in Input) Branch {
	if in.SecretKey != "" {
		f.secrets[kode] = in.SecretKey
	}
	s := f.secrets[kode]
	b := Branch{Kode: kode, Nama: in.Nama, KodeKwt: in.KodeKwt, ClientKey: in.ClientKey, SecretKeySet: s != "",
		IsActive: in.IsActive, IsInternal: in.IsInternal}
	if len(s) >= 12 {
		b.SecretKeyHint = s[len(s)-4:]
	}
	f.branches[kode] = b
	return b
}

func (f *fakeRepo) Create(_ context.Context, in Input) (Branch, error) {
	if _, ok := f.branches[in.Kode]; ok {
		return Branch{}, ErrCodeTaken
	}
	f.order = append(f.order, in.Kode)
	return f.save(in.Kode, in), nil
}

func (f *fakeRepo) Update(_ context.Context, kode string, in Input) (Branch, error) {
	if _, ok := f.branches[kode]; !ok {
		return Branch{}, ErrNotFound
	}
	return f.save(kode, in), nil
}

func newServer(t *testing.T) *http.ServeMux {
	t.Helper()
	pass := func(next http.Handler) http.Handler { return next }
	var guarded []string
	requireMenu := func(path string) func(http.Handler) http.Handler {
		guarded = append(guarded, path)
		return pass
	}
	mux := http.NewServeMux()
	NewHandler(&fakeRepo{branches: map[string]Branch{}, secrets: map[string]string{}}).Routes(mux, pass, requireMenu)
	if len(guarded) != 3 {
		t.Fatalf("%d routes guarded, want 3", len(guarded))
	}
	for _, p := range guarded {
		if p != PagePath {
			t.Fatalf("route guarded by %q, want %q", p, PagePath)
		}
	}
	return mux
}

func call(t *testing.T, mux *http.ServeMux, method, path string, body any) (int, map[string]any, string) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, path, &buf))
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out, rec.Body.String()
}

func TestCreateUpdateList(t *testing.T) {
	mux := newServer(t)
	secret := "fake-secret-for-tests-ZAs1"

	code, body, raw := call(t, mux, "POST", "/api/branches", map[string]any{
		"kode": "ctm", "nama": "KLINIK CITRA MEDIKA", "kodeKwt": "ctm", "clientKey": "client-1", "secretKey": secret,
		"isActive": true, "isInternal": true,
	})
	if code != http.StatusCreated || body["kode"] != "CTM" || body["kodeKwt"] != "CTM" || body["isInternal"] != true ||
		body["secretKeySet"] != true || body["secretKeyHint"] != "ZAs1" {
		t.Fatalf("create: %d %s", code, raw)
	}
	if strings.Contains(raw, secret) {
		t.Fatal("the secret key must never be returned")
	}

	if code, body, _ := call(t, mux, "POST", "/api/branches", map[string]any{"kode": "CTM", "nama": "Dup"}); code != http.StatusConflict || body["errors"] == nil {
		t.Errorf("duplicate kode: %d %v", code, body)
	}
	if code, body, _ := call(t, mux, "POST", "/api/branches", map[string]any{"nama": "No kode"}); code != http.StatusBadRequest || body["errors"] == nil {
		t.Errorf("missing kode: %d %v", code, body)
	}

	// The kode comes from the URL (lowercase is accepted); a kode in the body is ignored,
	// and an empty secretKey keeps the stored key.
	code, body, raw = call(t, mux, "PUT", "/api/branches/ctm", map[string]any{"kode": "NEW", "nama": "Renamed", "isActive": false})
	if code != http.StatusOK || body["kode"] != "CTM" || body["nama"] != "Renamed" || body["isActive"] != false || body["secretKeyHint"] != "ZAs1" {
		t.Fatalf("update: %d %s", code, raw)
	}
	if code, _, _ := call(t, mux, "PUT", "/api/branches/XXX", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}

	code, body, _ = call(t, mux, "GET", "/api/branches", nil)
	if code != http.StatusOK || len(body["branches"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, body)
	}
}
