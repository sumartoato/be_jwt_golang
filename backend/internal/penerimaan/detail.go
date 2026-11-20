package penerimaan

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ErrNotFound is returned for a receipt that does not exist.
var ErrNotFound = errors.New("penerimaan not found")

// DetailLine is a drug line of a saved receipt, as the form shows it.
type DetailLine struct {
	IDObat  int     `json:"idObat"`
	Nama    string  `json:"nama"`
	Jenis   string  `json:"jenis"`
	Satuan  string  `json:"satuan"`
	Harga   float64 `json:"harga"` // trxfakturobat.hbeli
	NoBatch string  `json:"noBatch"`
	Byk     float64 `json:"byk"`
	TglExp  *string `json:"tglExp"` // YYYY-MM-DD
	Disc    float64 `json:"disc"`   // percent
}

// Detail is a saved receipt for the View page (read-only).
type Detail struct {
	ID            int          `json:"id"` // No Trans
	NoGR          string       `json:"noGr"`
	TglTrans      *string      `json:"tglTrans"` // "YYYY-MM-DD HH:MM:SS"
	TglGR         *string      `json:"tglGr"`    // YYYY-MM-DD
	Cabang        string       `json:"cabang"`
	IDDistributor *int         `json:"idDistributor"`
	Distributor   string       `json:"distributor"`
	Supplier      string       `json:"supplier"` // "" when none
	CreateUser    string       `json:"createUser"`
	Total         float64      `json:"total"` // trxfakturtotal.total
	Obat          []DetailLine `json:"obat"`
}

// Detail loads a receipt with its lines.
func (p *PostgresRepository) Detail(ctx context.Context, id int) (Detail, error) {
	d := Detail{Obat: []DetailLine{}}
	err := p.pool.QueryRow(ctx, `SELECT f.id, COALESCE(f.no_fak, ''), to_char(f.tgl, 'YYYY-MM-DD HH24:MI:SS'), to_char(f.tgl_fak, 'YYYY-MM-DD'),
			COALESCE(f.cabang, ''), f.id_distributor, COALESCE(trim(di.distributor), ''), COALESCE(trim(s.nama), ''),
			COALESCE(f.create_user, ''), COALESCE(t.total, 0)
		FROM trxfaktur f
		LEFT JOIN trxfakturtotal t ON t.id_fak = f.id
		LEFT JOIN refdistributorobat di ON di.id = f.id_distributor
		LEFT JOIN refsupplier s ON s.id = f.id_supp
		WHERE f.id = $1`, id).Scan(&d.ID, &d.NoGR, &d.TglTrans, &d.TglGR, &d.Cabang, &d.IDDistributor, &d.Distributor, &d.Supplier,
		&d.CreateUser, &d.Total)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	rows, err := p.pool.Query(ctx, `SELECT fo.id_obat, COALESCE(o.nama, '(obat ' || fo.id_obat || ')'), COALESCE(jo.info, ''), COALESCE(sj.info, ''),
			COALESCE(fo.hbeli, 0), COALESCE(fo.no_batch, ''), COALESCE(fo.byk, 0), to_char(fo.tgl_exp, 'YYYY-MM-DD'), COALESCE(fo.disc, 0)
		FROM trxfakturobat fo
		LEFT JOIN refobat o ON o.id = fo.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
		WHERE fo.id_fak = $1 ORDER BY fo.id`, id)
	if err != nil {
		return d, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (DetailLine, error) {
		var l DetailLine
		err := r.Scan(&l.IDObat, &l.Nama, &l.Jenis, &l.Satuan, &l.Harga, &l.NoBatch, &l.Byk, &l.TglExp, &l.Disc)
		return l, err
	})
	if err != nil {
		return d, err
	}
	if list != nil {
		d.Obat = list
	}
	return d, nil
}
