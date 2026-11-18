// Package role manages roles, the menus each role can open, and which role
// each user has.
package role

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound      = errors.New("not found")
	ErrNameTaken     = errors.New("role name already exists")
	ErrInUse         = errors.New("role is assigned to users")
	ErrUnknownMenu   = errors.New("unknown menu id")
	ErrUnknownRole   = errors.New("unknown role id")
	ErrUnknownBranch = errors.New("unknown branch code")
)

type Role struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	AllAccess bool   `json:"allAccess"`
	MenuIDs   []int  `json:"menuIds"`
	UserCount int    `json:"userCount"`
}

// Input is the editable part of a role.
type Input struct {
	Name      string `json:"name"`
	AllAccess bool   `json:"allAccess"`
	MenuIDs   []int  `json:"menuIds"`
}

type User struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	RoleID *int   `json:"roleId"`
	// Branches are the refbranch codes the user is mapped to (refbranchauth).
	Branches []string `json:"branches"`
}

// BranchOption is a branch a user can be mapped to.
type BranchOption struct {
	Kode     string `json:"kode"`
	Nama     string `json:"nama"`
	IsActive bool   `json:"isActive"`
}

type Repository interface {
	List(ctx context.Context) ([]Role, error)
	Create(ctx context.Context, in Input) (Role, error)
	Update(ctx context.Context, id int, in Input) (Role, error)
	Delete(ctx context.Context, id int) error
	ListUsers(ctx context.Context) ([]User, error)
	SetUserRole(ctx context.Context, userID string, roleID *int) error
	ListBranchOptions(ctx context.Context) ([]BranchOption, error)
	// SetUserBranches replaces the user's branches with codes.
	SetUserBranches(ctx context.Context, userID string, codes []string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const selectRoles = `
SELECT r.id, r.name, r.all_access,
       COALESCE((SELECT array_agg(menu_id ORDER BY menu_id) FROM role_menus WHERE role_id = r.id), '{}'),
       (SELECT count(*) FROM users WHERE role_id = r.id)
FROM roles r`

func scanRoles(rows pgx.Rows) ([]Role, error) {
	defer rows.Close()
	out := []Role{}
	for rows.Next() {
		var (
			r   Role
			ids []int32
		)
		if err := rows.Scan(&r.ID, &r.Name, &r.AllAccess, &ids, &r.UserCount); err != nil {
			return nil, err
		}
		r.MenuIDs = make([]int, len(ids))
		for i, id := range ids {
			r.MenuIDs[i] = int(id)
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) List(ctx context.Context) ([]Role, error) {
	rows, err := p.pool.Query(ctx, selectRoles+` ORDER BY r.id`)
	if err != nil {
		return nil, err
	}
	return scanRoles(rows)
}

func (p *PostgresRepository) get(ctx context.Context, id int) (Role, error) {
	rows, err := p.pool.Query(ctx, selectRoles+` WHERE r.id = $1`, id)
	if err != nil {
		return Role{}, err
	}
	roles, err := scanRoles(rows)
	if err != nil {
		return Role{}, err
	}
	if len(roles) == 0 {
		return Role{}, ErrNotFound
	}
	return roles[0], nil
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (Role, error) {
	var id int
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx,
			`INSERT INTO roles (name, all_access) VALUES ($1, $2) RETURNING id`, in.Name, in.AllAccess,
		).Scan(&id); err != nil {
			return err
		}
		return saveMenus(ctx, tx, id, in.MenuIDs)
	})
	if err != nil {
		return Role{}, translate(err)
	}
	return p.get(ctx, id)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Role, error) {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE roles SET name = $2, all_access = $3 WHERE id = $1`, id, in.Name, in.AllAccess)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return saveMenus(ctx, tx, id, in.MenuIDs)
	})
	if err != nil {
		return Role{}, translate(err)
	}
	return p.get(ctx, id)
}

// saveMenus replaces the menus granted to a role.
func saveMenus(ctx context.Context, tx pgx.Tx, roleID int, menuIDs []int) error {
	if _, err := tx.Exec(ctx, `DELETE FROM role_menus WHERE role_id = $1`, roleID); err != nil {
		return err
	}
	if len(menuIDs) == 0 {
		return nil
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO role_menus (role_id, menu_id) SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`,
		roleID, menuIDs)
	return err
}

func (p *PostgresRepository) Delete(ctx context.Context, id int) error {
	tag, err := p.pool.Exec(ctx, `DELETE FROM roles WHERE id = $1`, id)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) ListUsers(ctx context.Context) ([]User, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT u.id, u.email, u.role_id,
		        COALESCE(array_agg(a.branch ORDER BY a.branch) FILTER (WHERE a.branch IS NOT NULL), '{}')
		 FROM users u
		 LEFT JOIN refbranchauth a ON a.userid = u.id::text
		 GROUP BY u.id
		 ORDER BY u.email`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []User{}
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.RoleID, &u.Branches); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) SetUserRole(ctx context.Context, userID string, roleID *int) error {
	tag, err := p.pool.Exec(ctx, `UPDATE users SET role_id = $2 WHERE id = $1`, userID, roleID)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			return ErrUnknownRole
		}
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) ListBranchOptions(ctx context.Context) ([]BranchOption, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT kode, COALESCE(nama, ''), COALESCE(isactive, 0) = 1 FROM refbranch ORDER BY nama, kode`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BranchOption{}
	for rows.Next() {
		var b BranchOption
		if err := rows.Scan(&b.Kode, &b.Nama, &b.IsActive); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (p *PostgresRepository) SetUserBranches(ctx context.Context, userID string, codes []string) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, userID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		if _, err := tx.Exec(ctx, `DELETE FROM refbranchauth WHERE userid = $1`, userID); err != nil {
			return err
		}
		if len(codes) == 0 {
			return nil
		}
		_, err := tx.Exec(ctx,
			`INSERT INTO refbranchauth (branch, userid) SELECT unnest($2::text[]), $1 ON CONFLICT DO NOTHING`,
			userID, codes)
		return err
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" && pgErr.TableName == "refbranchauth" {
			return ErrUnknownBranch
		}
		return err
	}
	return nil
}

// translate maps constraint violations to the package errors.
func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		return err
	}
	switch {
	case pgErr.Code == "23505" && pgErr.TableName == "roles":
		return ErrNameTaken
	case pgErr.Code == "23503" && pgErr.TableName == "role_menus":
		return ErrUnknownMenu
	case pgErr.Code == "23503" && pgErr.TableName == "users":
		return ErrInUse // deleting a role that users still reference
	}
	return err
}
