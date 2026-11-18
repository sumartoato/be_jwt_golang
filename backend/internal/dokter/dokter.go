// Package dokter serves Master Data Klinik > Dokter: doctors (refmdokter), the
// procedures they may perform (trxtindakan) and the branches they practise at
// (refmdokterauth). Practice branches can only be added or removed for the
// branches the user is mapped to (refbranchauth).
package dokter

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

var ErrNotFound = errors.New("dokter not found")

// InvalidRefError means a lookup or procedure id does not exist.
type InvalidRefError struct{ Field string }

func (e InvalidRefError) Error() string { return "unknown " + e.Field }

// Dokter is a refmdokter row with the codes of its practice branches.
type Dokter struct {
	ID            int        `json:"id"`
	Nama          string     `json:"nama"`
	IDJenis       *int       `json:"idJenis"`
	TglMasuk      *string    `json:"tglMasuk"` // YYYY-MM-DD
	TmptLahir     string     `json:"tmptLahir"`
	TglLahir      *string    `json:"tglLahir"`
	Alamat        string     `json:"alamat"`
	Telp          string     `json:"telp"`
	HP            string     `json:"hp"`
	Pendidikan    string     `json:"pendidikan"`
	Spesialis     string     `json:"spesialis"`
	IDGender      *int       `json:"idGender"`
	IDAgama       *int       `json:"idAgama"`
	IDGolDarah    *int       `json:"idGolDarah"`
	IDStatusKwn   *int       `json:"idStatusKwn"`
	NIK           string     `json:"nik"`
	IsActive      bool       `json:"isActive"` // status 1
	CabangPraktek []string   `json:"cabangPraktek"`
	UpdatedBy     string     `json:"updatedBy"`
	UpdatedAt     *time.Time `json:"updatedAt"`
}

// Input is the editable part of refmdokter.
type Input struct {
	Nama        string  `json:"nama"`
	IDJenis     *int    `json:"idJenis"`
	TglMasuk    *string `json:"tglMasuk"`
	TmptLahir   string  `json:"tmptLahir"`
	TglLahir    *string `json:"tglLahir"`
	Alamat      string  `json:"alamat"`
	Telp        string  `json:"telp"`
	HP          string  `json:"hp"`
	Pendidikan  string  `json:"pendidikan"`
	Spesialis   string  `json:"spesialis"`
	IDGender    *int    `json:"idGender"`
	IDAgama     *int    `json:"idAgama"`
	IDGolDarah  *int    `json:"idGolDarah"`
	IDStatusKwn *int    `json:"idStatusKwn"`
	NIK         string  `json:"nik"`
	IsActive    bool    `json:"isActive"`
}

// Normalize trims text and validates against the refmdokter column sizes.
func (in *Input) Normalize(now time.Time) httpx.FieldErrors {
	for _, f := range []*string{&in.Nama, &in.TmptLahir, &in.Alamat, &in.Telp, &in.HP, &in.Pendidikan, &in.Spesialis, &in.NIK} {
		*f = strings.TrimSpace(*f)
	}
	errs := httpx.FieldErrors{}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	date := func(field, label string, v **string) {
		if *v != nil && strings.TrimSpace(**v) == "" {
			*v = nil
		}
		if *v == nil {
			return
		}
		d, err := time.Parse("2006-01-02", strings.TrimSpace(**v))
		switch {
		case err != nil:
			errs[field] = label + " must be a date (YYYY-MM-DD)"
		case d.After(now):
			errs[field] = label + " cannot be in the future"
		default:
			s := d.Format("2006-01-02")
			*v = &s
		}
	}
	if in.Nama == "" {
		errs["nama"] = "Nama Dokter is required"
	}
	maxLen("nama", "Nama Dokter", in.Nama, 30)
	maxLen("tmptLahir", "Tempat Lahir", in.TmptLahir, 25)
	maxLen("alamat", "Alamat", in.Alamat, 100)
	maxLen("telp", "Telp", in.Telp, 20)
	maxLen("hp", "Hp", in.HP, 20)
	maxLen("pendidikan", "Pendidikan", in.Pendidikan, 30)
	maxLen("spesialis", "Spesialis", in.Spesialis, 30)
	maxLen("nik", "NIK", in.NIK, 20)
	date("tglLahir", "Tanggal Lahir", &in.TglLahir)
	date("tglMasuk", "Tanggal Masuk", &in.TglMasuk)
	return errs
}

// TindakanItem is a procedure in the Tindakan Dokter lists.
type TindakanItem struct {
	ID    int    `json:"id"`
	Nama  string `json:"nama"`
	Jenis string `json:"jenis"`
}

