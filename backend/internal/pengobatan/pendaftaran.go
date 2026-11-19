package pengobatan

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
)

// BillableStatuses are the visit statuses (refstatustransaksi) a treatment can
// still be recorded for: DAFTAR, PROSES and PAYMENT. The form's registration
// lookup ("Data Pendaftaran") only offers these, and only when no sale refers
// to the registration yet.
var BillableStatuses = []int{3, 4, 5}

// RegRow is a registration in the form's lookup, with what the form fills in.
type RegRow struct {
	ID            int     `json:"id"`
	TglMasuk      *string `json:"tglMasuk"` // "YYYY-MM-DD HH:MM:SS", clinic time
	IDJenisRawat  *int    `json:"idJenisRawat"`
	Pelayanan     string  `json:"pelayanan"`
	IsPemeriksaan bool    `json:"isPemeriksaan"` // a doctor's examination (poli)
	IDStatus      *int    `json:"idStatus"`
	Status        string  `json:"status"`
	IDPas         *int    `json:"idPas"`
	NoRM          string  `json:"noRm"` // "" when the patient no longer exists
	Nama          string  `json:"nama"`
	TglLahir      *string `json:"tglLahir"`
	Alamat        string  `json:"alamat"`
	HP            string  `json:"hp"` // mobile, else phone
	Penanggung    string  `json:"penanggung"`
	IDJamkes      *int    `json:"idJamkes"`
	Jamkes        string  `json:"jamkes"`
	NoJamkes      string  `json:"noJamkes"`
	Cabang        string  `json:"cabang"`
	// Dokter PJ: the examination's doctor (trxpemeriksaan.id_dok); for services
	// without an examination the branch's NON DOKTER; nil when not chosen yet.
	IDDokter *int   `json:"idDokter"`
	Dokter   string `json:"dokter"`
}

// RegParams selects one page of the registration lookup.
type RegParams struct {
	Branch       string // required, one of the caller's branches
	From, To     string // YYYY-MM-DD, inclusive, clinic dates
	Nama         string
	IDJenisRawat *int
	IDStatus     *int // one of BillableStatuses; nil = all of them
	Search       string
	Sort         string
	Desc         bool
	Limit        int
	Offset       int
}

type RegPage struct {
	Rows            []RegRow `json:"rows"`
	Total           int      `json:"total"`
	UnfilteredTotal int      `json:"unfilteredTotal"`
}

var regSortColumns = map[string]string{
	"id":        "d.id",
	"tglMasuk":  "d.tgl_masuk",
	"pelayanan": "r.info",
	"status":    "s.info",
	"noRm":      "p.no_rm",
	"nama":      "p.nama",
	"umur":      "p.tgl_lahir",
	"alamat":    "p.alamat",
	"cabang":    "d.cabang",
	"dokter":    "dk.nama",
}

const regFrom = ` FROM trxpendaftaran d
	LEFT JOIN refmpasien p ON p.id = d.id_pas
	LEFT JOIN refjenisrawat r ON r.id = d.id_jenis_rwt
	LEFT JOIN refstatustransaksi s ON s.id = d.id_statustr
	LEFT JOIN refjaminankesehatan jk ON jk.id = p.id_jamkes
	LEFT JOIN trxpemeriksaan pm ON pm.id_kun = d.id
	LEFT JOIN refmdokter dk ON dk.id = pm.id_dok`

const regColumns = `SELECT d.id, to_char(d.tgl_masuk, 'YYYY-MM-DD HH24:MI:SS'), d.id_jenis_rwt, COALESCE(r.info, ''),
	COALESCE(r.ispemeriksaan, false), d.id_statustr, COALESCE(s.info, ''), d.id_pas, COALESCE(p.no_rm, ''),
	COALESCE(p.nama, ''), to_char(p.tgl_lahir, 'YYYY-MM-DD'), COALESCE(p.alamat, ''),
	COALESCE(NULLIF(trim(p.hp), ''), NULLIF(trim(p.telp), ''), ''), COALESCE(p.penanggung, ''), NULLIF(p.id_jamkes, 0),
	COALESCE(jk.nama, ''), COALESCE(p.no_jamkes, ''), COALESCE(d.cabang, ''), dk.id, COALESCE(trim(dk.nama), '')`

