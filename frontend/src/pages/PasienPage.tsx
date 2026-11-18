import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Barcode, Pencil, Plus, Search } from "lucide-react";
import { api, ApiError, FieldErrors, KelurahanHit, Pasien, PasienInput, PasienOption, PasienOptions, PasienRow } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice, orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";
import { code128Svg } from "../components/barcode";
import { age, fmtDate, today } from "../components/dates";

/** Rows an export may fetch (the API caps one request at this size). */
const EXPORT_LIMIT = 5000;

const escapeHtml = (s: string) => s.replace(/[&<>"]/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[c]!);

/** Opens the patient card (name, barcode of No RM, No RM) in a new window and prints it. */
function printCard(p: PasienRow): boolean {
  const win = window.open("", "_blank");
  if (!win) return false;
  win.opener = null;
  win.document.write(
    `<!doctype html><html><head><meta charset="utf-8"><title>Kartu ${escapeHtml(p.noRm)}</title>` +
      `<style>@page{margin:8mm}body{margin:0;font-family:"Times New Roman",serif;color:#000}` +
      `.card{display:inline-block;padding:4px 6px;text-align:center}.name{font-size:10px;margin-bottom:3px;text-align:left}` +
      `.no{font-family:"Courier New",monospace;font-size:11px;font-weight:bold;margin-top:2px;letter-spacing:1px}</style></head><body>` +
      `<div class="card"><div class="name">${escapeHtml(p.nama)}</div>${code128Svg(p.noRm)}<div class="no">${escapeHtml(p.noRm)}</div></div>` +
      `<script>window.onload=function(){window.print()}</script></body></html>`,
  );
  win.document.close();
  win.focus();
  return true;
}

/** Master Data Klinik > Pasien: ~430k patients, paged on the server, limited to the user's branches. */
export default function PasienPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<PasienOptions | null>(null);
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [page, setPage] = useState<{ rows: PasienRow[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Pasien | "new" | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "info" | "error"; text: string } | null>(null);
  const lastQuery = useRef<RemoteQuery | null>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .pasienOptions(token)
      .then((o) => {
        setOptions(o);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page) => ({
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      lastQuery.current = q;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pasien(token, params(q))
        .then((res) => {
          if (mine !== seq.current) return; // a newer request is on its way
          setPage({ rows: res.rows, total: res.total, unfilteredTotal: res.unfilteredTotal });
        })
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof Error ? err.message : "Failed to load patients" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, branch], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const refresh = () => lastQuery.current && fetchPage(lastQuery.current);

  const fetchAll = async (q: RemoteQuery): Promise<PasienRow[]> => {
    if (!token) return [];
    const res = await api.pasien(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} matching patients. Narrow the search to export the rest.`,
      });
    }
    return res.rows;
  };

  const openEdit = async (row: PasienRow) => {
    if (!token) return;
    try {
      setEditing(await api.pasienDetail(token, row.id));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) expired();
      else setNotice({ kind: "error", text: err instanceof Error ? err.message : "Failed to load the patient" });
    }
  };

  const columns: Column<PasienRow>[] = [
    { key: "id", header: "Kode", value: (p) => String(p.id) },
    { key: "noRm", header: "No RM", value: (p) => p.noRm },
    { key: "nama", header: "Nama Pasien", value: (p) => p.nama, className: "dt-wrap" },
    { key: "tmptLahir", header: "Tempat Lahir", value: (p) => orDash(p.tmptLahir) },
    { key: "tglLahir", header: "Tanggal Lahir", value: (p) => fmtDate(p.tglLahir) },
    { key: "umur", header: "Umur", value: (p) => String(age(p.tglLahir) ?? "-") },
    { key: "alamat", header: "Alamat", value: (p) => orDash(p.alamat), className: "dt-wrap" },
    { key: "rt", header: "RT", value: (p) => orDash(p.rt) },
    { key: "rw", header: "RW", value: (p) => orDash(p.rw) },
    { key: "cabang", header: "Cabang", value: (p) => branchName.get(p.cabang) ?? p.cabang },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (p) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit ${p.noRm}`} title="Edit" onClick={() => openEdit(p)}>
            <Pencil aria-hidden="true" />
          </button>
          <button
            type="button"
            className="row-action"
            aria-label={`Print card ${p.noRm}`}
            title="Print card"
            onClick={() => {
              if (!printCard(p)) setNotice({ kind: "error", text: "The browser blocked the print window. Allow pop-ups for this site." });
            }}
          >
            <Barcode aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  const branches = options?.branches ?? [];
  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pasien</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!options || branches.length === 0}>
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
      ) : !options ? (
        <p className="muted">Loading…</p>
      ) : branches.length === 0 ? (
        <NoBranchNotice what="patients" />
      ) : (
        <DataTable
          // A new branch filter starts again from page 1.
          key={branch}
          title="Pasien"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(p) => p.id}
          initialSort={{ key: "id", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{
            total: page?.total ?? 0,
            unfilteredTotal: page?.unfilteredTotal ?? 0,
            loading: loading || !page,
            onQuery: fetchPage,
            fetchAll,
          }}
        />
      )}
      {editing && options && (
        <PasienForm
          pasien={editing === "new" ? null : editing}
          options={options}
          defaultBranch={branch === ALL_BRANCHES ? branches[0]?.kode ?? "" : branch}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(p, isNew) => {
            setEditing(null);
            setNotice({ kind: "success", text: `Pasien ${p.nama} ${isNew ? `registered with No RM ${p.noRm}` : "updated"}.` });
            refresh();
          }}
        />
      )}
    </section>
  );
}

