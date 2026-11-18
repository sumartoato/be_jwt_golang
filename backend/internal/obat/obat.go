// Package obat serves Master Data Klinik > Obat: the drug master (refobat) and
// stock/prices per branch (trxstock). A user only sees and edits the stock of
// the branches they are mapped to in refbranchauth.
package obat

import (
	"context"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

var ErrNotFound = errors.New("obat not found")

// InvalidRefError means a lookup id (jenis, satuan, ...) does not exist.
type InvalidRefError struct{ Field string }

func (e InvalidRefError) Error() string { return "unknown " + e.Field }

// Stock is one trxstock row: a drug's stock and prices at one branch.
type Stock struct {
	Cabang   string   `json:"cabang"`
	Stok     *float64 `json:"stok"`
	StokMin  *float64 `json:"stokMin"`
	HBeli    *float64 `json:"hbeli"`
	HP       *float64 `json:"hp"`
	HJual    *float64 `json:"hjual"`
	IsActive bool     `json:"isActive"`
}

// Obat is a refobat row plus its stock at the caller's branches.
// Lookup columns are ids; the client resolves names from GET /api/obat/options.
type Obat struct {
	ID            int        `json:"id"`
	Nama          string     `json:"nama"`
	Isi           string     `json:"isi"`
	IsiPerBox     string     `json:"isiPerBox"`
	IDJenis       *int       `json:"idJenis"`
	IDLogo        *int       `json:"idLogo"`
	IDProd        *int       `json:"idProd"`
	SatBeli       *int       `json:"satBeli"`
	SatJual       *int       `json:"satJual"`
	IDMoving      *int       `json:"idMoving"`
	IDMovingPo    *int       `json:"idMovingPo"`
	IDDistributor *int       `json:"idDistributor"`
	IDPrincipal   *int       `json:"idPrincipal"`
	IDGolongan    *int       `json:"idGolongan"`
	QJB           *float64   `json:"qjb"`
	HBeli         *float64   `json:"hbeli"`
	HJual         *float64   `json:"hjual"`
	HP            *float64   `json:"hp"`
	NonAktif      bool       `json:"nonAktif"`
	KenaPajak     bool       `json:"kenaPajak"`
	Komposisi     string     `json:"komposisi"`
	UpdatedBy     string     `json:"updatedBy"`
	UpdatedAt     *time.Time `json:"updatedAt"`
	Stocks        []Stock    `json:"stocks"`
}

// Input is the editable part of refobat.
type Input struct {
	Nama          string   `json:"nama"`
	Isi           string   `json:"isi"`
	IsiPerBox     string   `json:"isiPerBox"`
	IDJenis       *int     `json:"idJenis"`
	IDLogo        *int     `json:"idLogo"`
	IDProd        *int     `json:"idProd"`
	SatBeli       *int     `json:"satBeli"`
	SatJual       *int     `json:"satJual"`
	IDMoving      *int     `json:"idMoving"`
	IDMovingPo    *int     `json:"idMovingPo"`
	IDDistributor *int     `json:"idDistributor"`
	IDPrincipal   *int     `json:"idPrincipal"`
	IDGolongan    *int     `json:"idGolongan"`
	QJB           *float64 `json:"qjb"`
	HBeli         *float64 `json:"hbeli"`
	HJual         *float64 `json:"hjual"`
	HP            *float64 `json:"hp"`
	NonAktif      bool     `json:"nonAktif"`
	KenaPajak     bool     `json:"kenaPajak"`
	Komposisi     string   `json:"komposisi"`
}

// StockInput is the editable part of a trxstock row.
type StockInput struct {
	Stok     *float64 `json:"stok"`
	StokMin  *float64 `json:"stokMin"`
	HBeli    *float64 `json:"hbeli"`
	HP       *float64 `json:"hp"`
	HJual    *float64 `json:"hjual"`
	IsActive bool     `json:"isActive"`
}

func checkAmounts(errs httpx.FieldErrors, fields map[string]*float64) {
	for name, v := range fields {
		if v != nil && (math.IsNaN(*v) || math.IsInf(*v, 0) || *v < 0 || *v > 1e12) {
			errs[name] = "Must be a number from 0"
		}
	}
}

// Normalize trims text and validates against the refobat column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama, in.Isi, in.IsiPerBox = strings.TrimSpace(in.Nama), strings.TrimSpace(in.Isi), strings.TrimSpace(in.IsiPerBox)
	in.Komposisi = strings.TrimSpace(in.Komposisi)
	errs := httpx.FieldErrors{}
	maxLen := func(field, label, v string, n int) {
		if utf8.RuneCountInString(v) > n {
			errs[field] = label + " must be at most " + strconv.Itoa(n) + " characters"
		}
	}
	if in.Nama == "" {
		errs["nama"] = "Nama Obat is required"
	}
	maxLen("nama", "Nama Obat", in.Nama, 100)
	maxLen("isi", "Isi", in.Isi, 10)
	maxLen("isiPerBox", "Isi Per Box", in.IsiPerBox, 20)
	maxLen("komposisi", "Komposisi", in.Komposisi, 5000)
	checkAmounts(errs, map[string]*float64{"qjb": in.QJB, "hbeli": in.HBeli, "hjual": in.HJual, "hp": in.HP})
	return errs
}

