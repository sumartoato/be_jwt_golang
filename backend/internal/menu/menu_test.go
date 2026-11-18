package menu

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func ptr[T any](v T) *T { return &v }

func titles(items []*Item) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Title
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestBuildTree(t *testing.T) {
	rows := []Row{
		// A child listed before its parent must still be nested.
		{ID: 3, ParentID: ptr(2), Title: "MCU", Path: ptr("/master-data/mcu")},
		{ID: 1, Title: "Home", Path: ptr("/dashboard")},
		{ID: 2, Title: "Master Data"},
		{ID: 4, ParentID: ptr(2), Title: "Pasien", Path: ptr("/master-data/pasien")},
		// Parent 99 is inactive (not in rows): drop it.
		{ID: 5, ParentID: ptr(99), Title: "Orphan"},
		// Grandchild under the orphan: also hidden.
		{ID: 6, ParentID: ptr(5), Title: "Grandchild"},
	}
	tree := BuildTree(rows)

	if got := titles(tree); !equal(got, []string{"Home", "Master Data"}) {
		t.Fatalf("roots = %v", got)
	}
	if got := titles(tree[1].Children); !equal(got, []string{"MCU", "Pasien"}) {
		t.Fatalf("children = %v", got)
	}
	if tree[0].Children == nil {
		t.Fatal("leaf children must be an empty slice so JSON has [] not null")
	}
}

type fakeRepo struct {
	rows   []Row
	access *Access // nil means all access
	err    error
}

func (f fakeRepo) ListActive(context.Context) ([]Row, error) { return f.rows, f.err }

func (f fakeRepo) AccessOf(context.Context, string) (Access, error) {
	if f.access == nil {
		return Access{All: true}, nil
	}
	return *f.access, nil
}

func serve(repo Repository, authorized bool) *httptest.ResponseRecorder {
	requireAuth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !authorized {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	mux := http.NewServeMux()
	NewHandler(repo).Routes(mux, requireAuth)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/menus", nil))
	return rec
}

func TestListRequiresAuth(t *testing.T) {
	if rec := serve(fakeRepo{}, false); rec.Code != http.StatusUnauthorized {
		t.Fatalf("got %d, want 401", rec.Code)
	}
}

func TestListReturnsTree(t *testing.T) {
	rec := serve(fakeRepo{rows: []Row{
		{ID: 1, Title: "Home", Icon: "home", Path: ptr("/dashboard")},
		{ID: 2, Title: "Master Data", Icon: "database"},
		{ID: 3, ParentID: ptr(2), Title: "MCU", Icon: "circle", Path: ptr("/master-data/mcu")},
	}}, true)
	if rec.Code != http.StatusOK {
		t.Fatalf("got %d", rec.Code)
	}
	var body struct {
		Menus []struct {
			Title    string  `json:"title"`
			Path     *string `json:"path"`
			Children []struct {
				Title string `json:"title"`
			} `json:"children"`
		} `json:"menus"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Menus) != 2 || body.Menus[1].Path != nil || len(body.Menus[1].Children) != 1 || body.Menus[1].Children[0].Title != "MCU" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}

func TestListEmptyIsArray(t *testing.T) {
	rec := serve(fakeRepo{}, true)
	if got := rec.Body.String(); got != "{\"menus\":[],\"role\":null}\n" {
		t.Fatalf("got %q", got)
	}
}

func TestListRepoError(t *testing.T) {
	if rec := serve(fakeRepo{err: errors.New("db down")}, true); rec.Code != http.StatusInternalServerError {
		t.Fatalf("got %d, want 500", rec.Code)
	}
}

// clinic is a small menu: Home, a group with two children, a group with a
// nested group, and Management > Role.
var clinic = []Row{
	{ID: 1, Title: "Home", Path: ptr("/dashboard")},
	{ID: 2, Title: "Front Office"},
	{ID: 3, ParentID: ptr(2), Title: "Pendaftaran", Path: ptr("/fo/pendaftaran")},
	{ID: 4, ParentID: ptr(2), Title: "Riwayat", Path: ptr("/fo/riwayat")},
	{ID: 5, Title: "Report"},
	{ID: 6, ParentID: ptr(5), Title: "Harian"},
	{ID: 7, ParentID: ptr(6), Title: "Kunjungan", Path: ptr("/report/kunjungan")},
	{ID: 8, Title: "Management"},
	{ID: 9, ParentID: ptr(8), Title: "Role", Path: ptr("/management/role")},
}

func ids(rows []Row) []int {
	out := []int{}
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

func grant(menuIDs ...int) Access {
	a := Access{MenuIDs: map[int]bool{}}
	for _, id := range menuIDs {
		a.MenuIDs[id] = true
	}
	return a
}

func TestFilter(t *testing.T) {
	cases := []struct {
		name   string
		access Access
		want   []int
	}{
		{"no role sees nothing", Access{}, []int{}},
		{"all access sees everything", Access{All: true}, []int{1, 2, 3, 4, 5, 6, 7, 8, 9}},
		{"a group brings its children", grant(1, 2), []int{1, 2, 3, 4}},
		{"a group brings nested descendants", grant(5), []int{5, 6, 7}},
		{"a leaf brings its ancestors only", grant(7), []int{5, 6, 7}},
		{"one child of a group", grant(4), []int{2, 4}},
		{"unknown ids are ignored", grant(99), []int{}},
	}
	for _, c := range cases {
		got := ids(Filter(clinic, c.access))
		if len(got) != len(c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: got %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestFilterSurvivesParentCycle(t *testing.T) {
	rows := []Row{{ID: 1, ParentID: ptr(2), Title: "A"}, {ID: 2, ParentID: ptr(1), Title: "B"}}
	if got := Filter(rows, grant(1)); len(got) != 2 {
		t.Fatalf("got %v", got)
	}
}

func TestCanOpen(t *testing.T) {
	if !CanOpen(clinic, "/management/role/") {
		t.Error("trailing slash should match")
	}
	if CanOpen(Filter(clinic, grant(2)), "/management/role") {
		t.Error("role without Management must not open /management/role")
	}
}

func TestRequireMenu(t *testing.T) {
	for _, c := range []struct {
		name   string
		repo   fakeRepo
		status int
	}{
		{"granted", fakeRepo{rows: clinic, access: ptr(grant(8))}, http.StatusNoContent},
		{"not granted", fakeRepo{rows: clinic, access: ptr(grant(2))}, http.StatusForbidden},
		{"no role", fakeRepo{rows: clinic, access: &Access{}}, http.StatusForbidden},
		{"repo error", fakeRepo{err: errors.New("db down")}, http.StatusInternalServerError},
	} {
		ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
		rec := httptest.NewRecorder()
		NewHandler(c.repo).RequireMenu("/management/role")(ok).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
		if rec.Code != c.status {
			t.Errorf("%s: got %d, want %d", c.name, rec.Code, c.status)
		}
	}
}

func TestListIsFilteredByRole(t *testing.T) {
	name := "Perawat"
	acc := grant(2)
	acc.RoleName = &name
	rec := serve(fakeRepo{rows: clinic, access: &acc}, true)
	var body struct {
		Menus []Item  `json:"menus"`
		Role  *string `json:"role"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Menus) != 1 || body.Menus[0].Title != "Front Office" || body.Role == nil || *body.Role != "Perawat" {
		t.Fatalf("unexpected body: %s", rec.Body.String())
	}
}