// ---------- form ----------

function toInput(p: Pasien | null, cabang: string): PasienInput {
  return {
    nama: p?.nama ?? "",
    tmptLahir: p?.tmptLahir ?? "",
    tglLahir: p?.tglLahir ?? null,
    alamat: p?.alamat ?? "",
    idKel: p?.idKel ?? null,
    rt: p?.rt ?? "",
    rw: p?.rw ?? "",
    telp: p?.telp ?? "",
    hp: p?.hp ?? "",
    idGender: p?.idGender ?? null,
    idGolDarah: p?.idGolDarah ?? null,
    idAgama: p?.idAgama ?? null,
    idStatusKwn: p?.idStatusKwn ?? null,
    tglKun: p?.tglKun ?? today(),
    penanggung: p?.penanggung ?? "",
    idJamkes: p?.idJamkes ?? null,
    noJamkes: p?.noJamkes ?? "",
    cabang: p?.cabang || cabang,
    nik: p?.nik ?? "",
    alergi: p?.alergi ?? "",
  };
}

type TextKey = "nama" | "tmptLahir" | "alamat" | "rt" | "rw" | "telp" | "hp" | "penanggung" | "noJamkes" | "nik";
type LookupKey = "idGender" | "idGolDarah" | "idAgama" | "idStatusKwn" | "idJamkes";

