package pengobatan

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Penunjang is a supporting examination in the form's "Data Tindakan" lookup of
// the Penunjang tab: offered at the branch, with that branch's price.
type Penunjang struct {
	ID    int     `json:"id"`
	Nama  string  `json:"nama"`
	Jenis string  `json:"jenis"` // refjenispanjang (LAB MOHC, LABORATORIUM CV, ...)
	Harga float64 `json:"harga"` // refpenunjangdetail.hjual for the branch
}

// Penunjang lists the examinations that are active and offered at the branch
// (refpenunjang.isactive and refpenunjangdetail.isactive = 1), by id.
func (p *PostgresRepository) Penunjang(ctx context.Context, branch string) ([]Penunjang, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT p.id, COALESCE(p.nama, ''), COALESCE(j.info, ''), COALESCE(d.hjual, 0)
		 FROM refpenunjang p
		 JOIN refpenunjangdetail d ON d.id_penunjang = p.id AND d.cabang = $1
		 LEFT JOIN refjenispanjang j ON j.id = p.id_jenis
		 WHERE COALESCE(p.isactive, 1) = 1 AND COALESCE(d.isactive, 1) = 1
		 ORDER BY p.id`, branch)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Penunjang, error) {
		var x Penunjang
		err := r.Scan(&x.ID, &x.Nama, &x.Jenis, &x.Harga)
		return x, err
	})
	if list == nil {
		list = []Penunjang{}
	}
	return list, err
}
