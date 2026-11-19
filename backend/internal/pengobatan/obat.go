package pengobatan

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Obat is a drug in the form's "Data Obat" lookup: sold at the branch, with
// that branch's sale price and stock (trxstock).
type Obat struct {
	ID        int     `json:"id"`
	Nama      string  `json:"nama"`
	Jenis     string  `json:"jenis"`  // refjenisobat
	Satuan    string  `json:"satuan"` // sale unit (refsatuan)
	Harga     float64 `json:"harga"`  // trxstock.hjual
	Stok      float64 `json:"stok"`
	KenaPajak bool    `json:"kenaPajak"` // PPN applies (refobat.kena_pajak = 1)
}

// Obat lists the drugs sold at a branch (trxstock.isactive = 1), by id like the
// original screen.
func (p *PostgresRepository) Obat(ctx context.Context, branch string) ([]Obat, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT o.id, o.nama, COALESCE(j.info, ''), COALESCE(sj.info, ''), COALESCE(s.hjual, 0), COALESCE(s.stok, 0),
		        COALESCE(o.kena_pajak, 0) = 1
		 FROM trxstock s
		 JOIN refobat o ON o.id = s.id_obat
		 LEFT JOIN refjenisobat j ON j.id = o.id_jenis
		 LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
		 WHERE s.cabang = $1 AND s.isactive = 1
		 ORDER BY o.id`, branch)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Obat, error) {
		var o Obat
		err := r.Scan(&o.ID, &o.Nama, &o.Jenis, &o.Satuan, &o.Harga, &o.Stok, &o.KenaPajak)
		return o, err
	})
	if list == nil {
		list = []Obat{}
	}
	return list, err
}