function PasienForm({
  pasien,
  options,
  defaultBranch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  pasien: Pasien | null;
  options: PasienOptions;
  defaultBranch: string;
  onClose: () => void;
  onSaved: (p: Pasien, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<PasienInput>(() => toInput(pasien, defaultBranch));
  const [region, setRegion] = useState({
    kelurahan: pasien?.kelurahan ?? "",
    kecamatan: pasien?.kecamatan ?? "",
    kabupaten: pasien?.kabupaten ?? "",
    provinsi: pasien?.provinsi ?? "",
  });
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const update = <K extends keyof PasienInput>(key: K, value: PasienInput[K]) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const local: FieldErrors = {};
    if (!form.nama.trim()) local.nama = "Nama is required";
    if (!form.tmptLahir.trim()) local.tmptLahir = "Tempat Lahir is required";
    if (!form.tglLahir) local.tglLahir = "Tanggal Lahir is required";
    if (!form.alamat.trim()) local.alamat = "Alamat is required";
    if (form.idGender === null) local.idGender = "Jenis Kelamin is required";
    if (!form.cabang) local.cabang = "Cabang is required";
    setErrors(local);
    setError("");
    if (Object.keys(local).length) return;
    setBusy(true);
    try {
      const saved = pasien ? await api.updatePasien(token, pasien.id, form) : await api.createPasien(token, form);
      onSaved(saved, !pasien);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const text = (key: TextKey, label: string, max: number, opts: { required?: boolean; span?: number } = {}) => (
    <div className={opts.span ? `span-${opts.span}` : undefined}>
      <FieldShell id={`pasien-${key}`} label={label} error={errors[key]} required={opts.required}>
        <input
          id={`pasien-${key}`}
          value={form[key]}
          maxLength={max}
          aria-invalid={errors[key] ? "true" : undefined}
          onChange={(e) => update(key, e.target.value)}
        />
      </FieldShell>
    </div>
  );

  const lookup = (key: LookupKey, label: string, list: PasienOption[], required = false) => {
    const value = form[key];
    const missing = value !== null && !list.some((o) => o.id === value);
    return (
      <FieldShell id={`pasien-${key}`} label={label} error={errors[key]} required={required}>
        <select
          id={`pasien-${key}`}
          className="select"
          value={value ?? ""}
          aria-invalid={errors[key] ? "true" : undefined}
          onChange={(e) => update(key, e.target.value === "" ? null : Number(e.target.value))}
        >
          <option value="">-</option>
          {missing && <option value={value!}>#{value} (not in the list)</option>}
          {list.map((o) => (
            <option key={o.id} value={o.id}>
              {o.name}
              {o.isActive ? "" : " (non aktif)"}
            </option>
          ))}
        </select>
      </FieldShell>
    );
  };

  const date = (key: "tglLahir" | "tglKun", label: string, required = false) => (
    <FieldShell id={`pasien-${key}`} label={label} error={errors[key]} required={required}>
      <input
        id={`pasien-${key}`}
        type="date"
        max={today()}
        value={form[key] ?? ""}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => update(key, e.target.value || null)}
      />
    </FieldShell>
  );

  const readOnly = (id: string, label: string, value: string) => (
    <FieldShell id={id} label={label}>
      <input id={id} value={value} readOnly tabIndex={-1} className="readonly" />
    </FieldShell>
  );

  return (
    <Modal dialogRef={dialogRef} title={pasien ? "Edit Pasien" : "Add Pasien"} wide onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid form-grid-5">
        {readOnly("pasien-kode", "Kode", pasien ? String(pasien.id) : "(auto)")}
        {readOnly("pasien-norm", "No RM", pasien ? pasien.noRm : "(auto)")}
        {text("nama", "Nama", 50, { required: true })}
        {text("tmptLahir", "Tempat Lahir", 30, { required: true })}
        {date("tglLahir", "Tanggal Lahir", true)}

        {text("alamat", "Alamat", 100, { required: true, span: 2 })}
        <KelurahanPicker
          value={region.kelurahan}
          error={errors.idKel}
          onPick={(k) => {
            update("idKel", k?.id ?? null);
            setRegion(k ?? { kelurahan: "", kecamatan: "", kabupaten: "", provinsi: "" });
          }}
        />
        {readOnly("pasien-kecamatan", "Kecamatan", region.kecamatan)}
        {readOnly("pasien-kabupaten", "Kab / Kota", region.kabupaten)}

        {readOnly("pasien-provinsi", "Provinsi", region.provinsi)}
        {text("rt", "RT", 4)}
        {text("rw", "RW", 4)}
        {text("telp", "Telpon", 20)}
        {text("hp", "Hp", 50)}

        {lookup("idGender", "Jenis Kelamin", options.gender, true)}
        {lookup("idAgama", "Agama", options.agama)}
        {lookup("idGolDarah", "Gol Darah", options.golDarah)}
        {lookup("idStatusKwn", "Status Nikah", options.statusKwn)}
        {date("tglKun", "Tanggal Kun. Pertama")}

        {text("penanggung", "Penanggung / Ortu", 50)}
        {lookup("idJamkes", "Jaminan Kesehatan", options.jamkes)}
        {text("noJamkes", "Nomor Kartu Kesehatan", 100)}
        <FieldShell id="pasien-cabang" label="Cabang" error={errors.cabang} required>
          <select id="pasien-cabang" className="select" value={form.cabang} onChange={(e) => update("cabang", e.target.value)}>
            {options.branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.nama || b.kode}
              </option>
            ))}
          </select>
        </FieldShell>
        {text("nik", "NIK/KTP", 30)}

        <div className="span-5">
          <FieldShell id="pasien-alergi" label="Alergi" error={errors.alergi}>
            <textarea id="pasien-alergi" rows={2} maxLength={2000} value={form.alergi} onChange={(e) => update("alergi", e.target.value)} />
          </FieldShell>
        </div>
      </div>
    </Modal>
  );
}

