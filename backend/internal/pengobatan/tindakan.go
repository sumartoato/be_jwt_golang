package pengobatan

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Tindakan is a procedure in the form's "Data Tindakan Dokter" lookup: one the
// doctor performs (trxtindakan), offered at the branch, with that branch's price.
type Tindakan struct {
	ID     int     `json:"id"`
	Nama   string  `json:"nama"`
	Jenis  string  `json:"jenis"`  // refjenistindakan
	Harga  float64 `json:"harga"`  // reftindakandetail.hjual for the branch
	IsEdit bool    `json:"isEdit"` // the price may be changed in the form (reftindakan.isedit = 1)
}

// Tindakan lists a doctor's procedures that are active and offered at the
// branch (reftindakan.isactive and reftindakandetail.isactive = 1), by id.
func (p *PostgresRepository) Tindakan(ctx context.Context, branch string, dokterID int) ([]Tindakan, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT t.id, COALESCE(t.nama, ''), COALESCE(j.info, ''), COALESCE(d.hjual, 0), COALESCE(t.isedit, 0) = 1
		 FROM trxtindakan x
		 JOIN reftindakan t ON t.id = x.id_tindakan
		 JOIN reftindakandetail d ON d.id_tindakan = t.id AND d.cabang = $1
		 LEFT JOIN refjenistindakan j ON j.id = t.id_jenis
		 WHERE x.id_dokter = $2 AND COALESCE(t.isactive, 1) = 1 AND COALESCE(d.isactive, 1) = 1
		 ORDER BY t.id`, branch, dokterID)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Tindakan, error) {
		var t Tindakan
		err := r.Scan(&t.ID, &t.Nama, &t.Jenis, &t.Harga, &t.IsEdit)
		return t, err
	})
	if list == nil {
		list = []Tindakan{}
	}
	return list, err
}
