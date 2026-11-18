// Package pendaftaran serves Front Office Klinik > Pendaftaran Pasien: patient
// visit registrations (trxpendaftaran). A user only sees and registers visits
// at the branches they are mapped to (refbranchauth). Queue numbers follow the
// legacy rules:
//
//   - no_antri: 1, 2, 3, ... per branch and day;
//   - no_antri_poli: 1, 2, 3, ... per branch, day and service;
//   - no_antri_poli_str: the service's firstcode + 3 digits (U001) when the
//     service needs a queue number (refjenisrawat.neednoantrian = 1).
package pendaftaran

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	_ "time/tzdata" // the runtime image has no zoneinfo; embed it for ClinicTimeZone

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

// ClinicTimeZone is the clinics' local time. Registration times and "today"
// use it, because the server itself runs in UTC.
const ClinicTimeZone = "Asia/Jakarta"

// clinicTodaySQL is today's clinic date in SQL.
const clinicTodaySQL = "(now() AT TIME ZONE '" + ClinicTimeZone + "')::date"

// Visit statuses (refstatustransaksi).
const (
	StatusDaftar = 3
	StatusCancel = 6
)

var (
	ErrNotFound       = errors.New("pendaftaran not found")
	ErrNotEditable    = errors.New("only registrations with status DAFTAR can be changed")
	ErrUnknownPasien  = errors.New("unknown pasien")
	ErrUnknownService = errors.New("unknown or inactive pelayanan")
)

// Row is a registration as listed, with the patient's details.
type Row struct {
	ID          int      `json:"id"`
	NoAntri     *int     `json:"noAntri"`
	NoAntriPoli *string  `json:"noAntriPoli"`
	TglMasuk    *string  `json:"tglMasuk"` // "YYYY-MM-DD HH:MM:SS", clinic time
	IDJenis     *int     `json:"idJenis"`
	Pelayanan   string   `json:"pelayanan"`
	IDStatus    *int     `json:"idStatus"`
	Status      string   `json:"status"`
	IDPas       *int     `json:"idPas"`
	NoRM        string   `json:"noRm"` // "" when the patient no longer exists
	Nama        string   `json:"nama"`
	TglLahir    *string  `json:"tglLahir"`
	Alamat      string   `json:"alamat"`
	Cabang      string   `json:"cabang"`
	PakaiKartu  bool     `json:"pakaiKartu"`
	TotalPoint  *float64 `json:"totalPoint"`
	CancelBy    string   `json:"cancelBy"`
	CancelAt    *string  `json:"cancelAt"`
	CancelNote  string   `json:"cancelReason"`
}

// sortColumns are the sortable list columns by API key (SQL is fixed, never from the request).
var sortColumns = map[string]string{
	"id":          "d.id",
	"noAntri":     "d.no_antri",
	"noAntriPoli": "d.no_antri_poli_str",
	"tglMasuk":    "d.tgl_masuk",
	"pelayanan":   "j.info",
	"status":      "s.info",
	"noRm":        "p.no_rm",
	"nama":        "p.nama",
	"tglLahir":    "p.tgl_lahir",
	"umur":        "p.tgl_lahir",
	"alamat":      "p.alamat",
	"cabang":      "d.cabang",
	"pakaiKartu":  "d.status_kartu",
	"totalPoint":  "pt.point",
}

// ListParams selects one page of registrations.
type ListParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string
	IDJenis  *int
	IDStatus *int
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type Page struct {
	Rows            []Row `json:"rows"`
	Total           int   `json:"total"`
	UnfilteredTotal int   `json:"unfilteredTotal"`
}

// PasienHit is a patient in the registration form's search.
type PasienHit struct {
	ID         int     `json:"id"`
	NoRM       string  `json:"noRm"`
	Nama       string  `json:"nama"`
	TglLahir   *string `json:"tglLahir"`
	Alamat     string  `json:"alamat"`
	NIK        string  `json:"nik"`
	HP         string  `json:"hp"`
	Penanggung string  `json:"penanggung"`
	Jamkes     string  `json:"jamkes"`
	Cabang     string  `json:"cabang"`
}

type PasienPage struct {
	Rows            []PasienHit `json:"rows"`
	Total           int         `json:"total"`
	UnfilteredTotal int         `json:"unfilteredTotal"`
}

// Service is a refjenisrawat entry offered in the form.
type Service struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	NeedQueue     bool   `json:"needQueue"`
	FirstCode     string `json:"firstCode"`
	IsExamination bool   `json:"isExamination"`
}

type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PasienSearch narrows the patient list of the registration form: Q matches
// name, No RM, NIK or address; By and Key search one field, like the legacy
// "Pencarian" panel.
type PasienSearch struct {
	Q   string
	By  string // one of PasienSearchFields, or empty
	Key string
	Age int // Key parsed as years when By is "umur"
}

