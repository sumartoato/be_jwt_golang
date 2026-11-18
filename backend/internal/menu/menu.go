// Package menu serves the sidebar menu stored in the menus table, filtered by
// the caller's role, and guards API routes by menu access.
package menu

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/auth"
)

// Row is one record of the menus table.
type Row struct {
	ID       int
	ParentID *int
	Title    string
	Icon     string
	Path     *string
}

// Item is a node of the menu tree returned by the API.
type Item struct {
	ID       int     `json:"id"`
	Title    string  `json:"title"`
	Icon     string  `json:"icon"`
	Path     *string `json:"path"` // null for a group without its own page
	Children []*Item `json:"children"`
}

// Access is what a user's role grants. The zero value (no role) grants nothing.
type Access struct {
	RoleID   *int
	RoleName *string
	All      bool
	MenuIDs  map[int]bool
}

type Repository interface {
	// ListActive returns active rows ordered by sort_order, id.
	ListActive(ctx context.Context) ([]Row, error)
	// AccessOf returns the role grants of a user (zero Access if the user has no role).
	AccessOf(ctx context.Context, userID string) (Access, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) ListActive(ctx context.Context) ([]Row, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, parent_id, title, icon, path FROM menus WHERE is_active ORDER BY sort_order, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Row
	for rows.Next() {
		var m Row
		if err := rows.Scan(&m.ID, &m.ParentID, &m.Title, &m.Icon, &m.Path); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PostgresRepository) AccessOf(ctx context.Context, userID string) (Access, error) {
	var (
		a   Access
		id  int
		nm  string
		ids []int32
	)
	err := r.pool.QueryRow(ctx,
		`SELECT r.id, r.name, r.all_access,
		        COALESCE(array_agg(rm.menu_id) FILTER (WHERE rm.menu_id IS NOT NULL), '{}')
		 FROM users u
		 JOIN roles r ON r.id = u.role_id
		 LEFT JOIN role_menus rm ON rm.role_id = r.id
		 WHERE u.id = $1
		 GROUP BY r.id`, userID,
	).Scan(&id, &nm, &a.All, &ids)
	if errors.Is(err, pgx.ErrNoRows) {
		return Access{}, nil // unknown user or no role
	}
	if err != nil {
		return Access{}, err
	}
	a.RoleID, a.RoleName = &id, &nm
	a.MenuIDs = make(map[int]bool, len(ids))
	for _, m := range ids {
		a.MenuIDs[int(m)] = true
	}
	return a, nil
}

// Filter keeps the rows a role may see, in their original order:
//   - every row when the role has all access;
//   - a granted row and everything under it;
//   - the ancestors of a granted row, so it stays reachable in the tree.
func Filter(rows []Row, a Access) []Row {
	if a.All {
		return rows
	}
	parent := make(map[int]*int, len(rows))
	for _, r := range rows {
		parent[r.ID] = r.ParentID
	}
	// chain returns id, its parent, grandparent, ... (stopping on a cycle).
	chain := func(id int) []int {
		var out []int
		seen := map[int]bool{}
		for cur := &id; cur != nil && !seen[*cur]; cur = parent[*cur] {
			seen[*cur] = true
			out = append(out, *cur)
		}
		return out
	}
	visible := make(map[int]bool)
	for _, r := range rows {
		ch := chain(r.ID)
		for _, id := range ch {
			if a.MenuIDs[id] { // r or one of its ancestors is granted
				for _, v := range ch {
					visible[v] = true
				}
				break
			}
		}
	}
	out := []Row{}
	for _, r := range rows {
		if visible[r.ID] {
			out = append(out, r)
		}
	}
	return out
}

// CanOpen reports whether a reachable row in rows (already filtered) points to path.
func CanOpen(rows []Row, path string) bool {
	for _, it := range flatten(BuildTree(rows)) {
		if it.Path != nil && strings.TrimRight(*it.Path, "/") == strings.TrimRight(path, "/") {
			return true
		}
	}
	return false
}

func flatten(items []*Item) []*Item {
	var out []*Item
	for _, it := range items {
		out = append(out, it)
		out = append(out, flatten(it.Children)...)
	}
	return out
}

// BuildTree nests rows under their parents, keeping the input order among
// siblings. Rows whose parent is missing (inactive or deleted) are dropped,
// so deactivating a group hides everything under it.
func BuildTree(rows []Row) []*Item {
	items := make(map[int]*Item, len(rows))
	for _, r := range rows {
		items[r.ID] = &Item{ID: r.ID, Title: r.Title, Icon: r.Icon, Path: r.Path, Children: []*Item{}}
	}
	roots := []*Item{}
	for _, r := range rows {
		if r.ParentID == nil {
			roots = append(roots, items[r.ID])
		} else if parent, ok := items[*r.ParentID]; ok {
			parent.Children = append(parent.Children, items[r.ID])
		}
	}
	// Only nodes reachable from a root are returned, so an item stays hidden
	// when any ancestor is inactive, not just its direct parent.
	return roots
}

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers GET /api/menus behind requireAuth.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler) {
	mux.Handle("GET /api/menus", requireAuth(http.HandlerFunc(h.List)))
}

// visible returns the active rows the authenticated user's role may see.
func (h *Handler) visible(ctx context.Context) ([]Row, Access, error) {
	id, _ := auth.IdentityFrom(ctx)
	rows, err := h.repo.ListActive(ctx)
	if err != nil {
		return nil, Access{}, err
	}
	a, err := h.repo.AccessOf(ctx, id.UserID)
	if err != nil {
		return nil, Access{}, err
	}
	return Filter(rows, a), a, nil
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	rows, a, err := h.visible(r.Context())
	if err != nil {
		internalError(w, "list menus", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"menus": BuildTree(rows), "role": a.RoleName})
}

// RequireMenu lets a request through only when the caller's role can open
// the menu item with this path. It must run after the auth middleware.
// Access is read from the database on every request, so role changes apply at once.
func (h *Handler) RequireMenu(path string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rows, _, err := h.visible(r.Context())
			if err != nil {
				internalError(w, "check menu access", err)
				return
			}
			if !CanOpen(rows, path) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "your role cannot access this page"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func internalError(w http.ResponseWriter, what string, err error) {
	log.Printf("internal error: %s: %v", what, err)
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal server error"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
