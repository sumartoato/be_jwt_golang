import { FormEvent, useEffect, useMemo, useState } from "react";
import { ArrowDown, ArrowUp, Info, ListChecks, Pencil, Plus, Trash2 } from "lucide-react";
import { api, ApiError, FieldErrors, Jamkes, JamkesInput, JamkesItem, JamkesTerm, LookupOption } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { fmt, orDash } from "../components/branchView";
import { AmountInput, FieldShell, Modal, numText, textNum, useModal } from "../components/FormKit";

/** Master Data Klinik > Jaminan Kesehatan: insurers, their PKS terms and item lists. */
export default function JamkesPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<Jamkes[] | null>(null);
  const [types, setTypes] = useState<LookupOption[]>([]);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Jamkes | "new" | null>(null);
  const [pksOf, setPksOf] = useState<Jamkes | null>(null);
  const [itemsOf, setItemsOf] = useState<Jamkes | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .jamkes(token)
      .then((res) => {
        setList(res.jamkes);
        setTypes(res.types);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const typeName = useMemo(() => new Map(types.map((t) => [t.id, t.name])), [types]);
  const replace = (j: Jamkes, isNew: boolean) =>
    setList((l) => (isNew ? [...(l ?? []), j] : (l ?? []).map((x) => (x.id === j.id ? j : x))));

  const columns: Column<Jamkes>[] = [
    { key: "id", header: "Kode", value: (j) => String(j.id), sortValue: (j) => j.id },
    { key: "nama", header: "Nama", value: (j) => j.nama, className: "dt-wrap" },
    { key: "alamat", header: "Alamat", value: (j) => orDash(j.alamat), className: "dt-wrap" },
    { key: "jenis", header: "Jenis", value: (j) => orDash(j.idType === null ? "" : typeName.get(j.idType)) },
    { key: "telp", header: "Telp", value: (j) => orDash(j.telp) },
    { key: "fax", header: "Fax", value: (j) => orDash(j.fax) },
    { key: "status", header: "Status", value: (j) => (j.isActive ? "Active" : "Non Active") },
    { key: "statusLimit", header: "Status Limit", value: (j) => (j.hasLimit ? "Limit" : "No Limit") },
    { key: "limit", header: "Limit", value: (j) => (j.limit === null ? "0" : fmt(j.limit)), sortValue: (j) => j.limit ?? 0 },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (j) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${j.nama}`} title="Edit" onClick={() => setEditing(j)}>
            <Pencil aria-hidden="true" />
          </button>
          <button type="button" className="row-action" aria-label={`PKS terms of ${j.nama}`} title="Ketentuan PKS" onClick={() => setPksOf(j)}>
            <ListChecks aria-hidden="true" />
          </button>
          <button type="button" className="row-action" aria-label={`Item list of ${j.nama}`} title="Detail item" onClick={() => setItemsOf(j)}>
            <Info aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Jaminan Kesehatan</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !list ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable title="Jaminan Kesehatan" columns={columns} rows={list} rowKey={(j) => j.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <JamkesForm
          jamkes={editing === "new" ? null : editing}
          types={types}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(j, isNew) => {
            replace(j, isNew);
            setEditing(null);
            setNotice({ kind: "success", text: `${j.nama} ${isNew ? "added" : "updated"}.` });
          }}
        />
      )}
      {pksOf && (
        <PksDialog
          jamkes={pksOf}
          onClose={() => setPksOf(null)}
          onUnauthorized={expired}
          onDone={(text) => {
            setPksOf(null);
            setNotice({ kind: "success", text });
          }}
        />
      )}
      {itemsOf && (
        <ItemsDialog
          jamkes={itemsOf}
          onClose={() => setItemsOf(null)}
          onUnauthorized={expired}
          onDone={(text) => {
            setItemsOf(null);
            setNotice({ kind: "success", text });
          }}
        />
      )}
    </section>
  );
}

function errorText(err: unknown) {
  if (err instanceof ApiError) return Object.values(err.fieldErrors)[0] ?? err.message;
  return err instanceof Error ? err.message : "Failed to save";
}

// ---------- insurer form ----------

function JamkesForm({
  jamkes,
  types,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  jamkes: Jamkes | null;
  types: LookupOption[];
  onClose: () => void;
  onSaved: (j: Jamkes, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<JamkesInput>(() => ({
    nama: jamkes?.nama ?? "",
    alamat: jamkes?.alamat ?? "",
    telp: jamkes?.telp ?? "",
    fax: jamkes?.fax ?? "",
    idType: jamkes?.idType ?? null,
    isActive: jamkes?.isActive ?? true,
    hasLimit: jamkes?.hasLimit ?? false,
    limit: jamkes?.limit ?? null,
  }));
  const [limitText, setLimitText] = useState(numText(jamkes?.limit && jamkes.hasLimit ? jamkes.limit : null));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const update = <K extends keyof JamkesInput>(key: K, value: JamkesInput[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const limit = form.hasLimit ? textNum(limitText) : null;
    const local: FieldErrors = {};
    if (!form.nama.trim()) local.nama = "Nama is required";
    if (form.hasLimit && (limit === null || Number.isNaN(limit) || limit <= 0)) local.limit = "Enter the limit, or choose No Limit";
    setErrors(local);
    if (Object.keys(local).length) return;
    setBusy(true);
    setError("");
    try {
      const body = { ...form, limit };
      const saved = jamkes ? await api.updateJamkes(token, jamkes.id, body) : await api.createJamkes(token, body);
      onSaved(saved, !jamkes);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  const text = (key: "nama" | "alamat" | "telp" | "fax", label: string, max: number, extra: { required?: boolean; wide?: boolean } = {}) => (
    <FieldShell id={`jamkes-${key}`} label={label} error={errors[key]} required={extra.required} wide={extra.wide}>
      <input id={`jamkes-${key}`} value={form[key]} maxLength={max} aria-invalid={errors[key] ? "true" : undefined} onChange={(e) => update(key, e.target.value)} />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={jamkes ? `Edit Jaminan Kesehatan #${jamkes.id}` : "Add Jaminan Kesehatan"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        {text("nama", "Nama", 100, { required: true, wide: true })}
        {text("alamat", "Alamat", 100, { wide: true })}
        {text("telp", "Telp", 15)}
        {text("fax", "Fax", 15)}
        <FieldShell id="jamkes-idType" label="Jenis" error={errors.idType}>
          <select id="jamkes-idType" className="select" value={form.idType ?? ""} onChange={(e) => update("idType", e.target.value === "" ? null : Number(e.target.value))}>
            <option value="">-</option>
            {types.map((t) => (
              <option key={t.id} value={t.id}>
                {t.name}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="jamkes-isActive" label="Status">
          <select id="jamkes-isActive" className="select" value={form.isActive ? "1" : "0"} onChange={(e) => update("isActive", e.target.value === "1")}>
            <option value="1">Active</option>
            <option value="0">Non Active</option>
          </select>
        </FieldShell>
        <FieldShell id="jamkes-hasLimit" label="Status Limit">
          <select id="jamkes-hasLimit" className="select" value={form.hasLimit ? "1" : "0"} onChange={(e) => update("hasLimit", e.target.value === "1")}>
            <option value="0">No Limit</option>
            <option value="1">Limit</option>
          </select>
        </FieldShell>
        <FieldShell id="jamkes-limit" label="Limit" error={errors.limit} hint={form.hasLimit ? undefined : "Only used with Status Limit = Limit."}>
          {form.hasLimit ? (
            <AmountInput
              id="jamkes-limit"
              value={limitText}
              invalid={!!errors.limit}
              onChange={(v) => {
                setLimitText(v);
                setErrors(({ limit: _, ...rest }) => rest);
              }}
            />
          ) : (
            <input id="jamkes-limit" value="0" readOnly className="readonly" />
          )}
        </FieldShell>
      </div>
    </Modal>
  );
}

// ---------- PKS terms ----------

function PksDialog({ jamkes, onClose, onDone, onUnauthorized }: { jamkes: Jamkes; onClose: () => void; onDone: (text: string) => void; onUnauthorized: () => void }) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [terms, setTerms] = useState<JamkesTerm[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!token) return;
    api
      .jamkesPks(token, jamkes.id)
      .then((r) => setTerms(r.terms))
      .catch((err) => (err instanceof ApiError && err.status === 401 ? onUnauthorized() : setError(errorText(err))));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const set = (i: number, patch: Partial<JamkesTerm>) => setTerms((t) => t && t.map((x, j) => (j === i ? { ...x, ...patch } : x)));

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token || !terms) return;
    setBusy(true);
    setError("");
    try {
      await api.saveJamkesPks(token, jamkes.id, terms.map((t) => ({ detailId: t.detailId, value: t.value, shown: t.shown })));
      onDone(`PKS terms of ${jamkes.nama} saved.`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={`Ketentuan PKS · ${jamkes.nama}`} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      {!terms ? (
        <p className="muted">Loading…</p>
      ) : (
        <>
          <p className="muted form-note">Leave a term empty to remove it. Untick "Tampil" to keep the text but hide it.</p>
          <div className="term-list">
            {terms.map((t, i) => (
              <div key={t.detailId} className="term-row">
                <label htmlFor={`term-${t.detailId}`} className="term-label">
                  {t.description}
                </label>
                <textarea id={`term-${t.detailId}`} rows={2} maxLength={5000} value={t.value} onChange={(e) => set(i, { value: e.target.value })} />
                <label className="check-all term-shown">
                  <input type="checkbox" checked={t.shown} onChange={(e) => set(i, { shown: e.target.checked })} /> Tampil
                </label>
              </div>
            ))}
          </div>
        </>
      )}
    </Modal>
  );
}

// ---------- numbered item list ----------

function ItemsDialog({ jamkes, onClose, onDone, onUnauthorized }: { jamkes: Jamkes; onClose: () => void; onDone: (text: string) => void; onUnauthorized: () => void }) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [items, setItems] = useState<JamkesItem[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!token) return;
    api
      .jamkesItems(token, jamkes.id)
      .then((r) => setItems(r.items))
      .catch((err) => (err instanceof ApiError && err.status === 401 ? onUnauthorized() : setError(errorText(err))));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const set = (i: number, patch: Partial<JamkesItem>) => setItems((l) => l && l.map((x, j) => (j === i ? { ...x, ...patch } : x)));
  const move = (i: number, d: -1 | 1) =>
    setItems((l) => {
      if (!l || i + d < 0 || i + d >= l.length) return l;
      const next = [...l];
      [next[i], next[i + d]] = [next[i + d], next[i]];
      return next;
    });

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token || !items) return;
    setBusy(true);
    setError("");
    try {
      const saved = await api.saveJamkesItems(token, jamkes.id, items.map((it) => ({ value: it.value, shown: it.shown })));
      onDone(`${saved.items.length} item(s) of ${jamkes.nama} saved.`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={`Detail Item · ${jamkes.nama}`} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      {!items ? (
        <p className="muted">Loading…</p>
      ) : (
        <>
          {items.length === 0 && <p className="muted">No items yet.</p>}
          <ol className="item-list">
            {items.map((it, i) => (
              <li key={i} className="item-row">
                <span className="item-no" aria-hidden="true">
                  {i + 1}.
                </span>
                <textarea aria-label={`Item ${i + 1}`} rows={2} maxLength={5000} value={it.value} onChange={(e) => set(i, { value: e.target.value })} />
                <span className="item-tools">
                  <label className="check-all">
                    <input type="checkbox" checked={it.shown} onChange={(e) => set(i, { shown: e.target.checked })} /> Tampil
                  </label>
                  <button type="button" className="row-action" aria-label={`Move item ${i + 1} up`} disabled={i === 0} onClick={() => move(i, -1)}>
                    <ArrowUp aria-hidden="true" />
                  </button>
                  <button type="button" className="row-action" aria-label={`Move item ${i + 1} down`} disabled={i === items.length - 1} onClick={() => move(i, 1)}>
                    <ArrowDown aria-hidden="true" />
                  </button>
                  <button type="button" className="row-action danger" aria-label={`Remove item ${i + 1}`} onClick={() => setItems((l) => l && l.filter((_, j) => j !== i))}>
                    <Trash2 aria-hidden="true" />
                  </button>
                </span>
              </li>
            ))}
          </ol>
          <button type="button" className="btn-add" onClick={() => setItems((l) => [...(l ?? []), { no: (l?.length ?? 0) + 1, value: "", shown: true }])}>
            <Plus aria-hidden="true" /> Add item
          </button>
        </>
      )}
    </Modal>
  );
}