func (in *StockInput) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	checkAmounts(errs, map[string]*float64{"stok": in.Stok, "stokMin": in.StokMin, "hbeli": in.HBeli, "hp": in.HP, "hjual": in.HJual})
	return errs
}

// Option is an entry of a lookup table.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Options struct {
	Jenis       []Option `json:"jenis"`
	Satuan      []Option `json:"satuan"`
	Logo        []Option `json:"logo"`
	Produsen    []Option `json:"produsen"`
	Moving      []Option `json:"moving"`
	MovingPo    []Option `json:"movingPo"`
	Distributor []Option `json:"distributor"`
	Principal   []Option `json:"principal"`
	Golongan    []Option `json:"golongan"`
}

// Branch is a branch the caller is mapped to.
type Branch struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

type Repository interface {
	Options(ctx context.Context) (Options, error)
	// UserBranches returns the branches a user is mapped to in refbranchauth.
	UserBranches(ctx context.Context, userID string) ([]Branch, error)
	// List returns every drug with its stock at the given branches only.
	List(ctx context.Context, branches []string) ([]Obat, error)
	Get(ctx context.Context, id int, branches []string) (Obat, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	UpsertStock(ctx context.Context, id int, cabang string, s StockInput) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) Options(ctx context.Context) (Options, error) {
	var o Options
	lists := []struct {
		dst   *[]Option
		query string
	}{
		{&o.Jenis, `SELECT id, COALESCE(info, '') FROM refjenisobat ORDER BY info, id`},
		{&o.Satuan, `SELECT id, COALESCE(info, '') FROM refsatuan ORDER BY info, id`},
		{&o.Logo, `SELECT id, COALESCE(info, '') FROM reflogoobat ORDER BY id`},
		{&o.Produsen, `SELECT id, COALESCE(nama, '') FROM refprodusen ORDER BY nama, id`},
		{&o.Moving, `SELECT id, COALESCE(kodemoving, '') FROM refkodemovingobat ORDER BY id`},
		{&o.MovingPo, `SELECT id, COALESCE(kodemovingpo, '') FROM refkodemovingpo ORDER BY id`},
		{&o.Distributor, `SELECT id, COALESCE(distributor, '') FROM refdistributorobat ORDER BY distributor, id`},
		{&o.Principal, `SELECT id, COALESCE(principal, '') FROM refprincipalobat ORDER BY principal, id`},
		{&o.Golongan, `SELECT id, COALESCE(golonganobat, '') FROM refgolonganobat ORDER BY golonganobat, id`},
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

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]Branch, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT b.kode, COALESCE(b.nama, '')
		 FROM refbranchauth a JOIN refbranch b ON b.kode = a.branch
		 WHERE a.userid = $1
		 ORDER BY b.kode`, userID)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, func(r pgx.CollectableRow) (Branch, error) {
		var b Branch
		err := r.Scan(&b.Kode, &b.Nama)
		return b, err
	})
}

const obatColumns = `id, nama, COALESCE(isi, ''), COALESCE(isi_per_box, ''), id_jenis, id_logo, id_prod,
	sat_beli, sat_jual, id_moving, id_moving_po, id_distributor, id_principal, id_golongan_obat,
	qjb, hbeli, hjual, hp, COALESCE(non_aktif, 0) = 1, kena_pajak = 1, COALESCE(komposisi, ''),
	COALESCE(update_user, create_user, ''), COALESCE(update_date, create_date)`

func scanObat(r pgx.CollectableRow) (Obat, error) {
	var o Obat
	err := r.Scan(&o.ID, &o.Nama, &o.Isi, &o.IsiPerBox, &o.IDJenis, &o.IDLogo, &o.IDProd,
		&o.SatBeli, &o.SatJual, &o.IDMoving, &o.IDMovingPo, &o.IDDistributor, &o.IDPrincipal, &o.IDGolongan,
		&o.QJB, &o.HBeli, &o.HJual, &o.HP, &o.NonAktif, &o.KenaPajak, &o.Komposisi, &o.UpdatedBy, &o.UpdatedAt)
	o.Stocks = []Stock{}
	return o, err
}

// attachStocks fills Stocks for the drugs in list from trxstock at branches.
func (p *PostgresRepository) attachStocks(ctx context.Context, list []Obat, branches []string, only *int) error {
	if len(list) == 0 || len(branches) == 0 {
		return nil
	}
	index := make(map[int]int, len(list))
	for i, o := range list {
		index[o.ID] = i
	}
	rows, err := p.pool.Query(ctx,
		`SELECT id_obat, cabang, stok, stok_min, hbeli, hp, hjual, isactive = 1
		 FROM trxstock
		 WHERE cabang = ANY($1) AND ($2::int IS NULL OR id_obat = $2)
		 ORDER BY cabang`, branches, only)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			id int
			s  Stock
		)
		if err := rows.Scan(&id, &s.Cabang, &s.Stok, &s.StokMin, &s.HBeli, &s.HP, &s.HJual, &s.IsActive); err != nil {
			return err
		}
		if i, ok := index[id]; ok {
			list[i].Stocks = append(list[i].Stocks, s)
		}
	}
	return rows.Err()
}

func (p *PostgresRepository) List(ctx context.Context, branches []string) ([]Obat, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+obatColumns+` FROM refobat ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scanObat)
	if err != nil {
		return nil, err
	}
	if list == nil {
		list = []Obat{}
	}
	return list, p.attachStocks(ctx, list, branches, nil)
}

