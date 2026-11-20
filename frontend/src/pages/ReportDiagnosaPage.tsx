import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, ReportDiagnosa, ReportDiagnosaQuery, ReportPendapatanOptions } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";
import { fmtDate } from "../components/dates";

const EXPORT_LIMIT = 5000;

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; diagnosa: string; pelayanan: string };
const NO_CARI: Cari = { nama: "", diagnosa: "", pelayanan: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > Report Diagnosa (legacy /ReportDiagnosa): every diagnosis of
 * the sales at the user's branches, newest sale first, with the patient, the
 * prescription and the visit's main complaint. Saturasi Oksigen is shown as on
 * the legacy screen but is not stored in this database yet.
 */
export default function ReportDiagnosaPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<ReportPendapatanOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: ReportDiagnosa[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .reportDiagnosaOptions(token)
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
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): ReportDiagnosaQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    diagnosa: cari.diagnosa || undefined,
    pelayanan: cari.pelayanan ? Number(cari.pelayanan) : undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .reportDiagnosa(token, params(q))
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

  const fetchAll = async (q: RemoteQuery): Promise<ReportDiagnosa[]> => {
    if (!token) return [];
    const res = await api.reportDiagnosa(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} diagnoses. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim(), diagnosa: draft.diagnosa.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<ReportDiagnosa>[] = [
    { key: "idJual", header: "No Trans", value: (r) => String(r.idJual), sortValue: (r) => r.idJual },
    { key: "tgl", header: "Tanggal", value: (r) => r.tgl, className: "dt-wrap-sm" },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "pasien", header: "Pasien", value: (r) => r.pasien || "(pasien tidak ditemukan)", className: "dt-wrap-sm" },
    { key: "tglLahir", header: "Tanggal Lahir", value: (r) => fmtDate(r.tglLahir), sortValue: (r) => r.tglLahir },
    { key: "usia", header: "Usia", value: (r) => (r.usia === null ? "-" : String(r.usia)), sortValue: (r) => r.usia, className: "dt-num" },
    { key: "jenisKelamin", header: "Jenis Kelamin", value: (r) => r.jenisKelamin || "-" },
    { key: "dokter", header: "Dokter", value: (r) => r.dokter || "-", className: "dt-wrap-sm" },
    { key: "jenisRawat", header: "Jenis Rawat", value: (r) => r.jenisRawat || "-" },
    { key: "icd", header: "ICD.X", value: (r) => r.icd || "-" },
    { key: "diagnosa", header: "Diagnosa", value: (r) => r.diagnosa, className: "dt-wrap-sm" },
    { key: "resep", header: "Resep Obat", value: (r) => r.resep || "-", sortable: false, className: "dt-wrap" },
    { key: "keluhan", header: "Keluhan Utama", value: (r) => r.keluhan || "-", sortable: false, className: "dt-wrap-sm" },
    // Not stored in this database yet (the legacy screen added it later).
    { key: "saturasi", header: "Saturasi Oksigen", value: () => "-", sortable: false },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.diagnosa ? 1 : 0) + (cari.pelayanan ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Report Diagnosa</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="report-diagnosa-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="report-diagnosa-cari"
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
            <FieldShell id="cari-diagnosa" label="ICD.X / Diagnosa">
              <input id="cari-diagnosa" value={draft.diagnosa} placeholder="Kode atau nama diagnosa" onChange={(e) => setDraft({ ...draft, diagnosa: e.target.value })} />
            </FieldShell>
            <FieldShell id="cari-pelayanan" label="Jenis Rawat">
              <select id="cari-pelayanan" className="select" value={draft.pelayanan} onChange={(e) => setDraft({ ...draft, pelayanan: e.target.value })}>
                <option value="">Semua jenis rawat</option>
                {options.pelayanan.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search diagnoses" title="Search">
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
        <NoBranchNotice what="diagnoses" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.diagnosa}|${cari.pelayanan}`}
          title={`Report Diagnosa ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.id}
          initialSort={{ key: "idJual", dir: "desc" }}
          defaultPageSize={5}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
