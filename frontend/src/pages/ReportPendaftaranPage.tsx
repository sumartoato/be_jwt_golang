import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, ReportPendaftaran, ReportPendaftaranOptions, ReportPendaftaranQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";

const EXPORT_LIMIT = 5000;

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; pelayanan: string; statusKunjungan: string };
const NO_CARI: Cari = { nama: "", pelayanan: "", statusKunjungan: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > Pendaftaran (legacy /ReportPendaftaranKlinik): the
 * registrations at the user's branches, newest first, with the queue number,
 * service, visit status and patient.
 */
export default function ReportPendaftaranPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<ReportPendaftaranOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: ReportPendaftaran[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .reportPendaftaranOptions(token)
      .then((o) => {
        setOptions(o);
        // Default: yesterday and today (H-1 to H).
        setFrom(dayBefore(o.today));
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): ReportPendaftaranQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    pelayanan: cari.pelayanan ? Number(cari.pelayanan) : undefined,
    statusKunjungan: cari.statusKunjungan ? Number(cari.statusKunjungan) : undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .reportPendaftaran(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the report" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, cari], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<ReportPendaftaran[]> => {
    if (!token) return [];
    const res = await api.reportPendaftaran(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} registrations. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<ReportPendaftaran>[] = [
    { key: "id", header: "No Pendaftaran", value: (r) => String(r.id), sortValue: (r) => r.id },
    { key: "noAntri", header: "No Antrian", value: (r) => (r.noAntri === null ? "-" : String(r.noAntri)), sortValue: (r) => r.noAntri },
    { key: "tgl", header: "Tanggal / Waktu", value: (r) => r.tgl ?? "-" },
    { key: "pelayanan", header: "Pelayanan", value: (r) => r.pelayanan || "-" },
    { key: "statusKun", header: "Status Kun", value: (r) => r.statusKun || "-" },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "pasien", header: "Nama Pasien", value: (r) => r.pasien || "(pasien tidak ditemukan)", className: "dt-wrap-sm" },
    { key: "umur", header: "Umur", value: (r) => (r.umur === null ? "-" : String(r.umur)), sortValue: (r) => r.umur, className: "dt-num" },
    { key: "alamat", header: "Alamat", value: (r) => r.alamat || "-", className: "dt-wrap" },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.pelayanan ? 1 : 0) + (cari.statusKunjungan ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pendaftaran</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal pendaftaran">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="report-pendaftaran-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="report-pendaftaran-cari"
          className="search-panel"
          aria-label="Pencarian"
          onSubmit={(e) => {
            e.preventDefault();
            applyCari();
          }}
        >
          <h3>
            <Search aria-hidden="true" /> Pencarian
          </h3>
          <div className="search-panel-row cols-3">
            <FieldShell id="cari-nama" label="Nama Pasien">
              <input id="cari-nama" value={draft.nama} placeholder="Input nama pasien" onChange={(e) => setDraft({ ...draft, nama: e.target.value })} />
            </FieldShell>
            <FieldShell id="cari-pelayanan" label="Pelayanan">
              <select id="cari-pelayanan" className="select" value={draft.pelayanan} onChange={(e) => setDraft({ ...draft, pelayanan: e.target.value })}>
                <option value="">Semua pelayanan</option>
                {options.pelayanan.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <FieldShell id="cari-status" label="Status Kunjungan">
              <select id="cari-status" className="select" value={draft.statusKunjungan} onChange={(e) => setDraft({ ...draft, statusKunjungan: e.target.value })}>
                <option value="">Semua status</option>
                {options.statusKunjungan.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search registrations" title="Search">
                <Search aria-hidden="true" />
              </button>
              <button type="button" className="icon-round" aria-label="Reset search" title="Reset" onClick={resetCari}>
                <RotateCcw aria-hidden="true" />
              </button>
            </div>
          </div>
        </form>
      )}
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
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.pelayanan}|${cari.statusKunjungan}`}
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
    </section>
  );
}
