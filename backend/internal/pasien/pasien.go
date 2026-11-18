// Package pasien serves Master Data Klinik > Pasien: patients (refmpasien) and
// their lookups. There are ~430k patients, so the list is paged, searched and
// sorted in the database. A user only sees and edits patients of the branches
// they are mapped to (refbranchauth).
package pasien

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var (
	ErrNotFound    = errors.New("pasien not found")
	ErrNoRMExhaust = errors.New("no free No RM left in this branch")
)

// InvalidRefError means a lookup id does not exist.
type InvalidRefError struct{ Field string }

func (e InvalidRefError) Error() string { return "unknown " + e.Field }

// Row is a patient as shown in the list.
type Row struct {
	ID        int     `json:"id"`
	NoRM      string  `json:"noRm"`
	Nama      string  `json:"nama"`
	TmptLahir string  `json:"tmptLahir"`
	TglLahir  *string `json:"tglLahir"` // YYYY-MM-DD
	Alamat    string  `json:"alamat"`
	RT        string  `json:"rt"`
	RW        string  `json:"rw"`
	Cabang    string  `json:"cabang"`
}

// Pasien is the full record for the edit form, with the region names of its kelurahan.
type Pasien struct {
	Row
	IDKel       *int       `json:"idKel"`
	Kelurahan   string     `json:"kelurahan"`
	Kecamatan   string     `json:"kecamatan"`
	Kabupaten   string     `json:"kabupaten"`
	Provinsi    string     `json:"provinsi"`
	Telp        string     `json:"telp"`
	HP          string     `json:"hp"`
	IDGender    *int       `json:"idGender"`
	IDGolDarah  *int       `json:"idGolDarah"`
	IDAgama     *int       `json:"idAgama"`
	IDStatusKwn *int       `json:"idStatusKwn"`
	TglKun      *string    `json:"tglKun"` // first visit, YYYY-MM-DD
	Penanggung  string     `json:"penanggung"`
	IDJamkes    *int       `json:"idJamkes"`
	NoJamkes    string     `json:"noJamkes"`
	NIK         string     `json:"nik"`
	Alergi      string     `json:"alergi"`
	IHSID       string     `json:"ihsId"` // Satu Sehat patient id (read-only here)
	UpdatedBy   string     `json:"updatedBy"`
	UpdatedAt   *time.Time `json:"updatedAt"`
}

// Input is the editable part of refmpasien. No RM is generated on create and never changes.
type Input struct {
	Nama        string  `json:"nama"`
	TmptLahir   string  `json:"tmptLahir"`
	TglLahir    *string `json:"tglLahir"`
	Alamat      string  `json:"alamat"`
	IDKel       *int    `json:"idKel"`
	RT          string  `json:"rt"`
	RW          string  `json:"rw"`
	Telp        string  `json:"telp"`
	HP          string  `json:"hp"`
	IDGender    *int    `json:"idGender"`
	IDGolDarah  *int    `json:"idGolDarah"`
	IDAgama     *int    `json:"idAgama"`
	IDStatusKwn *int    `json:"idStatusKwn"`
	TglKun      *string `json:"tglKun"`
	Penanggung  string  `json:"penanggung"`
	IDJamkes    *int    `json:"idJamkes"`
	NoJamkes    string  `json:"noJamkes"`
	Cabang      string  `json:"cabang"`
	NIK         string  `json:"nik"`
	Alergi      string  `json:"alergi"`
}

const dateLayout = "2006-01-02"

// Normalize trims text, uppercases the branch and validates against the
// refmpasien column sizes. now is the current date (for "not in the future").
func (in *Input) Normalize(now time.Time) httpx.FieldErrors {
	for _, f := range []*string{&in.Nama, &in.TmptLahir, &in.Alamat, &in.RT, &in.RW, &in.Telp, &in.HP,
		&in.Penanggung, &in.NoJamkes, &in.Cabang, &in.NIK, &in.Alergi} {
		*f = strings.TrimSpace(*f)
	}
	in.Cabang = strings.ToUpper(in.Cabang)
	errs := httpx.FieldErrors{}
	required := func(field, label, v string) {
		if v == "" {
			errs[field] = label + " is required"
		}
	}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	date := func(field, label string, v **string, req bool) {
		if *v != nil && strings.TrimSpace(**v) == "" {
			*v = nil
		}
		if *v == nil {
			if req {
				errs[field] = label + " is required"
			}
			return
		}
		d, err := time.Parse(dateLayout, strings.TrimSpace(**v))
		if err != nil {
			errs[field] = label + " must be a date (YYYY-MM-DD)"
			return
		}
		if d.After(now) {
			errs[field] = label + " cannot be in the future"
			return
		}
		s := d.Format(dateLayout)
		*v = &s
	}

	required("nama", "Nama", in.Nama)
	maxLen("nama", "Nama", in.Nama, 50)
	required("tmptLahir", "Tempat Lahir", in.TmptLahir)
	maxLen("tmptLahir", "Tempat Lahir", in.TmptLahir, 30)
	date("tglLahir", "Tanggal Lahir", &in.TglLahir, true)
	required("alamat", "Alamat", in.Alamat)
	maxLen("alamat", "Alamat", in.Alamat, 100)
	if in.IDGender == nil {
		errs["idGender"] = "Jenis Kelamin is required"
	}
	required("cabang", "Cabang", in.Cabang)
	maxLen("cabang", "Cabang", in.Cabang, 10)
	maxLen("rt", "RT", in.RT, 4)
	maxLen("rw", "RW", in.RW, 4)
	maxLen("telp", "Telpon", in.Telp, 20)
	maxLen("hp", "Hp", in.HP, 50)
	maxLen("penanggung", "Penanggung", in.Penanggung, 50)
	maxLen("noJamkes", "Nomor Kartu Kesehatan", in.NoJamkes, 100)
	maxLen("nik", "NIK", in.NIK, 30)
	maxLen("alergi", "Alergi", in.Alergi, 2000)
	date("tglKun", "Tanggal Kunjungan Pertama", &in.TglKun, false)
	return errs
}