// Option is an entry of a lookup table.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Options struct {
	Jenis     []Option `json:"jenis"`
	Gender    []Option `json:"gender"`
	Agama     []Option `json:"agama"`
	GolDarah  []Option `json:"golDarah"`
	StatusKwn []Option `json:"statusKwn"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	AllBranches(ctx context.Context) ([]branchauth.Branch, error)
	Options(ctx context.Context) (Options, error)
	List(ctx context.Context) ([]Dokter, error)
	Get(ctx context.Context, id int) (Dokter, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	// Tindakan returns the doctor's procedures and all the others.
	Tindakan(ctx context.Context, id int) (assigned, available []TindakanItem, err error)
	ChangeTindakan(ctx context.Context, id int, add, remove []int) error
	ChangeCabang(ctx context.Context, id int, add, remove []string) error
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

func (p *PostgresRepository) AllBranches(ctx context.Context) ([]branchauth.Branch, error) {
	rows, err := p.pool.Query(ctx, `SELECT kode, COALESCE(nama, '') FROM refbranch ORDER BY kode`)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (branchauth.Branch, error) {
		var b branchauth.Branch
		err := r.Scan(&b.Kode, &b.Nama)
		return b, err
	})
}

func (p *PostgresRepository) Options(ctx context.Context) (Options, error) {
	var o Options
	lists := []struct {
		dst   *[]Option
		query string
	}{
		{&o.Jenis, `SELECT id, COALESCE(info, '') FROM refjenisdokter ORDER BY id`},
		{&o.Gender, `SELECT id, COALESCE(info, '') FROM refgender ORDER BY id`},
		{&o.Agama, `SELECT id, COALESCE(info, '') FROM refagama ORDER BY id`},
		{&o.GolDarah, `SELECT id, COALESCE(info, '') FROM refgoldarah ORDER BY id`},
		{&o.StatusKwn, `SELECT id, COALESCE(info, '') FROM refstatuskawin ORDER BY id`},
	}
	for _, l := range lists {
		rows, err := p.pool.Query(ctx, l.query)
		if err != nil {
			return o, err
		}
		opts, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
			var op Option
			err := r.Scan(&op.ID, &op.Name)
			return op, err
		})
		if err != nil {
			return o, err
		}
		*l.dst = opts
	}
	return o, nil
}

const dokterColumns = `d.id, d.nama, d.id_jenisdok, to_char(d.tgl_masuk, 'YYYY-MM-DD'), COALESCE(d.tmpt_lahir, ''),
	to_char(d.tgl_lahir, 'YYYY-MM-DD'), COALESCE(d.alamat, ''), COALESCE(d.telp, ''), COALESCE(d.hp, ''),
	COALESCE(d.pendidikan, ''), COALESCE(d.spesialis, ''), d.id_gender, d.id_agama, d.id_goldarah, d.id_statuskwn,
	COALESCE(d.nik, ''), COALESCE(d.status, 1) = 1,
	COALESCE((SELECT array_agg(a.cabang ORDER BY a.cabang) FROM refmdokterauth a WHERE a.dokterid = d.id), '{}'),
	COALESCE(d.update_user, d.create_user, ''), COALESCE(d.update_date, d.create_date)`

func scanDokter(r pgx.CollectableRow) (Dokter, error) {
	var x Dokter
	err := r.Scan(&x.ID, &x.Nama, &x.IDJenis, &x.TglMasuk, &x.TmptLahir, &x.TglLahir, &x.Alamat, &x.Telp, &x.HP,
		&x.Pendidikan, &x.Spesialis, &x.IDGender, &x.IDAgama, &x.IDGolDarah, &x.IDStatusKwn, &x.NIK, &x.IsActive,
		&x.CabangPraktek, &x.UpdatedBy, &x.UpdatedAt)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context) ([]Dokter, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+dokterColumns+` FROM refmdokter d ORDER BY d.id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scanDokter)
	if list == nil {
		list = []Dokter{}
	}
	return list, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Dokter, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+dokterColumns+` FROM refmdokter d WHERE d.id = $1`, id)
	if err != nil {
		return Dokter{}, err
	}
	list, err := pgx.CollectRows(rows, scanDokter)
	if err != nil {
		return Dokter{}, err
	}
	if len(list) == 0 {
		return Dokter{}, ErrNotFound
	}
	return list[0], nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func status(active bool) int {
	if active {
		return 1
	}
	return 2
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refmdokter (nama, id_jenisdok, tgl_masuk, tmpt_lahir, tgl_lahir, alamat, telp, hp, pendidikan,
		                         spesialis, id_gender, id_agama, id_goldarah, id_statuskwn, nik, status,
		                         create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
		 RETURNING id`,
		in.Nama, in.IDJenis, in.TglMasuk, nullIfEmpty(in.TmptLahir), in.TglLahir, nullIfEmpty(in.Alamat),
		nullIfEmpty(in.Telp), nullIfEmpty(in.HP), nullIfEmpty(in.Pendidikan), nullIfEmpty(in.Spesialis),
		in.IDGender, in.IDAgama, in.IDGolDarah, in.IDStatusKwn, nullIfEmpty(in.NIK), status(in.IsActive), user,
	).Scan(&id)
	return id, translate(err)
}

// Update changes the editable columns; legacy fields (id_internal, cabang,
// oldid, ihs_*) are left as they are.
func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refmdokter SET nama = $2, id_jenisdok = $3, tgl_masuk = $4, tmpt_lahir = $5, tgl_lahir = $6,
		        alamat = $7, telp = $8, hp = $9, pendidikan = $10, spesialis = $11, id_gender = $12, id_agama = $13,
		        id_goldarah = $14, id_statuskwn = $15, nik = $16, status = $17, update_user = $18, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, in.IDJenis, in.TglMasuk, nullIfEmpty(in.TmptLahir), in.TglLahir, nullIfEmpty(in.Alamat),
		nullIfEmpty(in.Telp), nullIfEmpty(in.HP), nullIfEmpty(in.Pendidikan), nullIfEmpty(in.Spesialis),
		in.IDGender, in.IDAgama, in.IDGolDarah, in.IDStatusKwn, nullIfEmpty(in.NIK), status(in.IsActive), user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) Tindakan(ctx context.Context, id int) ([]TindakanItem, []TindakanItem, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT t.id, COALESCE(t.nama, ''), COALESCE(j.info, ''), x.id IS NOT NULL
		 FROM reftindakan t
		 LEFT JOIN refjenistindakan j ON j.id = t.id_jenis
		 LEFT JOIN trxtindakan x ON x.id_tindakan = t.id AND x.id_dokter = $1
		 ORDER BY t.id`, id)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	assigned, available := []TindakanItem{}, []TindakanItem{}
	for rows.Next() {
		var (
			it  TindakanItem
			has bool
		)
		if err := rows.Scan(&it.ID, &it.Nama, &it.Jenis, &has); err != nil {
			return nil, nil, err
		}
		if has {
			assigned = append(assigned, it)
		} else {
			available = append(available, it)
		}
	}
	return assigned, available, rows.Err()
}

