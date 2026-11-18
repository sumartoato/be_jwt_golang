// Package branchauth answers which branches a user is mapped to (refbranchauth).
// Pages with data per branch (Obat, Tindakan, ...) show and edit only those branches.
package branchauth

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Branch is a branch the user is mapped to.
type Branch struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// Querier is satisfied by *pgxpool.Pool and pgx.Tx.
type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// ForUser returns the user's branches ordered by code.
func ForUser(ctx context.Context, q Querier, userID string) ([]Branch, error) {
	rows, err := q.Query(ctx,
		`SELECT b.kode, COALESCE(b.nama, '')
		 FROM refbranchauth a JOIN refbranch b ON b.kode = a.branch
		 WHERE a.userid = $1
		 ORDER BY b.kode`, userID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Branch, error) {
		var b Branch
		err := r.Scan(&b.Kode, &b.Nama)
		return b, err
	})
	if list == nil {
		list = []Branch{}
	}
	return list, err
}

// Codes returns the branch codes.
func Codes(list []Branch) []string {
	out := make([]string, len(list))
	for i, b := range list {
		out[i] = b.Kode
	}
	return out
}

// Has reports whether kode is one of the branches.
func Has(list []Branch, kode string) bool {
	for _, b := range list {
		if b.Kode == kode {
			return true
		}
	}
	return false
}