// FormatNoRM renders a record number like 890117 as "89-01-17".
func FormatNoRM(n int) string {
	return fmt.Sprintf("%02d-%02d-%02d", n/10000, (n/100)%100, n%100)
}

// plausibleBirth orders by birth date but puts impossible dates last (lab has
// birth dates from the year 1000 up to 9999).
const plausibleBirth = "(CASE WHEN tgl_lahir BETWEEN DATE '1900-01-01' AND current_date THEN tgl_lahir END)"

// sortColumns are the list columns that can be sorted, by API key (SQL is fixed, never from the request).
var sortColumns = map[string]string{
	"id":        "id",
	"noRm":      "no_rm",
	"nama":      "nama",
	"tmptLahir": "tmpt_lahir",
	"tglLahir":  plausibleBirth,
	"umur":      plausibleBirth, // age: same order, reversed direction
	"alamat":    "alamat",
	"rt":        "rt",
	"rw":        "rw",
	"cabang":    "cabang",
}

// ListParams selects one page of patients. Branches must not be empty.
type ListParams struct {
	Branches []string // the caller's branches (always applied)
	Search   string
	Sort     string // key of sortColumns
	Desc     bool
	Limit    int
	Offset   int
}

// Page is one page of the list.
type Page struct {
	Rows            []Row `json:"rows"`
	Total           int   `json:"total"`           // matching the search
	UnfilteredTotal int   `json:"unfilteredTotal"` // in the branches, without the search
}

// Option is an entry of a lookup table.
type Option struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsActive bool   `json:"isActive"`
}

type Options struct {
	Gender    []Option `json:"gender"`
	GolDarah  []Option `json:"golDarah"`
	Agama     []Option `json:"agama"`
	StatusKwn []Option `json:"statusKwn"`
	Jamkes    []Option `json:"jamkes"`
}

