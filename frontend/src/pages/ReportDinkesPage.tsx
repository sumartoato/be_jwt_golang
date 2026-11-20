import { useCallback, useEffect, useRef, useState } from "react";
import { CalendarDays, X } from "lucide-react";
import { api, ApiError, ReportDinkes, ReportDinkesQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { NoBranchNotice } from "../components/branchView";

const EXPORT_LIMIT = 5000;

type Notice = { kind: "info" | "error"; text: string } | null;
type Options = { branches: { kode: string; nama: string }[]; today: string };

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/** The age groups, as the legacy report labels them. */
const AGES: [keyof ReportDinkes, string][] = [
  ["hari0_7", "0-7 Hari"],
  ["hari8_28", "8-28 Hari"],
  ["bulan1_11", "1-11 Bulan"],
  ["tahun1_4", "1-4 Tahun"],
  ["tahun5_9", "5-9 Tahun"],
  ["tahun10_14", "10-14 Tahun"],
  ["tahun15_19", "15-19 Tahun"],
  ["tahun20_44", "20-44 Tahun"],
  ["tahun45_59", "45-59 Tahun"],
  ["tahun60", ">59 Tahun"],
];

/**
 * Report Klinik > Report ICD Dinkes (legacy /ReportDinkes): the cases of one
 * branch per ICD code and service, by age group and sex, for the health
 * office. The branch is chosen in the toolbar, as on the legacy screen.
 */
export default function ReportDinkesPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<Options | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [cabang, setCabang] = useState("");
  const [page, setPage] = useState<{ rows: ReportDinkes[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .reportDinkesOptions(token)
      .then((o) => {
        setOptions(o);
        // Default: yesterday and today (H-1 to H), the first of the user's branches.
        setFrom(dayBefore(o.today));
        setTo(o.today);
        setCabang(o.branches[0]?.kode ?? "");
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): ReportDinkesQuery => ({
    from,
    to,
    cabang,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from || !cabang) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .reportDinkes(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the report" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, cabang], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<ReportDinkes[]> => {
    if (!token) return [];
    const res = await api.reportDinkes(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const count = (key: keyof ReportDinkes, header: string): Column<ReportDinkes> => ({
    key,
    header,
    value: (r) => String(r[key]),
    sortValue: (r) => r[key] as number,
    className: "dt-num",
  });
  const columns: Column<ReportDinkes>[] = [
    { key: "kode", header: "Kode", value: (r) => r.kode || "-" },
    { key: "icd", header: "ICD", value: (r) => r.icd, className: "dt-wrap" },
    { key: "jenisRawat", header: "Jenis Rawat", value: (r) => r.jenisRawat || "-" },
    ...AGES.map(([k, h]) => count(k, h)),
    count("laki", "Laki-laki"),
    count("perempuan", "Perempuan"),
    count("total", "Total"),
  ];

  const branches = options?.branches ?? [];
  const branchName = branches.find((b) => b.kode === cabang)?.nama ?? cabang;

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Report ICD Dinkes</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          {branches.length > 0 && (
            <label className="inline-filter">
              <span>Cabang</span>
              <select className="select" aria-label="Cabang" value={cabang} onChange={(e) => setCabang(e.target.value)}>
                {branches.map((b) => (
                  <option key={b.kode} value={b.kode}>
                    {b.nama || b.kode}
                  </option>
                ))}
              </select>
            </label>
          )}
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
        <NoBranchNotice what="ICD cases" />
      ) : (
        <DataTable
          // A new period or branch starts again from page 1.
          key={`${from}|${to}|${cabang}`}
          title={`Report ICD Dinkes ${branchName} ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => `${r.kode}|${r.icd}|${r.jenisRawat}`}
          initialSort={{ key: "kode", dir: "desc" }}
          defaultPageSize={10}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