func (p *PostgresRepository) Get(ctx context.Context, id int, branches []string) (Obat, error) {
	rows, err := p.pool.Query(ctx, `SELECT `+obatColumns+` FROM refobat WHERE id = $1`, id)
	if err != nil {
		return Obat{}, err
	}
	list, err := pgx.CollectRows(rows, scanObat)
	if err != nil {
		return Obat{}, err
	}
	if len(list) == 0 {
		return Obat{}, ErrNotFound
	}
	return list[0], p.attachStocks(ctx, list, branches, &id)
}

func flag(v bool) int {
	if v {
		return 1
	}
	return 0
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refobat (nama, isi, isi_per_box, id_jenis, id_logo, id_prod, sat_beli, sat_jual, id_moving,
		                      id_moving_po, id_distributor, id_principal, id_golongan_obat, qjb, hbeli, hjual, hp,
		                      non_aktif, kena_pajak, komposisi, create_user, create_date)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, now())
		 RETURNING id`,
		in.Nama, nullIfEmpty(in.Isi), nullIfEmpty(in.IsiPerBox), in.IDJenis, in.IDLogo, in.IDProd, in.SatBeli, in.SatJual,
		in.IDMoving, in.IDMovingPo, in.IDDistributor, in.IDPrincipal, in.IDGolongan, in.QJB, in.HBeli, in.HJual, in.HP,
		flag(in.NonAktif), flag(in.KenaPajak), nullIfEmpty(in.Komposisi), user,
	).Scan(&id)
	return id, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refobat SET nama = $2, isi = $3, isi_per_box = $4, id_jenis = $5, id_logo = $6, id_prod = $7,
		        sat_beli = $8, sat_jual = $9, id_moving = $10, id_moving_po = $11, id_distributor = $12,
		        id_principal = $13, id_golongan_obat = $14, qjb = $15, hbeli = $16, hjual = $17, hp = $18,
		        non_aktif = $19, kena_pajak = $20, komposisi = $21, update_user = $22, update_date = now()
		 WHERE id = $1`,
		id, in.Nama, nullIfEmpty(in.Isi), nullIfEmpty(in.IsiPerBox), in.IDJenis, in.IDLogo, in.IDProd, in.SatBeli, in.SatJual,
		in.IDMoving, in.IDMovingPo, in.IDDistributor, in.IDPrincipal, in.IDGolongan, in.QJB, in.HBeli, in.HJual, in.HP,
		flag(in.NonAktif), flag(in.KenaPajak), nullIfEmpty(in.Komposisi), user)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (p *PostgresRepository) UpsertStock(ctx context.Context, id int, cabang string, s StockInput) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO trxstock (id_obat, cabang, stok, stok_min, hbeli, hp, hjual, isactive)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 ON CONFLICT (id_obat, cabang) DO UPDATE SET
		     stok = EXCLUDED.stok, stok_min = EXCLUDED.stok_min, hbeli = EXCLUDED.hbeli,
		     hp = EXCLUDED.hp, hjual = EXCLUDED.hjual, isactive = EXCLUDED.isactive`,
		id, cabang, s.Stok, s.StokMin, s.HBeli, s.HP, s.HJual, flag(s.IsActive))
	return translate(err)
}

// refFields maps foreign keys to the request field they come from.
var refFields = map[string]string{
	"fk_refobat_jenis":       "idJenis",
	"fk_refobat_logo":        "idLogo",
	"fk_refobat_sat_beli":    "satBeli",
	"fk_refobat_sat_jual":    "satJual",
	"fk_refobat_moving":      "idMoving",
	"fk_refobat_moving_po":   "idMovingPo",
	"fk_refobat_distributor": "idDistributor",
	"fk_refobat_principal":   "idPrincipal",
	"fk_refobat_golongan":    "idGolongan",
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		return err
	}
	if pgErr.ConstraintName == "fk_trxstock_obat" {
		return ErrNotFound
	}
	if f, ok := refFields[pgErr.ConstraintName]; ok {
		return InvalidRefError{Field: f}
	}
	return err
}
