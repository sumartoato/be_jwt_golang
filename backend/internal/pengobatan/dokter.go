package pengobatan

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Dokter is a doctor in the form's Dokter PJ lookup ("Data Dokter").
type Dokter struct {
	ID    int    `json:"id"`
	Nama  string `json:"nama"`
	Jenis string `json:"jenis"` // refjenisdokter, "" when not set
}

// Dokter lists the active doctors (refmdokter.status 1) mapped to a branch
// (refmdokterauth), by id like the original screen.
func (p *PostgresRepository) Dokter(ctx context.Context, branch string) ([]Dokter, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT d.id, trim(COALESCE(d.nama, '')), COALESCE(j.info, '')
		 FROM refmdokter d
		 JOIN refmdokterauth a ON a.dokterid = d.id AND a.cabang = $1
		 LEFT JOIN refjenisdokter j ON j.id = d.id_jenisdok
		 WHERE COALESCE(d.status, 1) = 1
		 ORDER BY d.id`, branch)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Dokter, error) {
		var d Dokter
		err := r.Scan(&d.ID, &d.Nama, &d.Jenis)
		return d, err
	})
	if list == nil {
		list = []Dokter{}
	}
	return list, err
}
