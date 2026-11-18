package orglokasi

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

func TestNormalize(t *testing.T) {
	in := Input{Organisasi: " POLI UMUM ", Lokasi: "Ruang POLI UMUM", OrganisasiID: " B85FF1FD-84CE-46DC-A216-863DF8F9993F ", LokasiID: ""}
	if errs := in.Normalize(); errs != nil {
		t.Fatalf("valid input: %v", errs)
	}
	if in.Organisasi != "POLI UMUM" || in.OrganisasiID != "b85ff1fd-84ce-46dc-a216-863df8f9993f" {
		t.Fatalf("not normalized: %+v", in)
	}
	errs := (&Input{Organisasi: "", Lokasi: strings.Repeat("x", 51), OrganisasiID: "abc", LokasiID: "123"}).Normalize()
	for _, f := range []string{"organisasi", "lokasi", "organisasiId", "lokasiId"} {
		if errs[f] == "" {
			t.Errorf("%s must fail: %v", f, errs)
		}
	}
}

type fakeRepo struct {
	branches []branchauth.Branch
	rows     map[int]Row
}

func (f *fakeRepo) UserBranches(context.Context, string) ([]branchauth.Branch, error) {
	return f.branches, nil
}
func (f *fakeRepo) List(_ context.Context, codes []string) ([]Row, error) {
	out := []Row{}
	for _, r := range f.rows {
		for _, c := range codes {
			if r.Cabang == c {
				out = append(out, r)
			}
		}
	}
	return out, nil
}
func (f *fakeRepo) Get(_ context.Context, id int) (Row, error) {
	r, ok := f.rows[id]
	if !ok {
		return Row{}, ErrNotFound
	}
	return r, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, user string) (Row, error) {
	r, ok := f.rows[id]
	if !ok {
		return Row{}, ErrNotFound
	}
	r.Organisasi, r.Lokasi, r.OrganisasiID, r.LokasiID, r.UpdatedBy = in.Organisasi, in.Lokasi, in.OrganisasiID, in.LokasiID, user
	f.rows[id] = r
	return r, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{
		branches: []branchauth.Branch{{Kode: "CTM", Nama: "KLINIK CITRA MEDIKA"}},
		rows: map[int]Row{
			1: {ID: 1, Cabang: "CTM", Poli: "POLI UMUM", Organisasi: "POLI UMUM", Lokasi: "Ruang POLI UMUM"},
			8: {ID: 8, Cabang: "KJI", Poli: "POLI UMUM", Organisasi: "POLI UMUM", Lokasi: "Ruang POLI UMUM"},
		},
	}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q", path)
		}
		guarded++
		return func(next http.Handler) http.Handler { return next }
	}
	mux := http.NewServeMux()
	NewHandler(repo).Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 2 {
		t.Fatalf("%d routes guarded, want 2", guarded)
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

	if code, out := do("GET", "/api/organisasi-lokasi", nil); code != http.StatusOK || len(out["rows"].([]any)) != 1 || len(out["branches"].([]any)) != 1 {
		t.Fatalf("list must only hold the caller's branch: %d %v", code, out)
	}
	valid := map[string]any{"organisasi": "POLI UMUM", "lokasi": "Ruang POLI UMUM", "organisasiId": "B85FF1FD-84CE-46DC-A216-863DF8F9993F", "lokasiId": ""}
	if code, out := do("PUT", "/api/organisasi-lokasi/1", valid); code != http.StatusOK ||
		out["organisasiId"] != "b85ff1fd-84ce-46dc-a216-863df8f9993f" || out["updatedBy"] != "admin@medika.com" {
		t.Fatalf("update: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/organisasi-lokasi/1", map[string]any{"organisasi": "", "lokasi": "x", "organisasiId": "nope"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("invalid: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/organisasi-lokasi/8", valid); code != http.StatusNotFound {
		t.Errorf("other branch must look missing: %d", code)
	}
	if code, _ := do("PUT", "/api/organisasi-lokasi/999", valid); code != http.StatusNotFound {
		t.Errorf("missing: %d", code)
	}

	repo.branches = nil
	if code, out := do("GET", "/api/organisasi-lokasi", nil); code != http.StatusOK || len(out["rows"].([]any)) != 0 {
		t.Errorf("no branches: %d %v", code, out)
	}
}
