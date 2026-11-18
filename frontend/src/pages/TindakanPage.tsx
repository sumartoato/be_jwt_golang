import { FormEvent, useEffect, useMemo, useState } from "react";
import { DollarSign, Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, LookupOption, Tindakan, TindakanInput, TindakanPrice } from "../api";
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

const EMPTY: TindakanInput = {
  nama: "",
  idJenis: null,
  hbeli: null,
  hjual: null,
  hjualJamkes: null,
  isActive: true,
  isEdit: false,
  cetakKartu: false,
};

const status = (on: boolean) => (on ? "Active" : "Non Active");

/** Master Data Klinik > Tindakan: procedures plus their prices at the user's branches. */
export default function TindakanPage() {
  const { token, logout } = useAuth();
  const [items, setItems] = useState<Tindakan[] | null>(null);
  const [branches, setBranches] = useState<BranchRef[]>([]);
  const [jenis, setJenis] = useState<LookupOption[] | null>(null);
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Tindakan | "new" | null>(null);
  const [priceOf, setPriceOf] = useState<Tindakan | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.tindakan(token), api.tindakanOptions(token)])
      .then(([list, opts]) => {
        setItems(list.tindakan);
        setBranches(list.branches);
        setJenis(opts.jenis);
        setBranch(initialBranch(list.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load tindakan");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const jenisName = useMemo(() => new Map((jenis ?? []).map((j) => [j.id, j.name])), [jenis]);

  const replace = (t: Tindakan, isNew: boolean) =>
    setItems((list) => (isNew ? [...(list ?? []), t] : (list ?? []).map((x) => (x.id === t.id ? t : x))));

  const perBranch = (
    key: string,
    header: string,
    pick: (p: TindakanPrice) => string,
    sortPick?: (p: TindakanPrice) => number | null,
    hidden = false,
  ) => perBranchColumn<Tindakan, TindakanPrice>({ key, header, rowsOf: (t) => t.prices, branch, pick, sortPick, hidden });

  const columns: Column<Tindakan>[] = [
    { key: "id", header: "Kode", value: (t) => String(t.id), sortValue: (t) => t.id },
    { key: "nama", header: "Nama Tindakan", value: (t) => t.nama, className: "dt-wrap" },
    { key: "jenis", header: "Jenis Tindakan", value: (t) => (t.idJenis === null ? "" : (jenisName.get(t.idJenis) ?? `#${t.idJenis}`)) },
    perBranch("hbeli", "Hrg Beli", (p) => fmt(p.hbeli), (p) => p.hbeli),
    perBranch("hjual", "Hrg Jual", (p) => fmt(p.hjual), (p) => p.hjual),
    perBranch("hjualJamkes", "Hrg Jual Jamkes", (p) => fmt(p.hjualJamkes), (p) => p.hjualJamkes),
    perBranch("branchStatus", "Status Cabang", (p) => status(p.isActive), undefined, true),
    { key: "status", header: "Status", value: (t) => status(t.isActive) },
    { key: "isEdit", header: "Is Edit", value: (t) => (t.isEdit ? "Ya" : "Tidak"), hidden: true },
    { key: "cetakKartu", header: "Cetak Kartu", value: (t) => (t.cetakKartu ? "Ya" : "Tidak"), hidden: true },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (t) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${t.nama}`} title="Edit" onClick={() => setEditing(t)}>
            <Pencil aria-hidden="true" />
          </button>
          <button
            type="button"
            className="row-action"
            aria-label={`Prices of ${t.nama}`}
            title={branches.length ? "Prices per branch" : "You are not assigned to any branch"}
            disabled={branches.length === 0}
            onClick={() => setPriceOf(t)}
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
        <h1>Tindakan</h1>
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
          title="Tindakan"
          columns={columns}
          rows={items}
          rowKey={(t) => t.id}
          initialSort={{ key: "id", dir: "asc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
        />
      )}
      {editing && jenis && (
        <TindakanForm
          tindakan={editing === "new" ? null : editing}
          jenis={jenis}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(t, isNew) => {
            replace(t, isNew);
            setSaved(`Tindakan ${t.nama} ${isNew ? "added" : "updated"}.`);
            setEditing(null);
          }}
        />
      )}
      {priceOf && (
        <PriceForm
          tindakan={priceOf}
          branches={branches}
          initialBranch={branch === ALL_BRANCHES ? branches[0]?.kode : branch}
          onClose={() => setPriceOf(null)}
          onUnauthorized={expired}
          onSaved={(t, cabang) => {
            replace(t, false);
            setSaved(`Prices of ${t.nama} at ${cabang} saved.`);
            setPriceOf(null);
          }}
        />
      )}
    </section>
  );
}

type Amount = "hbeli" | "hjual" | "hjualJamkes";

function saveError(err: unknown, onUnauthorized: () => void, setErrors: (e: FieldErrors) => void, setError: (s: string) => void) {
  if (err instanceof ApiError && err.status === 401) onUnauthorized();
  else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
  else setError(err instanceof Error ? err.message : "Failed to save");
}

// ---------- procedure master form ----------

function TindakanForm({
  tindakan,
  jenis,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  tindakan: Tindakan | null;
  jenis: LookupOption[];
  onClose: () => void;
  onSaved: (t: Tindakan, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const src = tindakan ?? EMPTY;
  const [form, setForm] = useState<TindakanInput>(() => ({
    nama: src.nama,
    idJenis: src.idJenis,
    hbeli: src.hbeli,
    hjual: src.hjual,
    hjualJamkes: src.hjualJamkes,
    isActive: src.isActive,
    isEdit: src.isEdit,
    cetakKartu: src.cetakKartu,
  }));
  const [amounts, setAmounts] = useState<Record<Amount, string>>(() => ({
    hbeli: numText(src.hbeli),
    hjual: numText(src.hjual),
    hjualJamkes: numText(src.hjualJamkes),
  }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // Keep a type id that is no longer in the lookup table (lab has one).
  const missingJenis = form.idJenis !== null && !jenis.some((j) => j.id === form.idJenis);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!form.nama.trim()) {
      setErrors({ nama: "Nama Tindakan is required" });
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
      const saved = tindakan ? await api.updateTindakan(token, tindakan.id, body) : await api.createTindakan(token, body);
      onSaved(saved, !tindakan);
    } catch (err) {
      saveError(err, onUnauthorized, setErrors, setError);
    } finally {
      setBusy(false);
    }
  }

  const amount = (key: Amount, label: string) => (
    <FieldShell id={`tindakan-${key}`} label={label} error={errors[key]}>
      <AmountInput
        id={`tindakan-${key}`}
        value={amounts[key]}
        invalid={!!errors[key]}
        onChange={(v) => {
          setAmounts((a) => ({ ...a, [key]: v }));
          setErrors(({ [key]: _, ...rest }) => rest);
        }}
      />
    </FieldShell>
  );

  const yesNo = (key: "isActive" | "isEdit" | "cetakKartu", label: string, yes: string, no: string) => (
    <FieldShell id={`tindakan-${key}`} label={label}>
      <select
        id={`tindakan-${key}`}
        className="select"
        value={form[key] ? "1" : "0"}
        onChange={(e) => setForm((f) => ({ ...f, [key]: e.target.value === "1" }))}
      >
        <option value="1">{yes}</option>
        <option value="0">{no}</option>
      </select>
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={tindakan ? `Edit Tindakan #${tindakan.id}` : "Add Tindakan"} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <fieldset className="form-section">
        <legend>Data Tindakan</legend>
        <div className="form-grid">
          <FieldShell id="tindakan-nama" label="Nama Tindakan" error={errors.nama} wide required>
            <input
              id="tindakan-nama"
              value={form.nama}
              maxLength={100}
              aria-invalid={errors.nama ? "true" : undefined}
              onChange={(e) => {
                setForm((f) => ({ ...f, nama: e.target.value }));
                setErrors(({ nama: _, ...rest }) => rest);
              }}
            />
          </FieldShell>
          <FieldShell id="tindakan-idJenis" label="Jenis Tindakan">
            <select
              id="tindakan-idJenis"
              className="select"
              value={form.idJenis ?? ""}
              onChange={(e) => setForm((f) => ({ ...f, idJenis: e.target.value === "" ? null : Number(e.target.value) }))}
            >
              <option value="">-</option>
              {missingJenis && <option value={form.idJenis!}>#{form.idJenis} (not in the list)</option>}
              {jenis.map((j) => (
                <option key={j.id} value={j.id}>
                  {j.name}
                </option>
              ))}
            </select>
          </FieldShell>
          {yesNo("isActive", "Status", "Active", "Non Active")}
          {yesNo("isEdit", "Is Edit", "Ya", "Tidak")}
          {yesNo("cetakKartu", "Cetak Kartu", "Ya", "Tidak")}
        </div>
      </fieldset>
      <fieldset className="form-section">
        <legend>Harga Default</legend>
        <p className="muted form-note">Prices per branch are set with the $ button.</p>
        <div className="form-grid form-grid-3">
          {amount("hbeli", "Hrg Beli")}
          {amount("hjual", "Hrg Jual")}
          {amount("hjualJamkes", "Hrg Jual Jamkes")}
        </div>
      </fieldset>
    </Modal>
  );
}

// ---------- prices per branch ----------

function PriceForm({
  tindakan,
  branches,
  initialBranch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  tindakan: Tindakan;
  branches: BranchRef[];
  initialBranch?: string;
  onClose: () => void;
  onSaved: (t: Tindakan, cabang: string) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [cabang, setCabang] = useState(initialBranch ?? branches[0]?.kode ?? "");
  const current = tindakan.prices.find((p) => p.cabang === cabang);
  // A branch without a price row starts from the procedure's default prices.
  const fromPrice = (p?: TindakanPrice): Record<Amount, string> => ({
    hbeli: numText(p ? p.hbeli : tindakan.hbeli),
    hjual: numText(p ? p.hjual : tindakan.hjual),
    hjualJamkes: numText(p ? p.hjualJamkes : tindakan.hjualJamkes),
  });
  const [vals, setVals] = useState(() => fromPrice(current));
  const [active, setActive] = useState(current?.isActive ?? true);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const pick = (kode: string) => {
    setCabang(kode);
    const p = tindakan.prices.find((x) => x.cabang === kode);
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
      const saved = await api.setTindakanPrice(token, tindakan.id, cabang, { ...parsed.ok, isActive: active });
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
    <Modal dialogRef={dialogRef} title={`Harga · ${tindakan.nama}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
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
        {amount("hbeli", "Hrg Beli")}
        {amount("hjual", "Hrg Jual")}
        {amount("hjualJamkes", "Hrg Jual Jamkes")}
        <FieldShell id="price-active" label="Status">
          <select id="price-active" className="select" value={active ? "1" : "0"} onChange={(e) => setActive(e.target.value === "1")}>
            <option value="1">Active</option>
            <option value="0">Non Active</option>
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
