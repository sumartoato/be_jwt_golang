package role

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/menu"
)

const (
	adminID = "11111111-1111-1111-1111-111111111111"
	nurseID = "22222222-2222-2222-2222-222222222222"
)

func ptr[T any](v T) *T { return &v }

var menuRows = []menu.Row{
	{ID: 1, Title: "Home", Path: ptr("/dashboard")},
	{ID: 2, Title: "Transaksi Klinik"},
	{ID: 3, ParentID: ptr(2), Title: "Tindakan", Path: ptr("/transaksi-klinik/tindakan")},
	{ID: 8, Title: "Management"},
	{ID: 9, ParentID: ptr(8), Title: "User", Path: ptr(UserPagePath)},
	{ID: 10, ParentID: ptr(8), Title: "Role", Path: ptr(RolePagePath)},
}

// fakeMenus answers AccessOf from the roles stored in fakeRoles, like the real
// database join does.
type fakeMenus struct{ roles *fakeRoles }

func (f fakeMenus) ListActive(context.Context) ([]menu.Row, error) { return menuRows, nil }

func (f fakeMenus) AccessOf(_ context.Context, userID string) (menu.Access, error) {
	rid := f.roles.userRole[userID]
	if rid == nil {
		return menu.Access{}, nil
	}
	ro := f.roles.roles[*rid]
	a := menu.Access{RoleID: rid, All: ro.AllAccess, MenuIDs: map[int]bool{}}
	for _, m := range ro.MenuIDs {
		a.MenuIDs[m] = true
	}
	return a, nil
}

type fakeRoles struct {
	roles        map[int]Role
	userRole     map[string]*int
	userBranches map[string][]string
	nextID       int
}

var knownBranches = []BranchOption{{"CTM", "KLINIK CITRA MEDIKA", true}, {"MED", "KLINIK MEDIKA", true}}

func (f *fakeRoles) ListBranchOptions(context.Context) ([]BranchOption, error) {
	return knownBranches, nil
}

func (f *fakeRoles) SetUserBranches(_ context.Context, userID string, codes []string) error {
	if _, ok := f.userRole[userID]; !ok {
		return ErrNotFound
	}
	for _, c := range codes {
		known := false
		for _, b := range knownBranches {
			known = known || b.Kode == c
		}
		if !known {
			return ErrUnknownBranch
		}
	}
	f.userBranches[userID] = codes
	return nil
}

func (f *fakeRoles) List(context.Context) ([]Role, error) {
	out := []Role{}
	for id := 1; id < f.nextID; id++ {
		if r, ok := f.roles[id]; ok {
			out = append(out, r)
		}
	}
	return out, nil
}

func (f *fakeRoles) Create(_ context.Context, in Input) (Role, error) {
	for _, r := range f.roles {
		if r.Name == in.Name {
			return Role{}, ErrNameTaken
		}
	}
	r := Role{ID: f.nextID, Name: in.Name, AllAccess: in.AllAccess, MenuIDs: in.MenuIDs}
	f.roles[r.ID] = r
	f.nextID++
	return r, nil
}

func (f *fakeRoles) Update(_ context.Context, id int, in Input) (Role, error) {
	if _, ok := f.roles[id]; !ok {
		return Role{}, ErrNotFound
	}
	r := Role{ID: id, Name: in.Name, AllAccess: in.AllAccess, MenuIDs: in.MenuIDs}
	f.roles[id] = r
	return r, nil
}

func (f *fakeRoles) Delete(_ context.Context, id int) error {
	if _, ok := f.roles[id]; !ok {
		return ErrNotFound
	}
	for _, rid := range f.userRole {
		if rid != nil && *rid == id {
			return ErrInUse
		}
	}
	delete(f.roles, id)
	return nil
}

func (f *fakeRoles) ListUsers(context.Context) ([]User, error) {
	return []User{{ID: adminID, Email: "admin@medika.com", RoleID: f.userRole[adminID], Branches: f.userBranches[adminID]}}, nil
}

func (f *fakeRoles) SetUserRole(_ context.Context, userID string, roleID *int) error {
	if _, ok := f.userRole[userID]; !ok {
		return ErrNotFound
	}
	if roleID != nil {
		if _, ok := f.roles[*roleID]; !ok {
			return ErrUnknownRole
		}
	}
	f.userRole[userID] = roleID
	return nil
}

type testServer struct {
	mux    *http.ServeMux
	tokens *auth.TokenManager
	roles  *fakeRoles
}

