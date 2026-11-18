import { FormEvent, KeyboardEvent, ReactNode, useEffect, useMemo, useState } from "react";
import { ArrowLeftRight, Building2, ListChecks, Lock, Pencil, Plus, Trash2 } from "lucide-react";
import {
  api,
  ApiError,
  Dokter,
  DokterCabangLists,
  DokterInput,
  DokterOptions,
  DokterTindakanItem,
  DokterTindakanLists,
  FieldErrors,
  LookupOption,
} from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";

type Tab = "data" | "tindakan" | "cabang";
const TABS: { id: Tab; label: string }[] = [
  { id: "data", label: "Data Dokter" },
  { id: "tindakan", label: "Tindakan Dokter" },
  { id: "cabang", label: "Cabang Praktek" },
];

/** Master Data Klinik > Dokter: doctors, the procedures they may perform and their practice branches. */
export default function DokterPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<Dokter[] | null>(null);
  const [branchNames, setBranchNames] = useState<Map<string, string>>(new Map());
  const [options, setOptions] = useState<DokterOptions | null>(null);
  const [loadError, setLoadError] = useState("");
  const [tab, setTab] = useState<Tab>("data");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [editing, setEditing] = useState<Dokter | "new" | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.dokter(token), api.dokterOptions(token)])
      .then(([res, opts]) => {
        setList(res.dokter);
        setBranchNames(new Map(res.branches.map((b) => [b.kode, b.nama])));
        setOptions(opts);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load doctors");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const names = useMemo(() => {
    const toMap = (l: LookupOption[] = []) => new Map(l.map((o) => [o.id, o.name]));
    return { jenis: toMap(options?.jenis), gender: toMap(options?.gender) };
  }, [options]);
  const nameOf = (m: Map<number, string>, id: number | null) => (id === null ? "" : (m.get(id) ?? ""));
  const selected = list?.find((d) => d.id === selectedId) ?? null;

  const replace = (d: Dokter, isNew: boolean) =>
    setList((l) => (isNew ? [...(l ?? []), d] : (l ?? []).map((x) => (x.id === d.id ? d : x))));
  const openTab = (d: Dokter, t: Tab) => {
    setSelectedId(d.id);
    setTab(t);
    setNotice(null);
  };

  const columns: Column<Dokter>[] = [
    { key: "id", header: "Kode", value: (d) => String(d.id), sortValue: (d) => d.id },
    { key: "nama", header: "Nama Dokter", value: (d) => d.nama, className: "dt-wrap" },
    { key: "jenis", header: "Jenis", value: (d) => orDash(nameOf(names.jenis, d.idJenis)) },
    { key: "alamat", header: "Alamat", value: (d) => orDash(d.alamat), className: "dt-wrap" },
    { key: "telp", header: "Telp", value: (d) => orDash(d.telp) },
    { key: "hp", header: "Hp", value: (d) => orDash(d.hp) },
    { key: "gender", header: "Jenis Kelamin", value: (d) => orDash(nameOf(names.gender, d.idGender)) },
    { key: "status", header: "Status Active", value: (d) => (d.isActive ? "Active" : "Non Active") },
    {
      key: "cabang",
      header: "Cabang Praktek",
      value: (d) => d.cabangPraktek.map((c) => branchNames.get(c) ?? c).join(", "),
      className: "dt-wrap dt-wide",
    },
    { key: "spesialis", header: "Spesialis", value: (d) => orDash(d.spesialis), hidden: true },
    { key: "pendidikan", header: "Pendidikan", value: (d) => orDash(d.pendidikan), hidden: true },
    { key: "nik", header: "NIK", value: (d) => orDash(d.nik), hidden: true },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${d.nama}`} title="Edit" onClick={() => setEditing(d)}>
            <Pencil aria-hidden="true" />
          </button>
          <button type="button" className="row-action" aria-label={`Tindakan of ${d.nama}`} title="Tindakan Dokter" onClick={() => openTab(d, "tindakan")}>
            <ListChecks aria-hidden="true" />
          </button>
          <button type="button" className="row-action" aria-label={`Cabang praktek of ${d.nama}`} title="Cabang Praktek" onClick={() => openTab(d, "cabang")}>
            <Building2 aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  // Arrow keys move between tabs (WAI-ARIA tabs pattern).
  const onTabKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    const i = TABS.findIndex((t) => t.id === tab);
    const next = e.key === "ArrowRight" ? (i + 1) % TABS.length : e.key === "ArrowLeft" ? (i + TABS.length - 1) % TABS.length : -1;
    if (next < 0) return;
    e.preventDefault();
    setTab(TABS[next].id);
    document.getElementById(`dokter-tab-${TABS[next].id}`)?.focus();
  };

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Dokter</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!options}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>

      <div className="tabs" role="tablist" aria-label="Dokter">
        {TABS.map((t) => (
          <button
            key={t.id}
            id={`dokter-tab-${t.id}`}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={`dokter-panel-${t.id}`}
            tabIndex={tab === t.id ? 0 : -1}
            className={tab === t.id ? "tab active" : "tab"}
            onClick={() => setTab(t.id)}
            onKeyDown={onTabKey}
          >
            {t.label}
          </button>
        ))}
      </div>

      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
        </p>
      )}

      <div role="tabpanel" id={`dokter-panel-${tab}`} aria-labelledby={`dokter-tab-${tab}`} className="tab-panel">
        {loadError ? (
          <p className="alert error" role="alert">
            {loadError}
          </p>
        ) : !list || !options ? (
          <p className="muted">Loading…</p>
        ) : tab === "data" ? (
          <DataTable title="Dokter" columns={columns} rows={list} rowKey={(d) => d.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
        ) : !selected ? (
          <p className="alert info" role="status">
            Choose a doctor first: in Data Dokter, use the <ListChecks aria-hidden="true" className="inline-icon" /> or{" "}
            <Building2 aria-hidden="true" className="inline-icon" /> button of a row.
          </p>
        ) : (
          <>
            <div className="form-grid selected-dokter">
              <FieldShell id="sel-nama" label="Nama">
                <input id="sel-nama" value={selected.nama} readOnly className="readonly" />
              </FieldShell>
              <FieldShell id="sel-jenis" label="Jenis">
                <input id="sel-jenis" value={nameOf(names.jenis, selected.idJenis) || "-"} readOnly className="readonly" />
              </FieldShell>
            </div>
            {tab === "tindakan" ? (
              <TindakanTab key={selected.id} dokter={selected} onUnauthorized={expired} onNotice={setNotice} />
            ) : (
              <CabangTab
                key={selected.id}
                dokter={selected}
                onUnauthorized={expired}
                onNotice={setNotice}
                onChanged={(codes) => replace({ ...selected, cabangPraktek: codes }, false)}
              />
            )}
          </>
        )}
      </div>

      {editing && options && (
        <DokterForm
          dokter={editing === "new" ? null : editing}
          options={options}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(d, isNew) => {
            replace(d, isNew);
            setEditing(null);
            setNotice({ kind: "success", text: `Dokter ${d.nama} ${isNew ? "added" : "updated"}.` });
          }}
        />
      )}
    </section>
  );
}

// ---------- two lists: assigned (left) and available (right) ----------

interface DualListProps<T> {
  rows: { assigned: T[]; available: T[] };
  rowKey: (row: T) => string | number;
  columns: Column<T>[];
  title: string;
  /** Rows the user may not change (shown with a lock). */
  locked?: (row: T) => boolean;
  busy: boolean;
  pageSize: number;
  onRemove: (rows: T[]) => void;
  onAdd: (rows: T[]) => void;
}

/** Left: what the doctor has (tick + Delete, or the trash icon). Right: what can be added (tick + Move). */
function DualList<T>({ rows, rowKey, columns, title, locked = () => false, busy, pageSize, onRemove, onAdd }: DualListProps<T>) {
  const [left, setLeft] = useState<Set<string | number>>(new Set());
  const [right, setRight] = useState<Set<string | number>>(new Set());

  // Selections refer to rows that may have moved; clear them when the lists change.
  useEffect(() => {
    setLeft(new Set());
    setRight(new Set());
  }, [rows]);

  const toggle = (set: Set<string | number>, setSet: (s: Set<string | number>) => void, key: string | number) => {
    const next = new Set(set);
    if (next.has(key)) next.delete(key);
    else next.add(key);
    setSet(next);
  };

  const selectable = (list: T[]) => list.filter((r) => !locked(r));
  const checkColumn = (
    list: T[],
    set: Set<string | number>,
    setSet: (s: Set<string | number>) => void,
    side: "left" | "right",
  ): Column<T> => {
    const all = selectable(list);
    const allChecked = all.length > 0 && all.every((r) => set.has(rowKey(r)));
    return {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      headerNode: (
        <label className="check-all">
          <input
            type="checkbox"
            checked={allChecked}
            disabled={all.length === 0 || busy}
            aria-label={`Select all ${side === "left" ? "assigned" : "available"} ${title}`}
            onChange={() => setSet(allChecked ? new Set() : new Set(all.map(rowKey)))}
          />
          Action
        </label>
      ),
      render: (r) => {
        const key = rowKey(r);
        if (locked(r)) {
          return (
            <span className="lock-note" title="Not one of your branches">
              <Lock aria-hidden="true" />
              <span className="visually-hidden">Not one of your branches</span>
            </span>
          );
        }
        return (
          <span className="row-actions">
            <input type="checkbox" checked={set.has(key)} disabled={busy} aria-label={`Select ${key}`} onChange={() => toggle(set, setSet, key)} />
            {side === "left" && (
              <button type="button" className="row-action danger" aria-label={`Remove ${key}`} title="Remove" disabled={busy} onClick={() => onRemove([r])}>
                <Trash2 aria-hidden="true" />
              </button>
            )}
          </span>
        );
      },
    };
  };

  const picked = (list: T[], set: Set<string | number>) => list.filter((r) => set.has(rowKey(r)));
  const pageSizes = [5, 10, 15, 20, 50, 100];

  return (
    <div className="dual-list">
      <div className="dual-side">
        <div className="dual-head">
          <h2>
            {title} ({rows.assigned.length})
          </h2>
          <button type="button" className="btn-add danger-outline" disabled={busy || left.size === 0} onClick={() => onRemove(picked(rows.assigned, left))}>
            <Trash2 aria-hidden="true" /> Delete{left.size ? ` (${left.size})` : ""}
          </button>
        </div>
        <DataTable
          title={`${title} assigned`}
          exports={false}
          columns={[...columns, checkColumn(rows.assigned, left, setLeft, "left")]}
          rows={rows.assigned}
          rowKey={rowKey}
          pageSizes={pageSizes}
          defaultPageSize={pageSize}
          initialSort={{ key: columns[0].key, dir: "asc" }}
        />
      </div>
      <div className="dual-side">
        <div className="dual-head">
          <h2>Available ({rows.available.length})</h2>
          <button type="button" className="btn-add" disabled={busy || right.size === 0} onClick={() => onAdd(picked(rows.available, right))}>
            <ArrowLeftRight aria-hidden="true" /> Move{right.size ? ` (${right.size})` : ""}
          </button>
        </div>
        <DataTable
          title={`${title} available`}
          exports={false}
          columns={[...columns, checkColumn(rows.available, right, setRight, "right")]}
          rows={rows.available}
          rowKey={rowKey}
          pageSizes={pageSizes}
          defaultPageSize={pageSize}
          initialSort={{ key: columns[0].key, dir: "asc" }}
        />
      </div>
    </div>
  );
}

type Notify = (n: { kind: "success" | "error"; text: string } | null) => void;

function useLists<L>(load: () => Promise<L>, onUnauthorized: () => void, onNotice: Notify) {
  const [lists, setLists] = useState<L | null>(null);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    load()
      .then(setLists)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) onUnauthorized();
        else onNotice({ kind: "error", text: err instanceof Error ? err.message : "Failed to load" });
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  const change = async (run: () => Promise<L>, done: string) => {
    setBusy(true);
    onNotice(null);
    try {
      setLists(await run());
      onNotice({ kind: "success", text: done });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else onNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to save" });
    } finally {
      setBusy(false);
    }
  };
  return { lists, busy, change };
}

function TindakanTab({ dokter, onUnauthorized, onNotice }: { dokter: Dokter; onUnauthorized: () => void; onNotice: Notify }) {
  const { token } = useAuth();
  const { lists, busy, change } = useLists<DokterTindakanLists>(() => api.dokterTindakan(token!, dokter.id), onUnauthorized, onNotice);
  const columns: Column<DokterTindakanItem>[] = [
    { key: "id", header: "Kode", value: (t) => String(t.id), sortValue: (t) => t.id },
    { key: "nama", header: "Tindakan", value: (t) => t.nama, className: "dt-wrap" },
    { key: "jenis", header: "Jenis Tindakan", value: (t) => orDash(t.jenis) },
  ];
  if (!lists) return <p className="muted">Loading…</p>;
  return (
    <DualList
      title="Tindakan"
      rows={lists}
      rowKey={(t) => t.id}
      columns={columns}
      busy={busy}
      pageSize={15}
      onRemove={(rows) =>
        change(() => api.changeDokterTindakan(token!, dokter.id, { remove: rows.map((t) => t.id) }), `${rows.length} tindakan removed from ${dokter.nama}.`)
      }
      onAdd={(rows) =>
        change(() => api.changeDokterTindakan(token!, dokter.id, { add: rows.map((t) => t.id) }), `${rows.length} tindakan added to ${dokter.nama}.`)
      }
    />
  );
}

type BranchRow = { kode: string; nama: string };

function CabangTab({
  dokter,
  onUnauthorized,
  onNotice,
  onChanged,
}: {
  dokter: Dokter;
  onUnauthorized: () => void;
  onNotice: Notify;
  onChanged: (codes: string[]) => void;
}) {
  const { token } = useAuth();
  const { lists, busy, change } = useLists<DokterCabangLists>(() => api.dokterCabang(token!, dokter.id), onUnauthorized, onNotice);
  useEffect(() => {
    if (lists) onChanged(lists.assigned.map((b) => b.kode));
  }, [lists]); // eslint-disable-line react-hooks/exhaustive-deps
  const columns: Column<BranchRow>[] = [
    { key: "kode", header: "Kode", value: (b) => b.kode },
    { key: "nama", header: "Nama Cabang", value: (b) => b.nama },
  ];
  if (!lists) return <p className="muted">Loading…</p>;
  const editable = new Set(lists.editable);
  return (
    <>
      <p className="muted form-note">You can only add or remove your own branches; the others show a lock.</p>
      <DualList
        title="Cabang Praktek"
        rows={lists}
        rowKey={(b) => b.kode}
        columns={columns}
        locked={(b) => !editable.has(b.kode)}
        busy={busy}
        pageSize={10}
        onRemove={(rows) =>
          change(() => api.changeDokterCabang(token!, dokter.id, { remove: rows.map((b) => b.kode) }), `Removed ${rows.map((b) => b.kode).join(", ")}.`)
        }
        onAdd={(rows) =>
          change(() => api.changeDokterCabang(token!, dokter.id, { add: rows.map((b) => b.kode) }), `Added ${rows.map((b) => b.kode).join(", ")}.`)
        }
      />
    </>
  );
}

// ---------- doctor form ----------

const EMPTY: DokterInput = {
  nama: "",
  idJenis: null,
  tglMasuk: null,
  tmptLahir: "",
  tglLahir: null,
  alamat: "",
  telp: "",
  hp: "",
  pendidikan: "",
  spesialis: "",
  idGender: null,
  idAgama: null,
  idGolDarah: null,
  idStatusKwn: null,
  nik: "",
  isActive: true,
};

function toInput(d: Dokter): DokterInput {
  const { id: _id, cabangPraktek: _c, updatedBy: _by, updatedAt: _at, ...rest } = d;
  return rest;
}

type TextKey = "nama" | "tmptLahir" | "alamat" | "telp" | "hp" | "pendidikan" | "spesialis" | "nik";
type LookupKey = "idJenis" | "idGender" | "idAgama" | "idGolDarah" | "idStatusKwn";

function DokterForm({
  dokter,
  options,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  dokter: Dokter | null;
  options: DokterOptions;
  onClose: () => void;
  onSaved: (d: Dokter, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<DokterInput>(() => (dokter ? toInput(dokter) : EMPTY));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const today = new Date().toISOString().slice(0, 10);

  const update = <K extends keyof DokterInput>(key: K, value: DokterInput[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!form.nama.trim()) {
      setErrors({ nama: "Nama Dokter is required" });
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = dokter ? await api.updateDokter(token, dokter.id, form) : await api.createDokter(token, form);
      onSaved(saved, !dokter);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const text = (key: TextKey, label: string, max: number, extra: { required?: boolean; wide?: boolean } = {}): ReactNode => (
    <FieldShell id={`dokter-${key}`} label={label} error={errors[key]} required={extra.required} wide={extra.wide}>
      <input id={`dokter-${key}`} value={form[key]} maxLength={max} aria-invalid={errors[key] ? "true" : undefined} onChange={(e) => update(key, e.target.value)} />
    </FieldShell>
  );
  const lookup = (key: LookupKey, label: string, list: LookupOption[]) => (
    <FieldShell id={`dokter-${key}`} label={label} error={errors[key]}>
      <select id={`dokter-${key}`} className="select" value={form[key] ?? ""} onChange={(e) => update(key, e.target.value === "" ? null : Number(e.target.value))}>
        <option value="">-</option>
        {list.map((o) => (
          <option key={o.id} value={o.id}>
            {o.name}
          </option>
        ))}
      </select>
    </FieldShell>
  );
  const date = (key: "tglLahir" | "tglMasuk", label: string) => (
    <FieldShell id={`dokter-${key}`} label={label} error={errors[key]}>
      <input id={`dokter-${key}`} type="date" max={today} value={form[key] ?? ""} onChange={(e) => update(key, e.target.value || null)} />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={dokter ? `Edit Dokter #${dokter.id}` : "Add Dokter"} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid form-grid-3">
        {text("nama", "Nama Dokter", 30, { required: true })}
        {lookup("idJenis", "Jenis", options.jenis)}
        <FieldShell id="dokter-isActive" label="Status">
          <select id="dokter-isActive" className="select" value={form.isActive ? "1" : "0"} onChange={(e) => update("isActive", e.target.value === "1")}>
            <option value="1">Active</option>
            <option value="0">Non Active</option>
          </select>
        </FieldShell>
        {text("tmptLahir", "Tempat Lahir", 25)}
        {date("tglLahir", "Tanggal Lahir")}
        {date("tglMasuk", "Tanggal Masuk")}
        {text("alamat", "Alamat", 100, { wide: true })}
        {text("telp", "Telp", 20)}
        {text("hp", "Hp", 20)}
        {text("nik", "NIK", 20)}
        {text("pendidikan", "Pendidikan", 30)}
        {text("spesialis", "Spesialis", 30)}
        {lookup("idGender", "Jenis Kelamin", options.gender)}
        {lookup("idAgama", "Agama", options.agama)}
        {lookup("idGolDarah", "Gol Darah", options.golDarah)}
        {lookup("idStatusKwn", "Status Kawin", options.statusKwn)}
      </div>
    </Modal>
  );
}
