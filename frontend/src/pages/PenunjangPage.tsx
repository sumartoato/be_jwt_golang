import { FormEvent, useEffect, useMemo, useState } from "react";
import { DollarSign, Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, LookupOption, Penunjang, PenunjangInput, PenunjangPrice } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import {
  ALL_BRANCHES,
  BranchFilter,
  BranchRef,
  fmt,
  initialBranch,
  NoBranchNotice,
  perBranchColumn,
} from "../components/branchView";
import { AmountInput, FieldShell, Modal, numText, parseAmounts, useModal } from "../components/FormKit";

const EMPTY: PenunjangInput = { nama: "", idJenis: null, hjual: null, hjualJamkes: null, isActive: true };

const status = (on: boolean) => (on ? "Aktif" : "Non Aktif");

type Amount = "hjual" | "hjualJamkes";

function saveError(err: unknown, onUnauthorized: () => void, setErrors: (e: FieldErrors) => void, setError: (s: string) => void) {
  if (err instanceof ApiError && err.status === 401) onUnauthorized();
  else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
  else setError(err instanceof Error ? err.message : "Failed to save");
}

/** Master Data Klinik > Penunjang: supporting examinations plus prices at the user's branches. */
export default function PenunjangPage() {
  const { token, logout } = useAuth();
  const [items, setItems] = useState<Penunjang[] | null>(null);
  const [branches, setBranches] = useState<BranchRef[]>([]);
  const [jenis, setJenis] = useState<LookupOption[] | null>(null);
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Penunjang | "new" | null>(null);
  const [priceOf, setPriceOf] = useState<Penunjang | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.penunjang(token), api.penunjangOptions(token)])
      .then(([list, opts]) => {
        setItems(list.penunjang);
        setBranches(list.branches);
        setJenis(opts.jenis);
        setBranch(initialBranch(list.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load penunjang");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const jenisName = useMemo(() => new Map((jenis ?? []).map((j) => [j.id, j.name])), [jenis]);

  const replace = (x: Penunjang, isNew: boolean) =>
    setItems((list) => (isNew ? [...(list ?? []), x] : (list ?? []).map((y) => (y.id === x.id ? x : y))));

  const perBranch = (
    key: string,
    header: string,
    pick: (p: PenunjangPrice) => string,
    sortPick?: (p: PenunjangPrice) => number | null,
    hidden = false,
  ) => perBranchColumn<Penunjang, PenunjangPrice>({ key, header, rowsOf: (x) => x.prices, branch, pick, sortPick, hidden });

  const columns: Column<Penunjang>[] = [
    { key: "id", header: "Kode", value: (x) => String(x.id), sortValue: (x) => x.id },
    { key: "nama", header: "Nama Penunjang", value: (x) => x.nama, className: "dt-wrap" },
    { key: "jenis", header: "Jenis Penunjang", value: (x) => (x.idJenis === null ? "" : (jenisName.get(x.idJenis) ?? `#${x.idJenis}`)) },
    perBranch("hjual", "Hrg Jual", (p) => fmt(p.hjual), (p) => p.hjual),
    perBranch("hjualJamkes", "Hrg Jual Jamkes", (p) => fmt(p.hjualJamkes), (p) => p.hjualJamkes),
    perBranch("branchStatus", "Status Cabang", (p) => status(p.isActive), undefined, true),
    { key: "status", header: "Status", value: (x) => status(x.isActive) },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (x) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${x.nama}`} title="Edit" onClick={() => setEditing(x)}>
            <Pencil aria-hidden="true" />
          </button>
          <button
            type="button"
            className="row-action"
            aria-label={`Prices of ${x.nama}`}
            title={branches.length ? "Prices per branch" : "You are not assigned to any branch"}
            disabled={branches.length === 0}
            onClick={() => setPriceOf(x)}
          >
            <DollarSign aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Penunjang</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!jenis}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {saved && (
        <p className="alert success" role="status">
          {saved}
        </p>
      )}
      {items && branches.length === 0 && <NoBranchNotice what="prices" />}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !items || !jenis ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable
          title="Penunjang"
          columns={columns}
          rows={items}
          rowKey={(x) => x.id}
          initialSort={{ key: "id", dir: "asc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
        />
      )}
      {editing && jenis && (
        <PenunjangForm
          penunjang={editing === "new" ? null : editing}
          jenis={jenis}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(x, isNew) => {
            replace(x, isNew);
            setSaved(`Penunjang ${x.nama} ${isNew ? "added" : "updated"}.`);
            setEditing(null);
          }}
        />
      )}
      {priceOf && (
        <PriceForm
          penunjang={priceOf}
          branches={branches}
          initialBranch={branch === ALL_BRANCHES ? branches[0]?.kode : branch}
          onClose={() => setPriceOf(null)}
          onUnauthorized={expired}
          onSaved={(x, cabang) => {
            replace(x, false);
            setSaved(`Prices of ${x.nama} at ${cabang} saved.`);
            setPriceOf(null);
          }}
        />
      )}
    </section>
  );
}

// ---------- examination master form ----------

function PenunjangForm({
  penunjang,
  jenis,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  penunjang: Penunjang | null;
  jenis: LookupOption[];
  onClose: () => void;
  onSaved: (x: Penunjang, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const src = penunjang ?? EMPTY;
  const [form, setForm] = useState<PenunjangInput>(() => ({
    nama: src.nama,
    idJenis: src.idJenis,
    hjual: src.hjual,
    hjualJamkes: src.hjualJamkes,
    isActive: src.isActive,
  }));
  const [amounts, setAmounts] = useState<Record<Amount, string>>(() => ({
    hjual: numText(src.hjual),
    hjualJamkes: numText(src.hjualJamkes),
  }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!form.nama.trim()) {
      setErrors({ nama: "Nama Penunjang is required" });
      return;
    }
    const parsed = parseAmounts(amounts);
    if ("bad" in parsed) {
      setErrors({ [parsed.bad]: "Must be a number from 0" });
      return;
    }
    setBusy(true);
    setError("");
    try {
      const body = { ...form, ...parsed.ok };
      const saved = penunjang ? await api.updatePenunjang(token, penunjang.id, body) : await api.createPenunjang(token, body);
      onSaved(saved, !penunjang);
    } catch (err) {
      saveError(err, onUnauthorized, setErrors, setError);
    } finally {
      setBusy(false);
    }
  }

  const amount = (key: Amount, label: string) => (
    <FieldShell id={`penunjang-${key}`} label={label} error={errors[key]}>
      <AmountInput
        id={`penunjang-${key}`}
        value={amounts[key]}
        invalid={!!errors[key]}
        onChange={(v) => {
          setAmounts((a) => ({ ...a, [key]: v }));
          setErrors(({ [key]: _, ...rest }) => rest);
        }}
      />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={penunjang ? `Edit Penunjang #${penunjang.id}` : "Add Penunjang"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="penunjang-nama" label="Nama Penunjang" error={errors.nama} wide required>
          <input
            id="penunjang-nama"
            value={form.nama}
            maxLength={100}
            aria-invalid={errors.nama ? "true" : undefined}
            onChange={(e) => {
              setForm((f) => ({ ...f, nama: e.target.value }));
              setErrors(({ nama: _, ...rest }) => rest);
            }}
          />
        </FieldShell>
        <FieldShell id="penunjang-idJenis" label="Jenis Penunjang" error={errors.idJenis}>
          <select
            id="penunjang-idJenis"
            className="select"
            value={form.idJenis ?? ""}
            onChange={(e) => {
              setForm((f) => ({ ...f, idJenis: e.target.value === "" ? null : Number(e.target.value) }));
              setErrors(({ idJenis: _, ...rest }) => rest);
            }}
          >
            <option value="">-</option>
            {jenis.map((j) => (
              <option key={j.id} value={j.id}>
                {j.name}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="penunjang-isActive" label="Status">
          <select
            id="penunjang-isActive"
            className="select"
            value={form.isActive ? "1" : "0"}
            onChange={(e) => setForm((f) => ({ ...f, isActive: e.target.value === "1" }))}
          >
            <option value="1">Aktif</option>
            <option value="0">Non Aktif</option>
          </select>
        </FieldShell>
        {amount("hjual", "Hrg Jual (default)")}
        {amount("hjualJamkes", "Hrg Jual Jamkes (default)")}
        <p className="muted form-note wide-note">Prices per branch are set with the $ button.</p>
      </div>
    </Modal>
  );
}

// ---------- prices per branch ----------

function PriceForm({
  penunjang,
  branches,
  initialBranch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  penunjang: Penunjang;
  branches: BranchRef[];
  initialBranch?: string;
  onClose: () => void;
  onSaved: (x: Penunjang, cabang: string) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [cabang, setCabang] = useState(initialBranch ?? branches[0]?.kode ?? "");
  const current = penunjang.prices.find((p) => p.cabang === cabang);
  // A branch without a price row starts from the examination's default prices.
  const fromPrice = (p?: PenunjangPrice): Record<Amount, string> => ({
    hjual: numText(p ? p.hjual : penunjang.hjual),
    hjualJamkes: numText(p ? p.hjualJamkes : penunjang.hjualJamkes),
  });
  const [vals, setVals] = useState(() => fromPrice(current));
  const [active, setActive] = useState(current?.isActive ?? true);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const pick = (kode: string) => {
    setCabang(kode);
    const p = penunjang.prices.find((x) => x.cabang === kode);
    setVals(fromPrice(p));
    setActive(p?.isActive ?? true);
    setErrors({});
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token || !cabang) return;
    const parsed = parseAmounts(vals);
    if ("bad" in parsed) {
      setErrors({ [parsed.bad]: "Must be a number from 0" });
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = await api.setPenunjangPrice(token, penunjang.id, cabang, { ...parsed.ok, isActive: active });
      onSaved(saved, cabang);
    } catch (err) {
      saveError(err, onUnauthorized, setErrors, setError);
    } finally {
      setBusy(false);
    }
  }

  const amount = (key: Amount, label: string) => (
    <FieldShell id={`price-${key}`} label={label} error={errors[key]}>
      <AmountInput
        id={`price-${key}`}
        value={vals[key]}
        invalid={!!errors[key]}
        onChange={(v) => {
          setVals((x) => ({ ...x, [key]: v }));
          setErrors(({ [key]: _, ...rest }) => rest);
        }}
      />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={`Harga · ${penunjang.nama}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell
          id="price-cabang"
          label="Cabang"
          wide
          hint={current ? undefined : `No price row for ${cabang} yet; saving creates it (starting from the default prices).`}
        >
          <select id="price-cabang" className="select" value={cabang} onChange={(e) => pick(e.target.value)}>
            {branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.kode} · {b.nama}
              </option>
            ))}
          </select>
        </FieldShell>
        {amount("hjual", "Hrg Jual")}
        {amount("hjualJamkes", "Hrg Jual Jamkes")}
        <FieldShell id="price-active" label="Status">
          <select id="price-active" className="select" value={active ? "1" : "0"} onChange={(e) => setActive(e.target.value === "1")}>
            <option value="1">Aktif</option>
            <option value="0">Non Aktif</option>
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