func (p *PostgresRepository) ChangeTindakan(ctx context.Context, id int, add, remove []int) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if len(remove) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM trxtindakan WHERE id_dokter = $1 AND id_tindakan = ANY($2)`, id, remove); err != nil {
				return err
			}
		}
		if len(add) > 0 {
			_, err := tx.Exec(ctx,
				`INSERT INTO trxtindakan (id_dokter, id_tindakan) SELECT $1, unnest($2::int[]) ON CONFLICT DO NOTHING`, id, add)
			return err
		}
		return nil
	})
	return translate(err)
}

func (p *PostgresRepository) ChangeCabang(ctx context.Context, id int, add, remove []string) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if len(remove) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM refmdokterauth WHERE dokterid = $1 AND cabang = ANY($2)`, id, remove); err != nil {
				return err
			}
		}
		if len(add) > 0 {
			_, err := tx.Exec(ctx,
				`INSERT INTO refmdokterauth (dokterid, cabang) SELECT $1, unnest($2::text[]) ON CONFLICT DO NOTHING`, id, add)
			return err
		}
		return nil
	})
	return translate(err)
}

var refFields = map[string]string{
	"fk_refmdokter_jenis":      "idJenis",
	"fk_refmdokter_gender":     "idGender",
	"fk_refmdokter_agama":      "idAgama",
	"fk_refmdokter_goldarah":   "idGolDarah",
	"fk_refmdokter_statuskwn":  "idStatusKwn",
	"fk_trxtindakan_tindakan":  "tindakan",
	"fk_refmdokterauth_cabang": "cabang",
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	switch pgErr.ConstraintName {
	case "fk_trxtindakan_dokter", "fk_refmdokterauth_dokter":
		return ErrNotFound
	}
	if f, ok := refFields[pgErr.ConstraintName]; ok {
		return InvalidRefError{Field: f}
	}
	return err
}