// PasienSearchFields are the fields the "Pencarian" panel can search.
var PasienSearchFields = []string{"norm", "nama", "umur", "alamat", "jamkes"}

// NewInput registers a visit.
type NewInput struct {
	Tanggal    string `json:"tanggal"` // YYYY-MM-DD, clinic date
	IDPas      int    `json:"idPas"`
	IDJenis    int    `json:"idJenis"`
	Cabang     string `json:"cabang"`
	PakaiKartu bool   `json:"pakaiKartu"`
}

// EditInput changes a registration that is still DAFTAR.
type EditInput struct {
	IDJenis    int  `json:"idJenis"`
	PakaiKartu bool `json:"pakaiKartu"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	List(ctx context.Context, p ListParams) (Page, error)
	Get(ctx context.Context, id int) (Row, error)
	Services(ctx context.Context) ([]Service, error)
	Statuses(ctx context.Context) ([]Option, error)
	SearchPasien(ctx context.Context, branches []string, s PasienSearch, limit, offset int) (PasienPage, error)
	Create(ctx context.Context, in NewInput, user string) (int, error)
	Update(ctx context.Context, id int, in EditInput, user string) error
	Cancel(ctx context.Context, id int, reason, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

const fromJoins = ` FROM trxpendaftaran d
	LEFT JOIN refmpasien p ON p.id = d.id_pas
	LEFT JOIN refjenisrawat j ON j.id = d.id_jenis_rwt
	LEFT JOIN refstatustransaksi s ON s.id = d.id_statustr
	LEFT JOIN refpasienpoint pt ON pt.id_pas = d.id_pas`

const rowColumns = `SELECT d.id, d.no_antri, NULLIF(d.no_antri_poli_str, ''), to_char(d.tgl_masuk, 'YYYY-MM-DD HH24:MI:SS'),
	d.id_jenis_rwt, COALESCE(j.info, ''), d.id_statustr, COALESCE(s.info, ''), d.id_pas,
	COALESCE(p.no_rm, ''), COALESCE(p.nama, ''), to_char(p.tgl_lahir, 'YYYY-MM-DD'), COALESCE(p.alamat, ''),
	COALESCE(d.cabang, ''), COALESCE(d.status_kartu, 0) = 1, pt.point,
	COALESCE(d.cancelby, ''), to_char(d.cancelat, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(d.cancelreason, '')`

func scanRow(r pgx.CollectableRow) (Row, error) {
	var x Row
	err := r.Scan(&x.ID, &x.NoAntri, &x.NoAntriPoli, &x.TglMasuk, &x.IDJenis, &x.Pelayanan, &x.IDStatus, &x.Status, &x.IDPas,
		&x.NoRM, &x.Nama, &x.TglLahir, &x.Alamat, &x.Cabang, &x.PakaiKartu, &x.TotalPoint, &x.CancelBy, &x.CancelAt, &x.CancelNote)
	return x, err
}

// where builds the filter shared by the page and its counts.
func where(lp ListParams, withSearch bool) (string, []any) {
	args := []any{lp.Branches, lp.From, lp.To}
	conds := []string{`d.cabang = ANY($1)`, `d.tgl_masuk >= $2::date`, `d.tgl_masuk < $3::date + 1`}
	if lp.IDJenis != nil {
		args = append(args, *lp.IDJenis)
		conds = append(conds, fmt.Sprintf(`d.id_jenis_rwt = $%d`, len(args)))
	}
	if lp.IDStatus != nil {
		args = append(args, *lp.IDStatus)
		conds = append(conds, fmt.Sprintf(`d.id_statustr = $%d`, len(args)))
	}
	if withSearch && lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`p.nama ILIKE $%d OR p.no_rm ILIKE $%d OR p.alamat ILIKE $%d OR j.info ILIKE $%d`, n, n, n, n)
		var id int
		if _, err := fmt.Sscanf(lp.Search, "%d", &id); err == nil && fmt.Sprint(id) == lp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR d.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "d.id"
	}
	desc := lp.Desc
	if lp.Sort == "umur" {
		desc = !desc
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	cond, args := where(lp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`%s%s WHERE %s ORDER BY %s %s NULLS LAST, d.id %s LIMIT $%d OFFSET $%d`,
		rowColumns, fromJoins, cond, col, dir, dir, n+1, n+2), append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scanRow); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []Row{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*) FROM trxpendaftaran d WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Row, error) {
	rows, err := p.pool.Query(ctx, rowColumns+fromJoins+` WHERE d.id = $1`, id)
	if err != nil {
		return Row{}, err
	}
	list, err := pgx.CollectRows(rows, scanRow)
	if err != nil {
		return Row{}, err
	}
	if len(list) == 0 {
		return Row{}, ErrNotFound
	}
	return list[0], nil
}

func (p *PostgresRepository) Services(ctx context.Context) ([]Service, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, COALESCE(info, ''), neednoantrian = 1, COALESCE(firstcode, ''), COALESCE(ispemeriksaan, false)
		 FROM refjenisrawat WHERE isactive = 1 ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Service, error) {
		var s Service
		err := r.Scan(&s.ID, &s.Name, &s.NeedQueue, &s.FirstCode, &s.IsExamination)
		return s, err
	})
}

func (p *PostgresRepository) Statuses(ctx context.Context) ([]Option, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(info, '') FROM refstatustransaksi ORDER BY id`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
}

// SearchPasien pages the patients of the given branches for the registration form.
func (p *PostgresRepository) SearchPasien(ctx context.Context, branches []string, s PasienSearch, limit, offset int) (PasienPage, error) {
	page := PasienPage{Rows: []PasienHit{}}
	args := []any{branches}
	cond := `p.cabang = ANY($1)`
	if s.Q != "" {
		args = append(args, "%"+escapeLike(s.Q)+"%")
		cond += fmt.Sprintf(` AND (p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR p.nik ILIKE $%[1]d OR p.alamat ILIKE $%[1]d)`, len(args))
	}
	if s.By == "umur" {
		// Exactly Age whole years old today (clinic date): born after today-(Age+1)y, on or before today-Age y.
		args = append(args, s.Age)
		cond += fmt.Sprintf(` AND p.tgl_lahir > %[1]s - make_interval(years => $%[2]d + 1) AND p.tgl_lahir <= %[1]s - make_interval(years => $%[2]d)`,
			clinicTodaySQL, len(args))
	} else if s.By != "" && s.Key != "" {
		args = append(args, "%"+escapeLike(s.Key)+"%")
		n := len(args)
		switch s.By {
		case "norm":
			cond += fmt.Sprintf(` AND p.no_rm ILIKE $%d`, n)
		case "nama":
			cond += fmt.Sprintf(` AND p.nama ILIKE $%d`, n)
		case "alamat":
			cond += fmt.Sprintf(` AND p.alamat ILIKE $%d`, n)
		case "jamkes":
			cond += fmt.Sprintf(` AND p.id_jamkes IN (SELECT id FROM refjaminankesehatan WHERE nama ILIKE $%d)`, n)
		}
	}
	filtered := len(args) > 1
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(
		`SELECT p.id, p.no_rm, p.nama, to_char(p.tgl_lahir, 'YYYY-MM-DD'), COALESCE(p.alamat, ''), COALESCE(p.nik, ''),
		        COALESCE(p.hp, ''), COALESCE(p.penanggung, ''), COALESCE(jk.nama, ''), COALESCE(p.cabang, '')
		 FROM refmpasien p LEFT JOIN refjaminankesehatan jk ON jk.id = p.id_jamkes
		 WHERE %s ORDER BY p.id DESC LIMIT $%d OFFSET $%d`, cond, n+1, n+2), append(args, limit, offset)...)
	if err != nil {
		return page, err
	}
	page.Rows, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (PasienHit, error) {
		var h PasienHit
		err := r.Scan(&h.ID, &h.NoRM, &h.Nama, &h.TglLahir, &h.Alamat, &h.NIK, &h.HP, &h.Penanggung, &h.Jamkes, &h.Cabang)
		return h, err
	})
	if err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []PasienHit{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM refmpasien p WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if !filtered {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	err = p.pool.QueryRow(ctx, `SELECT count(*) FROM refmpasien p WHERE p.cabang = ANY($1)`, branches).Scan(&page.UnfilteredTotal)
	return page, err
}

// lockDay serializes queue numbering for one branch and day.
func lockDay(ctx context.Context, tx pgx.Tx, cabang, day string) error {
	_, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('trxpendaftaran.antri:' || $1 || ':' || $2))`, cabang, day)
	return err
}

// poliQueue returns the next poli number and its text for a service on a day.
func poliQueue(ctx context.Context, tx pgx.Tx, cabang, day string, idJenis int) (int, *string, error) {
	var (
		n         int
		needQueue bool
		code      string
	)
	err := tx.QueryRow(ctx,
		`SELECT COALESCE((SELECT max(no_antri_poli) FROM trxpendaftaran
		                  WHERE cabang = $1 AND tgl_masuk >= $2::date AND tgl_masuk < $2::date + 1 AND id_jenis_rwt = $3), 0) + 1,
		        j.neednoantrian = 1, COALESCE(j.firstcode, '')
		 FROM refjenisrawat j WHERE j.id = $3 AND j.isactive = 1`, cabang, day, idJenis).Scan(&n, &needQueue, &code)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil, ErrUnknownService
	}
	if err != nil {
		return 0, nil, err
	}
	if !needQueue || code == "" {
		return n, nil, nil
	}
	s := fmt.Sprintf("%s%03d", code, n%1000)
	return n, &s, nil
}

// Create registers a visit at the clinic's current time of day on in.Tanggal.
func (p *PostgresRepository) Create(ctx context.Context, in NewInput, user string) (int, error) {
	var id int
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refmpasien WHERE id = $1)`, in.IDPas).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrUnknownPasien
		}
		if err := lockDay(ctx, tx, in.Cabang, in.Tanggal); err != nil {
			return err
		}
		var antri int
		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(max(no_antri), 0) + 1 FROM trxpendaftaran
			 WHERE cabang = $1 AND tgl_masuk >= $2::date AND tgl_masuk < $2::date + 1`, in.Cabang, in.Tanggal).Scan(&antri); err != nil {
			return err
		}
		poli, poliStr, err := poliQueue(ctx, tx, in.Cabang, in.Tanggal, in.IDJenis)
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO trxpendaftaran (no_antri, tgl_masuk, id_pas, id_jenis_rwt, id_statustr, cabang, status_kartu,
			                             no_antri_poli, no_antri_poli_str, create_user, create_date)
			 VALUES ($1, $2::date + (now() AT TIME ZONE '`+ClinicTimeZone+`')::time, $3, $4, $5, $6, $7, $8, $9, $10,
			         now() AT TIME ZONE '`+ClinicTimeZone+`')
			 RETURNING id`,
			antri, in.Tanggal, in.IDPas, in.IDJenis, StatusDaftar, in.Cabang, kartu(in.PakaiKartu), poli, poliStr, user,
		).Scan(&id)
	})
	return id, translate(err)
}

