import { FormEvent, ReactNode, RefObject, useEffect, useMemo, useRef, useState } from "react";
import { DollarSign, Pencil, Plus, X } from "lucide-react";
import { api, ApiError, FieldErrors, LookupOption, Obat, ObatInput, ObatOptions, ObatStock } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";

type BranchRef = { kode: string; nama: string };
const ALL = "__all__";
const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const fmt = (v: number | null | undefined) => (v === null || v === undefined ? "-" : num.format(v));
const orDash = (s: string | undefined) => (s ? s : "-");

const EMPTY_OBAT: ObatInput = {
  nama: "",
  isi: "",
  isiPerBox: "",
  idJenis: null,
  idLogo: null,
  idProd: null,
  satBeli: null,
  satJual: null,
  idMoving: null,
  idMovingPo: null,
  idDistributor: null,
  idPrincipal: null,
  idGolongan: null,
  qjb: null,
  hbeli: null,
  hjual: null,
  hp: null,
  nonAktif: false,
  kenaPajak: false,
  komposisi: "",
};

function toInput(o: Obat): ObatInput {
  const { id: _id, updatedBy: _by, updatedAt: _at, stocks: _stocks, ...rest } = o;
  return rest;
}

/** Master Data Klinik > Obat: drug master plus stock/prices at the user's branches. */
export default function ObatPage() {
  const { token, logout } = useAuth();
  const [obat, setObat] = useState<Obat[] | null>(null);
  const [branches, setBranches] = useState<BranchRef[]>([]);
  const [options, setOptions] = useState<ObatOptions | null>(null);
  const [branch, setBranch] = useState(ALL);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Obat | "new" | null>(null);
  const [stockOf, setStockOf] = useState<Obat | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.obat(token), api.obatOptions(token)])
      .then(([list, opts]) => {
        setObat(list.obat);
        setBranches(list.branches);
        setOptions(opts);
        // One branch: show it directly. Several: show all, like the original screen.
        setBranch(list.branches.length === 1 ? list.branches[0].kode : ALL);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load obat");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  // id -> name for every lookup list.
  const names = useMemo(() => {
    const toMap = (l: LookupOption[] = []) => new Map(l.map((o) => [o.id, o.name]));
    return {
      jenis: toMap(options?.jenis),
      satuan: toMap(options?.satuan),
      logo: toMap(options?.logo),
      produsen: toMap(options?.produsen),
      moving: toMap(options?.moving),
      movingPo: toMap(options?.movingPo),
      distributor: toMap(options?.distributor),
      principal: toMap(options?.principal),
      golongan: toMap(options?.golongan),
    };
  }, [options]);
  const nameOf = (m: Map<number, string>, id: number | null) => (id === null ? "" : (m.get(id) ?? ""));

  const replace = (o: Obat, isNew: boolean) =>
    setObat((list) => (isNew ? [...(list ?? []), o] : (list ?? []).map((x) => (x.id === o.id ? o : x))));

  /** Stock rows to show: the selected branch, or every branch of the user. */
  const shownStocks = (o: Obat): ObatStock[] =>
    branch === ALL ? o.stocks : o.stocks.filter((s) => s.cabang === branch);

  // A per-branch column: one value for a single branch, one line per branch for "all".
  const perBranch = (
    key: string,
    header: string,
    pick: (s: ObatStock) => string,
    sortPick?: (s: ObatStock) => number | null,
    hidden = false,
  ): Column<Obat> => ({
    key,
    header,
    hidden,
    value: (o) => {
      const rows = shownStocks(o);
      if (branch !== ALL) return rows[0] ? pick(rows[0]) : "-";
      return rows.map((s) => `${s.cabang}:${pick(s)}`).join("; ");
    },
    render: (o) => {
      const rows = shownStocks(o);
      if (branch !== ALL) return rows[0] ? pick(rows[0]) : <span className="muted">-</span>;
      if (rows.length === 0) return <span className="muted">-</span>;
      return (
        <ul className="branch-vals">
          {rows.map((s) => (
            <li key={s.cabang}>
              <span className="branch-vals-kode">{s.cabang}</span> {pick(s)}
            </li>
          ))}
        </ul>
      );
    },
    // Sorting only makes sense for one branch; with all branches it sorts by the first.
    sortValue: sortPick ? (o) => (shownStocks(o)[0] ? sortPick(shownStocks(o)[0]) : null) : undefined,
  });

  const columns: Column<Obat>[] = [
    { key: "id", header: "Kode", value: (o) => String(o.id), sortValue: (o) => o.id },
    { key: "nama", header: "Nama Obat", value: (o) => o.nama, className: "dt-wrap" },
    { key: "jenis", header: "Jenis", value: (o) => nameOf(names.jenis, o.idJenis) },
    { key: "satBeli", header: "Satuan Beli", value: (o) => nameOf(names.satuan, o.satBeli) },
    { key: "isiPerBox", header: "Isi Per Box", value: (o) => orDash(o.isiPerBox) },
    { key: "satJual", header: "Satuan Jual", value: (o) => nameOf(names.satuan, o.satJual) },
    perBranch("hbeli", "Harga Beli", (s) => fmt(s.hbeli), (s) => s.hbeli),
    perBranch("hjual", "Harga Jual", (s) => fmt(s.hjual), (s) => s.hjual),
    perBranch("status", "Status", (s) => (s.isActive ? "Active" : "Non Active")),
    { key: "movingPo", header: "Kode Moving PO", value: (o) => orDash(nameOf(names.movingPo, o.idMovingPo)) },
    perBranch("stok", "Stok Obat", (s) => fmt(s.stok), (s) => s.stok),
    perBranch("hp", "Harga Pokok", (s) => fmt(s.hp), (s) => s.hp, true),
    perBranch("stokMin", "Stok Min", (s) => fmt(s.stokMin), (s) => s.stokMin, true),
    {
      key: "komposisi",
      header: "Komposisi",
      value: (o) => orDash(o.komposisi),
      render: (o) =>
        o.komposisi ? (
          <span className="dt-clip" title={o.komposisi}>
            {o.komposisi}
          </span>
        ) : (
          "-"
        ),
    },
    { key: "principal", header: "Principle", value: (o) => orDash(nameOf(names.principal, o.idPrincipal)) },
    { key: "distributor", header: "Distributor", value: (o) => orDash(nameOf(names.distributor, o.idDistributor)) },
    { key: "logo", header: "Logo", value: (o) => orDash(nameOf(names.logo, o.idLogo)), hidden: true },
    { key: "golongan", header: "Golongan", value: (o) => orDash(nameOf(names.golongan, o.idGolongan)), hidden: true },
    { key: "moving", header: "Kode Moving", value: (o) => orDash(nameOf(names.moving, o.idMoving)), hidden: true },
    { key: "kenaPajak", header: "Kena Pajak", value: (o) => (o.kenaPajak ? "Ya" : "Tidak"), hidden: true },
    { key: "nonAktif", header: "Status Obat", value: (o) => (o.nonAktif ? "Non Aktif" : "Aktif"), hidden: true },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (o) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${o.nama}`} title="Edit" onClick={() => setEditing(o)}>
            <Pencil aria-hidden="true" />
          </button>
          <button
            type="button"
            className="row-action"
            aria-label={`Stock and prices of ${o.nama}`}
            title={branches.length ? "Stock and prices per branch" : "You are not assigned to any branch"}
            disabled={branches.length === 0}
            onClick={() => setStockOf(o)}
          >
            <DollarSign aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  const branchFilter =
    branches.length > 1 ? (
      <label className="dt-length">
        <span>Cabang</span>
        <select className="select dt-branch" aria-label="Cabang" value={branch} onChange={(e) => setBranch(e.target.value)}>
          <option value={ALL}>Semua cabang ({branches.length})</option>
          {branches.map((b) => (
            <option key={b.kode} value={b.kode}>
              {b.kode} · {b.nama}
            </option>
          ))}
        </select>
      </label>
    ) : branches.length === 1 ? (
      <span className="dt-length">
        Cabang <span className="chip">{branches[0].kode}</span>
      </span>
    ) : null;

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Obat</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!options}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {saved && (
        <p className="alert success" role="status">
          {saved}
        </p>
      )}
      {obat && branches.length === 0 && (
        <p className="alert info" role="status">
          You are not assigned to any branch, so stock and prices per branch are hidden. An admin can assign branches
          under Management › User.
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !obat || !options ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable
          title="Obat"
          columns={columns}
          rows={obat}
          rowKey={(o) => o.id}
          initialSort={{ key: "id", dir: "asc" }}
          defaultPageSize={10}
          toolbarStart={branchFilter}
        />
      )}
      {editing && options && (
        <ObatForm
          obat={editing === "new" ? null : editing}
          options={options}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(o, isNew) => {
            replace(o, isNew);
            setSaved(`Obat ${o.nama} ${isNew ? "added" : "updated"}.`);
            setEditing(null);
          }}
        />
      )}
      {stockOf && (
        <StockForm
          obat={stockOf}
          branches={branches}
          initialBranch={branch === ALL ? branches[0]?.kode : branch}
          onClose={() => setStockOf(null)}
          onUnauthorized={expired}
          onSaved={(o, cabang) => {
            replace(o, false);
            setSaved(`Stock of ${o.nama} at ${cabang} saved.`);
            setStockOf(null);
          }}
        />
      )}
    </section>
  );
}

// ---------- shared dialog bits ----------

function useModal() {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  return ref;
}

function Modal({
  dialogRef,
  title,
  wide,
  onClose,
  onSubmit,
  error,
  busy,
  children,
}: {
  dialogRef: RefObject<HTMLDialogElement>;
  title: string;
  wide?: boolean;
  onClose: () => void;
  onSubmit: (e: FormEvent) => void;
  error: string;
  busy: boolean;
  children: ReactNode;
}) {
  return (
    <dialog ref={dialogRef} className={wide ? "modal modal-wide" : "modal"} aria-labelledby="obat-dialog-title" onClose={onClose}>
      <form onSubmit={onSubmit} noValidate>
        <header className="modal-head">
          <h2 id="obat-dialog-title">{title}</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">{children}</div>
        {error && (
          <p className="alert error modal-alert" role="alert">
            {error}
          </p>
        )}
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-small btn-primary" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}

const numText = (v: number | null) => (v === null ? "" : String(v));
const textNum = (s: string) => (s.trim() === "" ? null : Number(s));

function FieldShell({ id, label, error, wide, required, children }: { id: string; label: string; error?: string; wide?: boolean; required?: boolean; children: ReactNode }) {
  return (
    <div className={wide ? "field wide" : "field"}>
      <label htmlFor={id}>
        {label}
        {required && <span aria-hidden="true"> *</span>}
      </label>
      {children}
      {error && (
        <span id={`${id}-error`} className="field-error">
          {error}
        </span>
      )}
    </div>
  );
}

// ---------- drug master form ----------

type LookupKey = "idJenis" | "idLogo" | "idProd" | "satBeli" | "satJual" | "idMoving" | "idMovingPo" | "idDistributor" | "idPrincipal" | "idGolongan";
type NumKey = "qjb" | "hbeli" | "hjual" | "hp";

function ObatForm({
  obat,
  options,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  obat: Obat | null;
  options: ObatOptions;
  onClose: () => void;
  onSaved: (o: Obat, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<ObatInput>(() => (obat ? toInput(obat) : EMPTY_OBAT));
  // Number fields are edited as text so "" means empty (NULL).
  const [nums, setNums] = useState<Record<NumKey, string>>(() => {
    const src = obat ?? EMPTY_OBAT;
    return { qjb: numText(src.qjb), hbeli: numText(src.hbeli), hjual: numText(src.hjual), hp: numText(src.hp) };
  });
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const clearError = (k: string) => setErrors(({ [k]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!form.nama.trim()) {
      setErrors({ nama: "Nama Obat is required" });
      return;
    }
    const body: ObatInput = { ...form };
    for (const k of Object.keys(nums) as NumKey[]) {
      const v = textNum(nums[k]);
      if (v !== null && (Number.isNaN(v) || v < 0)) {
        setErrors({ [k]: "Must be a number from 0" });
        return;
      }
      body[k] = v;
    }
    setBusy(true);
    setError("");
    try {
      const saved = obat ? await api.updateObat(token, obat.id, body) : await api.createObat(token, body);
      onSaved(saved, !obat);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const text = (key: "nama" | "isi" | "isiPerBox", label: string, max: number, required = false) => (
    <FieldShell id={`obat-${key}`} label={label} error={errors[key]} required={required} wide={key === "nama"}>
      <input
        id={`obat-${key}`}
        value={form[key]}
        maxLength={max}
        aria-invalid={errors[key] ? "true" : undefined}
        aria-describedby={errors[key] ? `obat-${key}-error` : undefined}
        onChange={(e) => {
          setForm((f) => ({ ...f, [key]: e.target.value }));
          clearError(key);
        }}
      />
    </FieldShell>
  );

  const lookup = (key: LookupKey, label: string, list: LookupOption[]) => {
    const value = form[key];
    // Keep ids that are no longer in the lookup table (lab has such producers).
    const missing = value !== null && !list.some((o) => o.id === value);
    return (
      <FieldShell id={`obat-${key}`} label={label} error={errors[key]}>
        <select
          id={`obat-${key}`}
          className="select"
          value={value ?? ""}
          aria-invalid={errors[key] ? "true" : undefined}
          onChange={(e) => {
            setForm((f) => ({ ...f, [key]: e.target.value === "" ? null : Number(e.target.value) }));
            clearError(key);
          }}
        >
          <option value="">-</option>
          {missing && <option value={value!}>#{value} (not in the list)</option>}
          {list.map((o) => (
            <option key={o.id} value={o.id}>
              {o.name}
            </option>
          ))}
        </select>
      </FieldShell>
    );
  };

  const number = (key: NumKey, label: string) => (
    <FieldShell id={`obat-${key}`} label={label} error={errors[key]}>
      <input
        id={`obat-${key}`}
        type="number"
        inputMode="decimal"
        min={0}
        step="any"
        value={nums[key]}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => {
          setNums((n) => ({ ...n, [key]: e.target.value }));
          clearError(key);
        }}
      />
    </FieldShell>
  );

  const yesNo = (key: "kenaPajak" | "nonAktif", label: string, yes: string, no: string) => (
    <FieldShell id={`obat-${key}`} label={label}>
      <select
        id={`obat-${key}`}
        className="select"
        value={form[key] ? "1" : "0"}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value === "1" }))}
      >
        <option value="0">{no}</option>
        <option value="1">{yes}</option>
      </select>
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={obat ? `Edit Obat #${obat.id}` : "Add Obat"} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <fieldset className="form-section">
        <legend>Data Obat</legend>
        <div className="form-grid">
          {text("nama", "Nama Obat", 100, true)}
          {lookup("idJenis", "Jenis", options.jenis)}
          {lookup("idGolongan", "Golongan", options.golongan)}
          {lookup("satBeli", "Satuan Beli", options.satuan)}
          {lookup("satJual", "Satuan Jual", options.satuan)}
          {text("isi", "Isi", 10)}
          {text("isiPerBox", "Isi Per Box", 20)}
          {number("qjb", "QJB (konversi satuan)")}
          {lookup("idLogo", "Logo", options.logo)}
          {lookup("idProd", "Produsen", options.produsen)}
          {yesNo("kenaPajak", "Kena Pajak", "Ya", "Tidak")}
          {yesNo("nonAktif", "Status Obat", "Non Aktif", "Aktif")}
        </div>
      </fieldset>
      <fieldset className="form-section">
        <legend>Harga Default</legend>
        <p className="muted form-note">Prices per branch are set with the $ button.</p>
        <div className="form-grid form-grid-3">
          {number("hbeli", "Harga Beli")}
          {number("hp", "Harga Pokok")}
          {number("hjual", "Harga Jual")}
        </div>
      </fieldset>
      <fieldset className="form-section">
        <legend>Distribusi</legend>
        <div className="form-grid">
          {lookup("idDistributor", "Distributor", options.distributor)}
          {lookup("idPrincipal", "Principle", options.principal)}
          {lookup("idMoving", "Kode Moving", options.moving)}
          {lookup("idMovingPo", "Kode Moving PO", options.movingPo)}
          <FieldShell id="obat-komposisi" label="Komposisi" error={errors.komposisi} wide>
            <textarea
              id="obat-komposisi"
              rows={3}
              maxLength={5000}
              value={form.komposisi}
              onChange={(e) => setForm((f) => ({ ...f, komposisi: e.target.value }))}
            />
          </FieldShell>
        </div>
      </fieldset>
    </Modal>
  );
}

// ---------- stock and prices per branch ----------

type StockKey = "stok" | "stokMin" | "hbeli" | "hp" | "hjual";

function StockForm({
  obat,
  branches,
  initialBranch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  obat: Obat;
  branches: BranchRef[];
  initialBranch?: string;
  onClose: () => void;
  onSaved: (o: Obat, cabang: string) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [cabang, setCabang] = useState(initialBranch ?? branches[0]?.kode ?? "");
  const current = obat.stocks.find((s) => s.cabang === cabang);
  const fromStock = (s?: ObatStock): Record<StockKey, string> => ({
    stok: numText(s?.stok ?? null),
    stokMin: numText(s?.stokMin ?? null),
    hbeli: numText(s?.hbeli ?? obat.hbeli),
    hp: numText(s?.hp ?? obat.hp),
    hjual: numText(s?.hjual ?? obat.hjual),
  });
  const [vals, setVals] = useState(() => fromStock(current));
  const [active, setActive] = useState(current?.isActive ?? true);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // Switching branch loads that branch's values (or the drug's default prices).
  const pick = (kode: string) => {
    setCabang(kode);
    const s = obat.stocks.find((x) => x.cabang === kode);
    setVals(fromStock(s));
    setActive(s?.isActive ?? true);
    setErrors({});
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token || !cabang) return;
    const body: Record<StockKey, number | null> = { stok: null, stokMin: null, hbeli: null, hp: null, hjual: null };
    for (const k of Object.keys(vals) as StockKey[]) {
      const v = textNum(vals[k]);
      if (v !== null && (Number.isNaN(v) || v < 0)) {
        setErrors({ [k]: "Must be a number from 0" });
        return;
      }
      body[k] = v;
    }
    setBusy(true);
    setError("");
    try {
      const saved = await api.setObatStock(token, obat.id, cabang, { ...body, isActive: active });
      onSaved(saved, cabang);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const number = (key: StockKey, label: string) => (
    <FieldShell id={`stock-${key}`} label={label} error={errors[key]}>
      <input
        id={`stock-${key}`}
        type="number"
        inputMode="decimal"
        min={0}
        step="any"
        value={vals[key]}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => {
          setVals((v) => ({ ...v, [key]: e.target.value }));
          setErrors(({ [key]: _, ...rest }) => rest);
        }}
      />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={`Stok & Harga · ${obat.nama}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="stock-cabang" label="Cabang" wide>
          <select id="stock-cabang" className="select" value={cabang} onChange={(e) => pick(e.target.value)}>
            {branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.kode} · {b.nama}
              </option>
            ))}
          </select>
          {!current && (
            <span className="field-hint">No stock row for {cabang} yet; saving creates it (prices start from the drug's defaults).</span>
          )}
        </FieldShell>
        {number("stok", "Stok")}
        {number("stokMin", "Stok Min")}
        {number("hbeli", "Harga Beli")}
        {number("hp", "Harga Pokok")}
        {number("hjual", "Harga Jual")}
        <FieldShell id="stock-active" label="Status">
          <select id="stock-active" className="select" value={active ? "1" : "0"} onChange={(e) => setActive(e.target.value === "1")}>
            <option value="1">Active</option>
            <option value="0">Non Active</option>
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