// Kelurahan is a search result with its region hierarchy.
type Kelurahan struct {
	ID        int    `json:"id"`
	Kelurahan string `json:"kelurahan"`
	Kecamatan string `json:"kecamatan"`
	Kabupaten string `json:"kabupaten"`
	Provinsi  string `json:"provinsi"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	List(ctx context.Context, p ListParams) (Page, error)
	Get(ctx context.Context, id int) (Pasien, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	Options(ctx context.Context) (Options, error)
	SearchKelurahan(ctx context.Context, q string, limit int) ([]Kelurahan, error)
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

// escapeLike makes s match literally inside ILIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// where builds the shared filter. Searching by name, No RM, NIK or address uses
// the trigram indexes; a number also matches the patient id (Kode).
func where(p ListParams, withSearch bool) (string, []any) {
	args := []any{p.Branches}
	sql := `cabang = ANY($1)`
	if withSearch && p.Search != "" {
		args = append(args, "%"+escapeLike(p.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`nama ILIKE $%d OR no_rm ILIKE $%d OR nik ILIKE $%d OR alamat ILIKE $%d`, n, n, n, n)
		if id, err := parseID(p.Search); err == nil {
			args = append(args, id)
			cond += fmt.Sprintf(` OR id = $%d`, len(args))
		}
		sql += ` AND (` + cond + `)`
	}
	return sql, args
}

func parseID(s string) (int, error) {
	var id int
	if _, err := fmt.Sscanf(s, "%d", &id); err != nil || fmt.Sprint(id) != s || id <= 0 {
		return 0, errors.New("not an id")
	}
	return id, nil
}

const rowColumns = `id, no_rm, nama, COALESCE(tmpt_lahir, ''), to_char(tgl_lahir, 'YYYY-MM-DD'),
	COALESCE(alamat, ''), COALESCE(rt, ''), COALESCE(rw, ''), COALESCE(cabang, '')`

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col, lp.Desc = "id", true
	}
	desc := lp.Desc
	if lp.Sort == "umur" {
		desc = !desc // older = earlier birth date
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}

	cond, args := where(lp, true)
	args = append(args, lp.Limit, lp.Offset)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM refmpasien WHERE %s ORDER BY %s %s NULLS LAST, id %s LIMIT $%d OFFSET $%d`,
		rowColumns, cond, col, dir, dir, len(args)-1, len(args)), args...)
	if err != nil {
		return page, err
	}
	page.Rows, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Row, error) {
		var x Row
		err := r.Scan(&x.ID, &x.NoRM, &x.Nama, &x.TmptLahir, &x.TglLahir, &x.Alamat, &x.RT, &x.RW, &x.Cabang)
		return x, err
	})
	if err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []Row{}
	}

	cond, args = where(lp, true)
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM refmpasien WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*) FROM refmpasien WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Pasien, error) {
	var x Pasien
	err := p.pool.QueryRow(ctx,
		`SELECT p.id, p.no_rm, p.nama, COALESCE(p.tmpt_lahir, ''), to_char(p.tgl_lahir, 'YYYY-MM-DD'),
		        COALESCE(p.alamat, ''), COALESCE(p.rt, ''), COALESCE(p.rw, ''), COALESCE(p.cabang, ''),
		        p.id_kel, COALESCE(k.info, ''), COALESCE(c.info, ''), COALESCE(b.info, ''), COALESCE(v.info, ''),
		        COALESCE(p.telp, ''), COALESCE(p.hp, ''), p.id_gender, p.id_goldarah, p.id_agama, p.id_status_kwn,
		        to_char(p.tgl_kun, 'YYYY-MM-DD'), COALESCE(p.penanggung, ''), p.id_jamkes, COALESCE(p.no_jamkes, ''),
		        COALESCE(p.nik, ''), COALESCE(p.alergi, ''), COALESCE(p.ihs_id, ''),
		        COALESCE(p.update_user, p.create_user, ''), COALESCE(p.update_date, p.create_date)
		 FROM refmpasien p
		 LEFT JOIN refkelurahan k ON k.id = p.id_kel
		 LEFT JOIN refkecamatan c ON c.id = k.id_kec
		 LEFT JOIN refkabupaten b ON b.id = c.id_kab
		 LEFT JOIN refprovinsi v ON v.id = b.id_prov
		 WHERE p.id = $1`, id,
	).Scan(&x.ID, &x.NoRM, &x.Nama, &x.TmptLahir, &x.TglLahir, &x.Alamat, &x.RT, &x.RW, &x.Cabang,
		&x.IDKel, &x.Kelurahan, &x.Kecamatan, &x.Kabupaten, &x.Provinsi,
		&x.Telp, &x.HP, &x.IDGender, &x.IDGolDarah, &x.IDAgama, &x.IDStatusKwn,
		&x.TglKun, &x.Penanggung, &x.IDJamkes, &x.NoJamkes, &x.NIK, &x.Alergi, &x.IHSID, &x.UpdatedBy, &x.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Pasien{}, ErrNotFound
	}
	return x, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// nextNoRM continues the branch's newest record number, skipping numbers
// already used in that branch. Callers hold the branch's advisory lock.
func nextNoRM(ctx context.Context, tx pgx.Tx, cabang string) (string, error) {
	var last int
	err := tx.QueryRow(ctx,
		`SELECT replace(no_rm, '-', '')::int FROM refmpasien
		 WHERE cabang = $1 AND no_rm ~ '^[0-9]{2}-[0-9]{2}-[0-9]{2}$'
		 ORDER BY id DESC LIMIT 1`, cabang).Scan(&last)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	for n := last + 1; n <= 999999; n++ {
		candidate := FormatNoRM(n)
		var used bool
		if err := tx.QueryRow(ctx,
			`SELECT EXISTS (SELECT 1 FROM refmpasien WHERE no_rm = $1 AND cabang = $2)`, candidate, cabang,
		).Scan(&used); err != nil {
			return "", err
		}
		if !used {
			return candidate, nil
		}
	}
	return "", ErrNoRMExhaust
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		// Serialize record-number generation per branch.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('refmpasien.no_rm:' || $1))`, in.Cabang); err != nil {
			return err
		}
		noRM, err := nextNoRM(ctx, tx, in.Cabang)
		if err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO refmpasien (no_rm, nama, tmpt_lahir, tgl_lahir, alamat, id_kel, rt, rw, telp, hp, id_gender,
			                         id_goldarah, id_agama, id_status_kwn, tgl_kun, penanggung, id_jamkes, no_jamkes,
			                         cabang, nik, alergi, create_user, create_date)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, COALESCE($15::date, current_date),
			         $16, $17, $18, $19, $20, $21, $22, now())
			 RETURNING id`,
			noRM, in.Nama, nullIfEmpty(in.TmptLahir), in.TglLahir, nullIfEmpty(in.Alamat), in.IDKel,
			nullIfEmpty(in.RT), nullIfEmpty(in.RW), nullIfEmpty(in.Telp), nullIfEmpty(in.HP), in.IDGender,
			in.IDGolDarah, in.IDAgama, in.IDStatusKwn, in.TglKun, nullIfEmpty(in.Penanggung), in.IDJamkes,
			nullIfEmpty(in.NoJamkes), in.Cabang, nullIfEmpty(in.NIK), nullIfEmpty(in.Alergi), user,
		).Scan(&id)
	})
	return id, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refmpasien SET nama = $2, tmpt_lahir = $3, tgl_lahir = $4, alamat = $5, id_kel = $6, rt = $7, rw = $8,
		        telp = $9, hp = $10, id_gender = $11, id_goldarah = $12, id_agama = $13, id_status_kwn = $14,
		        tgl_kun = $15, penanggung = $16, id_jamkes = $17, no_jamkes = $18, cabang = $19, nik = $20,
		        alergi = $21, update_user = $22, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, nullIfEmpty(in.TmptLahir), in.TglLahir, nullIfEmpty(in.Alamat), in.IDKel,
		nullIfEmpty(in.RT), nullIfEmpty(in.RW), nullIfEmpty(in.Telp), nullIfEmpty(in.HP), in.IDGender,
		in.IDGolDarah, in.IDAgama, in.IDStatusKwn, in.TglKun, nullIfEmpty(in.Penanggung), in.IDJamkes,
		nullIfEmpty(in.NoJamkes), in.Cabang, nullIfEmpty(in.NIK), nullIfEmpty(in.Alergi), user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) Options(ctx context.Context) (Options, error) {
	var o Options
	lists := []struct {
		dst   *[]Option
		query string
	}{
		{&o.Gender, `SELECT id, COALESCE(info, ''), true FROM refgender ORDER BY id`},
		{&o.GolDarah, `SELECT id, COALESCE(info, ''), true FROM refgoldarah ORDER BY id`},
		{&o.Agama, `SELECT id, COALESCE(info, ''), true FROM refagama ORDER BY id`},
		{&o.StatusKwn, `SELECT id, COALESCE(info, ''), true FROM refstatuskawin ORDER BY id`},
		{&o.Jamkes, `SELECT id, nama, COALESCE(isactive, 0) = 1 FROM refjaminankesehatan ORDER BY nama, id`},
	}
	for _, l := range lists {
		rows, err := p.pool.Query(ctx, l.query)
		if err != nil {
			return o, err
		}
		opts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
			var op Option
			err := r.Scan(&op.ID, &op.Name, &op.IsActive)
			return op, err
		})
		if err != nil {
			return o, err
		}
		*l.dst = opts
	}
	return o, nil
}

func (p *PostgresRepository) SearchKelurahan(ctx context.Context, q string, limit int) ([]Kelurahan, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT k.id, COALESCE(k.info, ''), COALESCE(c.info, ''), COALESCE(b.info, ''), COALESCE(v.info, '')
		 FROM refkelurahan k
		 LEFT JOIN refkecamatan c ON c.id = k.id_kec
		 LEFT JOIN refkabupaten b ON b.id = c.id_kab
		 LEFT JOIN refprovinsi v ON v.id = b.id_prov
		 WHERE k.info ILIKE $1 OR c.info ILIKE $1 OR b.info ILIKE $1
		 ORDER BY (k.info ILIKE $2) DESC, k.info, k.id
		 LIMIT $3`, "%"+escapeLike(q)+"%", escapeLike(q)+"%", limit)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Kelurahan, error) {
		var k Kelurahan
		err := r.Scan(&k.ID, &k.Kelurahan, &k.Kecamatan, &k.Kabupaten, &k.Provinsi)
		return k, err
	})
	if list == nil {
		list = []Kelurahan{}
	}
	return list, err
}

// refFields maps foreign keys to the request field they come from.
var refFields = map[string]string{
	"fk_refmpasien_kel":       "idKel",
	"fk_refmpasien_gender":    "idGender",
	"fk_refmpasien_goldarah":  "idGolDarah",
	"fk_refmpasien_agama":     "idAgama",
	"fk_refmpasien_statuskwn": "idStatusKwn",
	"fk_refmpasien_jamkes":    "idJamkes",
	"fk_refmpasien_cabang":    "cabang",
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23503" {
		if f, ok := refFields[pgErr.ConstraintName]; ok {
			return InvalidRefError{Field: f}
		}
	}
	return err
}