// truncate cuts s to n characters (update_user is varchar(20), cancelby varchar(30)).
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		r = r[:n]
	}
	return string(r)
}

func kartu(v bool) int {
	if v {
		return 1
	}
	return 0
}

// Update changes the service and card use of a DAFTAR registration. A new
// service gets the next poli queue number of that day.
func (p *PostgresRepository) Update(ctx context.Context, id int, in EditInput, user string) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var (
			status  *int
			idJenis *int
			cabang  string
			day     string
		)
		err := tx.QueryRow(ctx,
			`SELECT id_statustr, id_jenis_rwt, COALESCE(cabang, ''), to_char(tgl_masuk, 'YYYY-MM-DD')
			 FROM trxpendaftaran WHERE id = $1 FOR UPDATE`, id).Scan(&status, &idJenis, &cabang, &day)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status == nil || *status != StatusDaftar {
			return ErrNotEditable
		}
		if idJenis == nil || *idJenis != in.IDJenis {
			if err := lockDay(ctx, tx, cabang, day); err != nil {
				return err
			}
			poli, poliStr, err := poliQueue(ctx, tx, cabang, day, in.IDJenis)
			if err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE trxpendaftaran SET id_jenis_rwt = $2, no_antri_poli = $3, no_antri_poli_str = $4 WHERE id = $1`,
				id, in.IDJenis, poli, poliStr); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx,
			`UPDATE trxpendaftaran SET status_kartu = $2, update_user = $3, update_date = now() AT TIME ZONE '`+ClinicTimeZone+`'
			 WHERE id = $1`, id, kartu(in.PakaiKartu), user)
		return err
	})
	return translate(err)
}

// Cancel sets a DAFTAR registration to CANCEL with who, when and why.
func (p *PostgresRepository) Cancel(ctx context.Context, id int, reason, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE trxpendaftaran SET id_statustr = $2, cancelby = $3, cancelat = now() AT TIME ZONE '`+ClinicTimeZone+`',
		        cancelreason = $4, update_user = $6, update_date = now() AT TIME ZONE '`+ClinicTimeZone+`'
		 WHERE id = $1 AND id_statustr = $5`, id, StatusCancel, user, reason, StatusDaftar, truncate(user, 20))
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var exists bool
		if err := p.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trxpendaftaran WHERE id = $1)`, id).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return ErrNotFound
		}
		return ErrNotEditable
	}
	return nil
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "fk_trxpendaftaran_jenis" {
		return ErrUnknownService
	}
	return err
}

// ClinicToday is today's date in the clinic's time zone.
func ClinicToday(now time.Time) string {
	loc, err := time.LoadLocation(ClinicTimeZone)
	if err != nil {
		loc = time.FixedZone("WIB", 7*60*60)
	}
	return now.In(loc).Format("2006-01-02")
}
