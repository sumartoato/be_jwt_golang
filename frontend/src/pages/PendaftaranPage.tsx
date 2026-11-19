import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, Pencil, Plus, RotateCcw, Search, SlidersHorizontal, X } from "lucide-react";
import { api, ApiError, Pendaftaran, PendaftaranOptions, PendaftaranPasien, PasienSearchField, STATUS_DAFTAR } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice, orDash } from "../components/branchView";
import { age, fmtDate } from "../components/dates";
import { FieldShell, Modal, useModal } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

const PASIEN_FIELDS: { value: PasienSearchField; label: string; placeholder: string }[] = [
  { value: "norm", label: "No RM", placeholder: "e.g. 33-01-03" },
  { value: "nama", label: "Nama", placeholder: "Patient name" },
  { value: "umur", label: "Umur", placeholder: "Age in years, e.g. 13" },
  { value: "alamat", label: "Alamat", placeholder: "Address" },
  { value: "jamkes", label: "Jam Kes", placeholder: "Insurer, e.g. BPJS" },
];

type Notice = { kind: "success" | "info" | "error"; text: string } | null;

/** Front Office Klinik > Pendaftaran Pasien: visit registrations at the user's branches. */
export default function PendaftaranPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<PendaftaranOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [pelayanan, setPelayanan] = useState("");
  const [status, setStatus] = useState("");
  const [filterOpen, setFilterOpen] = useState(false);
  const [page, setPage] = useState<{ rows: Pendaftaran[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<Pendaftaran | null>(null);
  const [notice, setNotice] = useState<Notice>(null);
  const lastQuery = useRef<RemoteQuery | null>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .pendaftaranOptions(token)
      .then((o) => {
        setOptions(o);
        setFrom(o.today);
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page) => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    pelayanan: pelayanan ? Number(pelayanan) : undefined,
    status: status ? Number(status) : undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      lastQuery.current = q;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pendaftaran(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load registrations" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, pelayanan, status], // eslint-disable-line react-hooks/exhaustive-deps
  );
  const refresh = () => lastQuery.current && fetchPage(lastQuery.current);

  const fetchAll = async (q: RemoteQuery): Promise<Pendaftaran[]> => {
    if (!token) return [];
    const res = await api.pendaftaran(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} registrations. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const columns: Column<Pendaftaran>[] = [
    { key: "id", header: "No Pendaftaran", value: (r) => String(r.id) },
    { key: "noAntri", header: "No Antrian", value: (r) => (r.noAntri === null ? "-" : String(r.noAntri)) },
    { key: "noAntriPoli", header: "No Antrian Poli", value: (r) => r.noAntriPoli ?? "" },
    {
      key: "tglMasuk",
      header: "Tanggal Waktu",
      value: (r) => r.tglMasuk ?? "-",
      // Date and time on two lines, like the original screen.
      render: (r) =>
        r.tglMasuk ? (
          <span className="dt-datetime">
            {r.tglMasuk.slice(0, 10)}
            <br />
            {r.tglMasuk.slice(11)}
          </span>
        ) : (
          "-"
        ),
    },
    { key: "pelayanan", header: "Pelayanan", value: (r) => r.pelayanan },
    {
      key: "status",
      header: "Status Kunjungan",
      value: (r) => r.status,
      render: (r) => (
        <span className={`visit-status s${r.idStatus ?? 0}`} title={r.cancelReason ? `${r.cancelReason} (${r.cancelBy}, ${r.cancelAt})` : undefined}>
          {r.status}
        </span>
      ),
    },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "nama", header: "Nama Pasien", value: (r) => r.nama || "(pasien tidak ditemukan)", className: "dt-wrap" },
    { key: "tglLahir", header: "Tanggal Lahir", value: (r) => fmtDate(r.tglLahir) },
    { key: "umur", header: "Umur", value: (r) => String(age(r.tglLahir) ?? "-") },
    { key: "alamat", header: "Alamat", value: (r) => orDash(r.alamat), className: "dt-wrap" },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap" },
    { key: "pakaiKartu", header: "Pakai Kartu", value: (r) => (r.pakaiKartu ? "Ya" : "Tidak") },
    { key: "totalPoint", header: "Total Point", value: (r) => (r.totalPoint === null ? "0" : num.format(r.totalPoint)) },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => (
        <button type="button" className="row-action" aria-label={`Edit pendaftaran ${r.id}`} title="Edit" onClick={() => setEditing(r)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  const branches = options?.branches ?? [];
  const filterCount = (pelayanan ? 1 : 0) + (status ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pendaftaran Pasien</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal pendaftaran">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <div className="filter-menu">
            <button type="button" className="btn-add" aria-expanded={filterOpen} onClick={() => setFilterOpen((o) => !o)}>
              <SlidersHorizontal aria-hidden="true" /> Filter{filterCount ? ` (${filterCount})` : ""}
            </button>
            {filterOpen && options && (
              <div className="filter-pop" role="group" aria-label="Filter">
                <FieldShell id="filter-pelayanan" label="Pelayanan">
                  <select id="filter-pelayanan" className="select" value={pelayanan} onChange={(e) => setPelayanan(e.target.value)}>
                    <option value="">Semua pelayanan</option>
                    {options.pelayanan.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </FieldShell>
                <FieldShell id="filter-status" label="Status Kunjungan">
                  <select id="filter-status" className="select" value={status} onChange={(e) => setStatus(e.target.value)}>
                    <option value="">Semua status</option>
                    {options.status.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                </FieldShell>
                <div className="form-actions">
                  <button type="button" className="btn-small" onClick={() => (setPelayanan(""), setStatus(""))}>
                    Reset
                  </button>
                  <button type="button" className="btn-small btn-primary" onClick={() => setFilterOpen(false)}>
                    Close
                  </button>
                </div>
              </div>
            )}
          </div>
          <button type="button" className="btn-add" onClick={() => setAdding(true)} disabled={!options || branches.length === 0}>
            <Plus aria-hidden="true" /> Add
          </button>
        </div>
      </div>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice(null)}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !options ? (
        <p className="muted">Loading…</p>
      ) : branches.length === 0 ? (
        <NoBranchNotice what="registrations" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${pelayanan}|${status}`}
          title={`Pendaftaran ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.id}
          initialSort={{ key: "id", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
      {adding && options && (
        <AddDialog
          options={options}
          defaultBranch={branch === ALL_BRANCHES ? (branches.length === 1 ? branches[0].kode : "") : branch}
          onClose={() => setAdding(false)}
          onUnauthorized={expired}
          onSaved={(r) => {
            setAdding(false);
            setNotice({
              kind: "success",
              text: `Pendaftaran ${r.id}: ${r.nama} · ${r.pelayanan} · No Antrian ${r.noAntri}${r.noAntriPoli ? ` · Antrian Poli ${r.noAntriPoli}` : ""}.`,
            });
            refresh();
          }}
        />
      )}
      {editing && options && (
        <EditDialog
          row={editing}
          options={options}
          branchName={branchName.get(editing.cabang) ?? editing.cabang}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(r, text) => {
            setEditing(null);
            setNotice({ kind: "success", text: `${text} (No Pendaftaran ${r.id}).` });
            refresh();
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

// ---------- new registration ----------

function AddDialog({
  options,
  defaultBranch,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  options: PendaftaranOptions;
  defaultBranch: string;
  onClose: () => void;
  onSaved: (r: Pendaftaran) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [tanggal, setTanggal] = useState(options.today);
  const [idJenis, setIdJenis] = useState("");
  const [cabang, setCabang] = useState(defaultBranch);
  const [pakaiKartu, setPakaiKartu] = useState(false);
  const [pasien, setPasien] = useState<PendaftaranPasien | null>(null);
  const [hits, setHits] = useState<{ rows: PendaftaranPasien[]; total: number; unfilteredTotal: number } | null>(null);
  const [hitsLoading, setHitsLoading] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const seq = useRef(0);
  // "Pencarian" panel: the field and key being typed, and the pair last applied to the list.
  const [searchOpen, setSearchOpen] = useState(false);
  const [by, setBy] = useState<PasienSearchField>("norm");
  const [key, setKey] = useState("");
  const [applied, setApplied] = useState<{ by: PasienSearchField; key: string } | null>(null);
  const branchName = new Map(options.branches.map((b) => [b.kode, b.nama]));

  const fetchPasien = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setHitsLoading(true);
      api
        .pendaftaranPasien(token, { q: q.search, by: applied?.by ?? "", key: applied?.key ?? "", page: q.page, size: q.pageSize })
        .then((res) => mine === seq.current && setHits(res))
        .catch((err) => mine === seq.current && (err instanceof ApiError && err.status === 401 ? onUnauthorized() : setError(errorText(err))))
        .finally(() => mine === seq.current && setHitsLoading(false));
    },
    [token, applied], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const applySearch = () => {
    const k = key.trim();
    if (by === "umur" && k && !/^\d{1,3}$/.test(k)) {
      setError("Umur must be a whole number of years.");
      return;
    }
    setError("");
    setApplied(k ? { by, key: k } : null);
  };
  const resetSearch = () => {
    setKey("");
    setApplied(null);
    setError("");
  };

  const choose = (p: PendaftaranPasien) => {
    setPasien((cur) => (cur?.id === p.id ? null : p));
    // Default the branch to the patient's own when the user has it.
    if (!cabang && options.branches.some((b) => b.kode === p.cabang)) setCabang(p.cabang);
    setError("");
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const missing = [!idJenis && "Pelayanan", !cabang && "Cabang", !pasien && "a patient (tick one in the list)"].filter(Boolean);
    if (missing.length) {
      setError(`Choose ${missing.join(", ")}.`);
      return;
    }
    setBusy(true);
    setError("");
    try {
      onSaved(await api.createPendaftaran(token, { tanggal, idPas: pasien!.id, idJenis: Number(idJenis), cabang, pakaiKartu }));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  }

  const columns: Column<PendaftaranPasien>[] = [
    { key: "noRm", header: "No RM", value: (p) => p.noRm, sortable: false },
    { key: "nama", header: "Nama Pasien", value: (p) => p.nama, className: "dt-wrap", sortable: false },
    { key: "tglLahir", header: "Tanggal Lahir", value: (p) => fmtDate(p.tglLahir), sortable: false },
    { key: "umur", header: "Umur", value: (p) => String(age(p.tglLahir) ?? "-"), sortable: false },
    { key: "alamat", header: "Alamat", value: (p) => orDash(p.alamat), className: "dt-wrap", sortable: false },
    { key: "nik", header: "NIK", value: (p) => orDash(p.nik), sortable: false },
    { key: "hp", header: "No Hp", value: (p) => orDash(p.hp), sortable: false },
    { key: "penanggung", header: "Penanggung", value: (p) => orDash(p.penanggung), className: "dt-wrap", sortable: false },
    { key: "jamkes", header: "Jaminan Kesehatan", value: (p) => orDash(p.jamkes), className: "dt-wrap", sortable: false },
    { key: "cabang", header: "Cabang", value: (p) => branchName.get(p.cabang) ?? p.cabang, className: "dt-wrap", sortable: false },
    {
      key: "pick",
      header: "Pilih",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (p) => (
        <input type="checkbox" checked={pasien?.id === p.id} aria-label={`Choose ${p.noRm}`} onChange={() => choose(p)} />
      ),
    },
  ];

  return (
    <Modal dialogRef={dialogRef} title="Add Pendaftaran Pasien" wide="xl" onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid form-grid-4 reg-head">
        <FieldShell id="reg-tanggal" label="Tanggal">
          <input id="reg-tanggal" type="date" value={tanggal} max={options.today} onChange={(e) => setTanggal(e.target.value || options.today)} />
        </FieldShell>
        <FieldShell id="reg-pelayanan" label="Pelayanan" required>
          <select id="reg-pelayanan" className="select" value={idJenis} onChange={(e) => setIdJenis(e.target.value)}>
            <option value="">-- Pilih Jenis --</option>
            {options.pelayanan.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="reg-cabang" label="Cabang" required>
          <select id="reg-cabang" className="select" value={cabang} onChange={(e) => setCabang(e.target.value)}>
            <option value="">-- Pilih Cabang --</option>
            {options.branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.nama || b.kode}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="reg-kartu" label="Pakai Kartu">
          <select id="reg-kartu" className="select" value={pakaiKartu ? "1" : "0"} onChange={(e) => setPakaiKartu(e.target.value === "1")}>
            <option value="0">Tidak</option>
            <option value="1">Ya</option>
          </select>
        </FieldShell>
        <div className="reg-head-tools">
          <button type="button" className="btn-add" aria-expanded={searchOpen} aria-controls="reg-search" onClick={() => setSearchOpen((o) => !o)}>
            Cari <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {searchOpen && (
        <section id="reg-search" className="search-panel" aria-label="Pencarian pasien">
          <h3>
            <Search aria-hidden="true" /> Pencarian
          </h3>
          <div className="search-panel-row">
            <FieldShell id="reg-search-by" label="Select">
              <select id="reg-search-by" className="select" value={by} onChange={(e) => setBy(e.target.value as PasienSearchField)}>
                {PASIEN_FIELDS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </FieldShell>
            <FieldShell id="reg-search-key" label="Key">
              <input
                id="reg-search-key"
                value={key}
                inputMode={by === "umur" ? "numeric" : undefined}
                placeholder={PASIEN_FIELDS.find((o) => o.value === by)?.placeholder}
                onChange={(e) => setKey(e.target.value)}
                onKeyDown={(e) => {
                  // Enter searches instead of submitting the registration.
                  if (e.key === "Enter") {
                    e.preventDefault();
                    applySearch();
                  }
                }}
              />
            </FieldShell>
            <div className="search-panel-actions">
              <button type="button" className="icon-round" aria-label="Search patients" title="Search" onClick={applySearch}>
                <Search aria-hidden="true" />
              </button>
              <button type="button" className="icon-round" aria-label="Reset patient search" title="Reset" onClick={resetSearch}>
                <RotateCcw aria-hidden="true" />
              </button>
            </div>
          </div>
        </section>
      )}
      <p className="selected-patient" aria-live="polite">
        {pasien ? (
          <>
            Pasien: <strong>{pasien.noRm}</strong> · {pasien.nama} · {fmtDate(pasien.tglLahir)}
          </>
        ) : (
          <span className="muted">Search and tick a patient below.</span>
        )}
      </p>
      <DataTable
        key={applied ? `${applied.by}:${applied.key}` : "all"}
        title="Pasien"
        columns={columns}
        rows={hits?.rows ?? []}
        rowKey={(p) => p.id}
        pageSizes={[5, 10, 20, 50]}
        defaultPageSize={5}
        exports={false}
        remote={{ total: hits?.total ?? 0, unfilteredTotal: hits?.unfilteredTotal ?? 0, loading: hitsLoading || !hits, onQuery: fetchPasien }}
      />
    </Modal>
  );
}

// ---------- edit / cancel ----------

function EditDialog({
  row,
  options,
  branchName,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  row: Pendaftaran;
  options: PendaftaranOptions;
  branchName: string;
  onClose: () => void;
  onSaved: (r: Pendaftaran, text: string) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const editable = row.idStatus === STATUS_DAFTAR;
  const [idJenis, setIdJenis] = useState(String(row.idJenis ?? ""));
  const [pakaiKartu, setPakaiKartu] = useState(row.pakaiKartu);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  // An inactive service the registration still has stays selectable.
  const services = options.pelayanan.some((s) => s.id === row.idJenis) || row.idJenis === null
    ? options.pelayanan
    : [{ id: row.idJenis, name: row.pelayanan, needQueue: false, firstCode: "", isExamination: false }, ...options.pelayanan];

  const run = async (fn: () => Promise<Pendaftaran>, text: string) => {
    setBusy(true);
    setError("");
    try {
      onSaved(await fn(), text);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token || !editable) return onClose();
    if (!idJenis) return setError("Choose a Pelayanan.");
    await run(() => api.updatePendaftaran(token, row.id, { idJenis: Number(idJenis), pakaiKartu }), "Pendaftaran updated");
  }

  const cancel = async () => {
    if (!token) return;
    if (!reason.trim()) return setError("Enter the reason for cancelling.");
    await run(() => api.cancelPendaftaran(token, row.id, reason.trim()), "Pendaftaran cancelled");
  };

  return (
    <Modal dialogRef={dialogRef} title={`Edit Pendaftaran ${row.id}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <dl className="reg-summary">
        <div><dt>Pasien</dt><dd>{row.noRm ? `${row.noRm} · ${row.nama}` : "(pasien tidak ditemukan)"}</dd></div>
        <div><dt>Tanggal</dt><dd>{row.tglMasuk ?? "-"}</dd></div>
        <div><dt>Cabang</dt><dd>{branchName}</dd></div>
        <div><dt>Antrian</dt><dd>{row.noAntri ?? "-"}{row.noAntriPoli ? ` · Poli ${row.noAntriPoli}` : ""}</dd></div>
        <div><dt>Status</dt><dd>{row.status}</dd></div>
        {row.cancelReason && <div><dt>Alasan batal</dt><dd>{row.cancelReason} ({row.cancelBy}, {row.cancelAt})</dd></div>}
      </dl>
      {!editable ? (
        <p className="alert info">Only registrations with status DAFTAR can be changed.</p>
      ) : (
        <>
          <div className="form-grid">
            <FieldShell id="edit-pelayanan" label="Pelayanan" hint="A different service gets a new poli queue number.">
              <select id="edit-pelayanan" className="select" value={idJenis} onChange={(e) => setIdJenis(e.target.value)}>
                {services.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <FieldShell id="edit-kartu" label="Pakai Kartu">
              <select id="edit-kartu" className="select" value={pakaiKartu ? "1" : "0"} onChange={(e) => setPakaiKartu(e.target.value === "1")}>
                <option value="0">Tidak</option>
                <option value="1">Ya</option>
              </select>
            </FieldShell>
          </div>
          <fieldset className="form-section cancel-box">
            <legend>Batalkan pendaftaran</legend>
            <div className="input-with-btn">
              <input aria-label="Alasan batal" placeholder="Alasan batal" maxLength={100} value={reason} onChange={(e) => setReason(e.target.value)} />
              <button type="button" className="btn-small btn-danger" disabled={busy} onClick={cancel}>
                Batalkan
              </button>
            </div>
          </fieldset>
        </>
      )}
    </Modal>
  );
}