// newServer: role 1 Admin (all access) held by adminID, role 2 Perawat
// (Transaksi Klinik) held by nurseID.
func newServer() *testServer {
	roles := &fakeRoles{
		roles: map[int]Role{
			1: {ID: 1, Name: "Admin", AllAccess: true, MenuIDs: []int{}},
			2: {ID: 2, Name: "Perawat", MenuIDs: []int{2}},
		},
		userRole:     map[string]*int{adminID: ptr(1), nurseID: ptr(2)},
		userBranches: map[string][]string{},
		nextID:       3,
	}
	menus := fakeMenus{roles: roles}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	mux := http.NewServeMux()
	NewHandler(roles, menus).Routes(mux, tokens.Middleware, menu.NewHandler(menus).RequireMenu)
	return &testServer{mux: mux, tokens: tokens, roles: roles}
}

func (s *testServer) do(t *testing.T, userID, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if userID != "" {
		tok, _, err := s.tokens.Generate(userID, "x@example.com")
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestAccessFollowsMenu(t *testing.T) {
	s := newServer()
	if code, _ := s.do(t, "", "GET", "/api/roles", nil); code != http.StatusUnauthorized {
		t.Errorf("no token: got %d", code)
	}
	if code, _ := s.do(t, nurseID, "GET", "/api/roles", nil); code != http.StatusForbidden {
		t.Errorf("nurse: got %d, want 403", code)
	}
	if code, _ := s.do(t, nurseID, "GET", "/api/users", nil); code != http.StatusForbidden {
		t.Errorf("nurse users: got %d, want 403", code)
	}
	code, body := s.do(t, adminID, "GET", "/api/roles", nil)
	if code != http.StatusOK || len(body["roles"].([]any)) != 2 || len(body["menus"].([]any)) != 3 {
		t.Fatalf("admin: got %d %v", code, body)
	}
	// Granting the Role menu to Perawat opens the API immediately.
	s.roles.roles[2] = Role{ID: 2, Name: "Perawat", MenuIDs: []int{10}}
	if code, _ := s.do(t, nurseID, "GET", "/api/roles", nil); code != http.StatusOK {
		t.Errorf("nurse after grant: got %d", code)
	}
}

func TestCreateUpdateDelete(t *testing.T) {
	s := newServer()
	if code, body := s.do(t, adminID, "POST", "/api/roles", map[string]any{"name": "  "}); code != http.StatusBadRequest || body["errors"] == nil {
		t.Errorf("empty name: got %d %v", code, body)
	}
	code, body := s.do(t, adminID, "POST", "/api/roles", map[string]any{"name": " Kasir ", "menuIds": []int{2}})
	if code != http.StatusCreated || body["name"] != "Kasir" {
		t.Fatalf("create: got %d %v", code, body)
	}
	if code, _ := s.do(t, adminID, "POST", "/api/roles", map[string]any{"name": "Kasir"}); code != http.StatusConflict {
		t.Errorf("duplicate: got %d", code)
	}
	if code, _ := s.do(t, adminID, "PUT", "/api/roles/3", map[string]any{"name": "Kasir", "menuIds": []int{1}}); code != http.StatusOK {
		t.Errorf("update: got %d", code)
	}
	if code, _ := s.do(t, adminID, "PUT", "/api/roles/99", map[string]any{"name": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: got %d", code)
	}
	if code, _ := s.do(t, adminID, "DELETE", "/api/roles/2", nil); code != http.StatusConflict {
		t.Errorf("delete in use: got %d", code)
	}
	if code, _ := s.do(t, adminID, "DELETE", "/api/roles/3", nil); code != http.StatusNoContent {
		t.Errorf("delete: got %d", code)
	}
}

func TestCannotLockYourselfOut(t *testing.T) {
	s := newServer()
	code, _ := s.do(t, adminID, "PUT", "/api/roles/1", map[string]any{"name": "Admin", "allAccess": false, "menuIds": []int{2}})
	if code != http.StatusBadRequest {
		t.Errorf("dropping own access: got %d, want 400", code)
	}
	// Keeping the Management group (which contains Role) is fine.
	if code, _ := s.do(t, adminID, "PUT", "/api/roles/1", map[string]any{"name": "Admin", "menuIds": []int{8}}); code != http.StatusOK {
		t.Errorf("keeping Management: got %d", code)
	}
	if code, body := s.do(t, adminID, "PUT", "/api/users/"+adminID+"/role", map[string]any{"roleId": nil}); code != http.StatusBadRequest {
		t.Errorf("own role change: got %d %v", code, body)
	}
}

func TestSetUserRole(t *testing.T) {
	s := newServer()
	if code, _ := s.do(t, adminID, "PUT", "/api/users/"+nurseID+"/role", map[string]any{"roleId": 1}); code != http.StatusOK {
		t.Errorf("assign: got %d", code)
	}
	if got := s.roles.userRole[nurseID]; got == nil || *got != 1 {
		t.Errorf("role not saved: %v", got)
	}
	if code, _ := s.do(t, adminID, "PUT", "/api/users/"+nurseID+"/role", map[string]any{"roleId": 42}); code != http.StatusBadRequest {
		t.Errorf("unknown role: got %d", code)
	}
	if code, _ := s.do(t, adminID, "PUT", "/api/users/not-a-uuid/role", map[string]any{"roleId": 1}); code != http.StatusNotFound {
		t.Errorf("bad id: got %d", code)
	}
}

func TestSetUserBranches(t *testing.T) {
	s := newServer()
	path := "/api/users/" + nurseID + "/branches"

	// Codes are trimmed, uppercased, de-duplicated and sorted.
	code, body := s.do(t, adminID, "PUT", path, map[string]any{"branches": []string{" med", "CTM", "MED", ""}})
	if code != http.StatusOK {
		t.Fatalf("set: got %d %v", code, body)
	}
	if got := s.roles.userBranches[nurseID]; len(got) != 2 || got[0] != "CTM" || got[1] != "MED" {
		t.Fatalf("saved %v", got)
	}

	if code, body := s.do(t, adminID, "PUT", path, map[string]any{"branches": []string{"XYZ"}}); code != http.StatusBadRequest || body["errors"] == nil {
		t.Errorf("unknown branch: got %d %v", code, body)
	}
	if code, _ := s.do(t, adminID, "PUT", path, map[string]any{"branches": []string{"ABCDEFGHIJK"}}); code != http.StatusBadRequest {
		t.Errorf("too long: got %d", code)
	}
	if code, _ := s.do(t, adminID, "PUT", path, map[string]any{"branches": []string{}}); code != http.StatusOK || len(s.roles.userBranches[nurseID]) != 0 {
		t.Errorf("clear: got %d %v", code, s.roles.userBranches[nurseID])
	}
	if code, _ := s.do(t, adminID, "PUT", "/api/users/33333333-3333-3333-3333-333333333333/branches", map[string]any{"branches": []string{}}); code != http.StatusNotFound {
		t.Errorf("unknown user: got %d", code)
	}
	// Your own branches may be changed (they do not grant page access).
	if code, _ := s.do(t, adminID, "PUT", "/api/users/"+adminID+"/branches", map[string]any{"branches": []string{"CTM"}}); code != http.StatusOK {
		t.Errorf("own branches: got %d", code)
	}
	// Needs the User menu.
	if code, _ := s.do(t, nurseID, "PUT", path, map[string]any{"branches": []string{}}); code != http.StatusForbidden {
		t.Errorf("nurse: got %d, want 403", code)
	}

	code, body = s.do(t, adminID, "GET", "/api/users", nil)
	if code != http.StatusOK || len(body["branches"].([]any)) != 2 {
		t.Fatalf("list: got %d %v", code, body)
	}
	if u := body["users"].([]any)[0].(map[string]any); len(u["branches"].([]any)) != 1 {
		t.Errorf("user branches missing: %v", u)
	}
}

func TestUserMenus(t *testing.T) {
	s := newServer()
	titles := func(v any) []string {
		out := []string{}
		for _, m := range v.([]any) {
			out = append(out, m.(map[string]any)["title"].(string))
		}
		return out
	}

	// Perawat is granted the Transaksi Klinik group: the group and its child.
	code, body := s.do(t, adminID, "GET", "/api/users/"+nurseID+"/menus", nil)
	if code != http.StatusOK || body["allAccess"] != false {
		t.Fatalf("nurse: got %d %v", code, body)
	}
	if got := titles(body["menus"]); len(got) != 1 || got[0] != "Transaksi Klinik" {
		t.Fatalf("nurse menus = %v", got)
	}
	if kids := body["menus"].([]any)[0].(map[string]any)["children"].([]any); len(kids) != 1 {
		t.Errorf("nurse children = %v", kids)
	}

	// Admin has all access: every root.
	code, body = s.do(t, adminID, "GET", "/api/users/"+adminID+"/menus", nil)
	if got := titles(body["menus"]); code != http.StatusOK || body["allAccess"] != true || len(got) != 3 {
		t.Fatalf("admin: got %d %v", code, got)
	}

	// No role: empty tree, role null.
	s.roles.userRole[nurseID] = nil
	code, body = s.do(t, adminID, "GET", "/api/users/"+nurseID+"/menus", nil)
	if code != http.StatusOK || body["role"] != nil || len(body["menus"].([]any)) != 0 {
		t.Errorf("no role: got %d %v", code, body)
	}

	if code, _ := s.do(t, adminID, "GET", "/api/users/not-a-uuid/menus", nil); code != http.StatusNotFound {
		t.Errorf("bad id: got %d", code)
	}
	if code, _ := s.do(t, nurseID, "GET", "/api/users/"+adminID+"/menus", nil); code != http.StatusForbidden {
		t.Errorf("without the User menu: got %d, want 403", code)
	}
}