/** Kelurahan search: type 2+ letters, pick a result; fills kecamatan, kab/kota and provinsi. */
function KelurahanPicker({ value, error, onPick }: { value: string; error?: string; onPick: (k: KelurahanHit | null) => void }) {
  const { token } = useAuth();
  const [query, setQuery] = useState(value);
  const [hits, setHits] = useState<KelurahanHit[]>([]);
  const [open, setOpen] = useState(false);
  const [active, setActive] = useState(0);
  const boxRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open || !token || query.trim().length < 2) {
      setHits([]);
      return;
    }
    let cancelled = false;
    const t = setTimeout(() => {
      api
        .searchKelurahan(token, query.trim())
        .then((r) => !cancelled && (setHits(r.kelurahan), setActive(0)))
        .catch(() => !cancelled && setHits([]));
    }, 250);
    return () => {
      cancelled = true;
      clearTimeout(t);
    };
  }, [query, open, token]);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => !boxRef.current?.contains(e.target as Node) && setOpen(false);
    document.addEventListener("mousedown", onDown);
    return () => document.removeEventListener("mousedown", onDown);
  }, [open]);

  const pick = (k: KelurahanHit) => {
    onPick(k);
    setQuery(k.kelurahan);
    setOpen(false);
  };

  return (
    <div className="field kel-picker" ref={boxRef}>
      <label htmlFor="pasien-kelurahan">Kelurahan</label>
      <div className="input-with-btn">
        <input
          id="pasien-kelurahan"
          role="combobox"
          aria-expanded={open && hits.length > 0}
          aria-controls="kel-results"
          aria-autocomplete="list"
          aria-invalid={error ? "true" : undefined}
          placeholder="Type to search"
          value={query}
          autoComplete="off"
          onChange={(e) => {
            setQuery(e.target.value);
            setOpen(true);
            if (!e.target.value) onPick(null);
          }}
          onFocus={() => setOpen(true)}
          onKeyDown={(e) => {
            if (!open || hits.length === 0) return;
            if (e.key === "ArrowDown") (e.preventDefault(), setActive((a) => Math.min(a + 1, hits.length - 1)));
            if (e.key === "ArrowUp") (e.preventDefault(), setActive((a) => Math.max(a - 1, 0)));
            if (e.key === "Enter") (e.preventDefault(), pick(hits[active]));
            if (e.key === "Escape") (e.stopPropagation(), setOpen(false));
          }}
        />
        <Search aria-hidden="true" className="kel-icon" />
      </div>
      {open && hits.length > 0 && (
        <ul id="kel-results" className="kel-results" role="listbox">
          {hits.map((k, i) => (
            <li
              key={k.id}
              role="option"
              aria-selected={i === active}
              className={i === active ? "active" : undefined}
              onMouseDown={(e) => (e.preventDefault(), pick(k))}
              onMouseEnter={() => setActive(i)}
            >
              <strong>{k.kelurahan}</strong>
              <span>
                {[k.kecamatan, k.kabupaten, k.provinsi].filter(Boolean).join(", ")}
              </span>
            </li>
          ))}
        </ul>
      )}
      {error && <span className="field-error">{error}</span>}
    </div>
  );
}