func scanReg(r pgx.CollectableRow) (RegRow, error) {
	var x RegRow
	err := r.Scan(&x.ID, &x.TglMasuk, &x.IDJenisRawat, &x.Pelayanan, &x.IsPemeriksaan, &x.IDStatus, &x.Status, &x.IDPas,
		&x.NoRM, &x.Nama, &x.TglLahir, &x.Alamat, &x.HP, &x.Penanggung, &x.IDJamkes, &x.Jamkes, &x.NoJamkes, &x.Cabang,
		&x.IDDokter, &x.Dokter)
	return x, err
}

// regWhere builds the lookup filter; without withSearch it leaves out the search box.
func regWhere(rp RegParams, withSearch bool) (string, []any) {
	statuses := BillableStatuses
	if rp.IDStatus != nil {
		statuses = []int{*rp.IDStatus}
	}
	args := []any{rp.Branch, rp.From, rp.To, statuses}
	conds := []string{
		`d.cabang = $1`, `d.tgl_masuk >= $2::date`, `d.tgl_masuk < $3::date + 1`, `d.id_statustr = ANY($4)`,
		`NOT EXISTS (SELECT 1 FROM trxjual j WHERE j.id_kun = d.id)`,
	}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if rp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(rp.Nama)+"%")
	}
	if rp.IDJenisRawat != nil {
		add(`d.id_jenis_rwt = $%d`, *rp.IDJenisRawat)
	}
	if withSearch && rp.Search != "" {
		args = append(args, "%"+escapeLike(rp.Search)+"%")
		cond := fmt.Sprintf(`p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR p.alamat ILIKE $%[1]d OR r.info ILIKE $%[1]d`, len(args))
		var id int
		if _, err := fmt.Sscanf(rp.Search, "%d", &id); err == nil && fmt.Sprint(id) == rp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR d.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) Pendaftaran(ctx context.Context, rp RegParams) (RegPage, error) {
	page := RegPage{Rows: []RegRow{}}
	col, ok := regSortColumns[rp.Sort]
	if !ok {
		col = "d.id"
	}
	desc := rp.Desc
	if rp.Sort == "umur" {
		desc = !desc // older birth date = higher age
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	cond, args := regWhere(rp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`%s%s WHERE %s ORDER BY %s %s NULLS LAST, d.id %s LIMIT $%d OFFSET $%d`,
		regColumns, regFrom, cond, col, dir, dir, n+1, n+2), append(args, rp.Limit, rp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scanReg); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []RegRow{}
	}
	// Services without an examination get the branch's NON DOKTER as Dokter PJ.
	var nonID *int
	var nonName string
	for i := range page.Rows {
		r := &page.Rows[i]
		if r.IDDokter != nil || r.IsPemeriksaan {
			continue
		}
		if nonID == nil {
			var id int
			err := p.pool.QueryRow(ctx,
				`SELECT dk.id, trim(dk.nama) FROM refmdokter dk JOIN refmdokterauth a ON a.dokterid = dk.id AND a.cabang = $1
				 WHERE upper(trim(dk.nama)) = 'NON DOKTER' ORDER BY dk.id LIMIT 1`, rp.Branch).Scan(&id, &nonName)
			if err == pgx.ErrNoRows {
				break
			}
			if err != nil {
				return page, err
			}
			nonID = &id
		}
		r.IDDokter, r.Dokter = nonID, nonName
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+regFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if rp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = regWhere(rp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+regFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// ProsesStatus lists the visit statuses the lookup's "Proses" filter offers.
func (p *PostgresRepository) ProsesStatus(ctx context.Context) ([]Option, error) {
	return p.options(ctx, fmt.Sprintf(`SELECT id, COALESCE(info, '') FROM refstatustransaksi WHERE id IN (%d, %d, %d) ORDER BY id`,
		BillableStatuses[0], BillableStatuses[1], BillableStatuses[2]))
}
