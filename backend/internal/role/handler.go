package role

import (
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/google/uuid"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/menu"
)

// Access to these APIs follows the menu: whoever's role can open the page
// can use its API. Both menu items are seeded by 012_management_menu.sql.
const (
	RolePagePath = "/management/role"
	UserPagePath = "/management/user"
)

type Handler struct {
	roles Repository
	menus menu.Repository
}

func NewHandler(roles Repository, menus menu.Repository) *Handler {
	return &Handler{roles: roles, menus: menus}
}

// Routes registers the role and user-role endpoints. requireMenu(path)
// rejects callers whose role cannot open the menu item with that path.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(path string, f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(path)(f)) }
	mux.Handle("GET /api/roles", guard(RolePagePath, h.ListRoles))
	mux.Handle("POST /api/roles", guard(RolePagePath, h.CreateRole))
	mux.Handle("PUT /api/roles/{id}", guard(RolePagePath, h.UpdateRole))
	mux.Handle("DELETE /api/roles/{id}", guard(RolePagePath, h.DeleteRole))
	mux.Handle("GET /api/users", guard(UserPagePath, h.ListUsers))
	mux.Handle("PUT /api/users/{id}/role", guard(UserPagePath, h.SetUserRole))
	mux.Handle("PUT /api/users/{id}/branches", guard(UserPagePath, h.SetUserBranches))
	mux.Handle("GET /api/users/{id}/menus", guard(UserPagePath, h.UserMenus))
}

// ListRoles returns every role plus the full menu tree for the role editor.
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.roles.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	rows, err := h.menus.ListActive(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"roles": roles, "menus": menu.BuildTree(rows)})
}

func (h *Handler) CreateRole(w http.ResponseWriter, r *http.Request) {
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	role, err := h.roles.Create(r.Context(), in)
	if err != nil {
		h.writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, role)
}

func (h *Handler) UpdateRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decodeInput(w, r)
	if !ok {
		return
	}
	// Do not let admins lock themselves out of this page by editing their own role.
	caller, err := h.callerAccess(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if caller.RoleID != nil && *caller.RoleID == id {
		rows, err := h.menus.ListActive(r.Context())
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		after := menu.Access{All: in.AllAccess, MenuIDs: map[int]bool{}}
		for _, m := range in.MenuIDs {
			after.MenuIDs[m] = true
		}
		if !menu.CanOpen(menu.Filter(rows, after), RolePagePath) {
			httpx.WriteError(w, http.StatusBadRequest, "this is your own role: keep Management > Role in it, or you would lose access to this page")
			return
		}
	}
	role, err := h.roles.Update(r.Context(), id, in)
	if err != nil {
		h.writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, role)
}

func (h *Handler) DeleteRole(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.roles.Delete(r.Context(), id); err != nil {
		h.writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListUsers returns every user with their role id and branches, plus the
// roles and branches to choose from.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.roles.ListUsers(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	roles, err := h.roles.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	type option struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	opts := make([]option, len(roles))
	for i, ro := range roles {
		opts[i] = option{ro.ID, ro.Name}
	}
	branches, err := h.roles.ListBranchOptions(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": users, "roles": opts, "branches": branches})
}

func (h *Handler) SetUserRole(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if _, err := uuid.Parse(userID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	if id, _ := auth.IdentityFrom(r.Context()); id.UserID == userID {
		httpx.WriteError(w, http.StatusBadRequest, "you cannot change your own role")
		return
	}
	var body struct {
		RoleID *int `json:"roleId"` // null removes the role
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	if err := h.roles.SetUserRole(r.Context(), userID, body.RoleID); err != nil {
		h.writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": userID, "roleId": body.RoleID})
}

// UserMenus returns the menu tree the user gets from their role, computed
// exactly like their own sidebar (GET /api/menus).
func (h *Handler) UserMenus(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if _, err := uuid.Parse(userID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	rows, err := h.menus.ListActive(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	a, err := h.menus.AccessOf(r.Context(), userID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"role":      a.RoleName,
		"allAccess": a.All,
		"menus":     menu.BuildTree(menu.Filter(rows, a)),
	})
}

// maxBranchCode is the size of refbranchauth.branch (varchar(10)).
const maxBranchCode = 10

// SetUserBranches replaces the branches a user is mapped to. Unlike the role,
// you may change your own branches: they do not grant access to pages.
func (h *Handler) SetUserBranches(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("id")
	if _, err := uuid.Parse(userID); err != nil {
		httpx.WriteError(w, http.StatusNotFound, "user not found")
		return
	}
	var body struct {
		Branches []string `json:"branches"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	codes, msg := normalizeCodes(body.Branches)
	if msg != "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"branches": msg})
		return
	}
	if err := h.roles.SetUserBranches(r.Context(), userID, codes); err != nil {
		h.writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"id": userID, "branches": codes})
}

// normalizeCodes trims, uppercases, de-duplicates and sorts branch codes.
func normalizeCodes(in []string) ([]string, string) {
	if len(in) > 200 {
		return nil, "Too many branches"
	}
	seen := map[string]bool{}
	out := []string{}
	for _, c := range in {
		c = strings.ToUpper(strings.TrimSpace(c))
		if c == "" || seen[c] {
			continue
		}
		if utf8.RuneCountInString(c) > maxBranchCode {
			return nil, "Branch code " + c + " is longer than 10 characters"
		}
		seen[c] = true
		out = append(out, c)
	}
	sort.Strings(out)
	return out, ""
}

func (h *Handler) callerAccess(r *http.Request) (menu.Access, error) {
	id, _ := auth.IdentityFrom(r.Context())
	return h.menus.AccessOf(r.Context(), id.UserID)
}

func (h *Handler) writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "not found")
	case errors.Is(err, ErrNameTaken):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": "role name already exists", "errors": httpx.FieldErrors{"name": "A role with this name already exists"},
		})
	case errors.Is(err, ErrInUse):
		httpx.WriteError(w, http.StatusConflict, "this role is still assigned to users: give them another role first")
	case errors.Is(err, ErrUnknownMenu):
		httpx.WriteError(w, http.StatusBadRequest, "menuIds contains an unknown menu")
	case errors.Is(err, ErrUnknownRole):
		httpx.WriteError(w, http.StatusBadRequest, "unknown role")
	case errors.Is(err, ErrUnknownBranch):
		httpx.WriteValidation(w, httpx.FieldErrors{"branches": "One of the branches does not exist"})
	default:
		httpx.InternalError(w, err)
	}
}

const maxNameLen = 50

func decodeInput(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	in.Name = strings.TrimSpace(in.Name)
	switch n := utf8.RuneCountInString(in.Name); {
	case n == 0:
		httpx.WriteValidation(w, httpx.FieldErrors{"name": "Name is required"})
		return in, false
	case n > maxNameLen:
		httpx.WriteValidation(w, httpx.FieldErrors{"name": "Name must be at most 50 characters"})
		return in, false
	}
	if in.MenuIDs == nil {
		in.MenuIDs = []int{}
	}
	return in, true
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "not found")
		return 0, false
	}
	return id, true
}
