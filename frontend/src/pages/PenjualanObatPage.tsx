import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, PengobatanStatus, PenjualanObat, PenjualanObatOptions, PenjualanObatQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

const STATUS_LABEL: Record<PengobatanStatus, string> = { open: "Open", close: "Close", cancel: "Cancel" };

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; jenis: string; status: PengobatanStatus | "" };
const NO_CARI: Cari = { nama: "", jenis: "", status: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/**
 * Report Klinik > Penjualan Obat (legacy /PenjualanObat): the drugs sold per
 * day at the user's branches, one row per drug, price, branch and status with
 * Banyak summed, by drug name then day.
 */
export default function PenjualanObatPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<PenjualanObatOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: PenjualanObat[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .penjualanObatOptions(token)
      .then((o) => {
        setOptions(o);
        // Like the legacy report: yesterday and today.
        const d = new Date(`${o.today}T00:00:00Z`);
        d.setUTCDate(d.getUTCDate() - 1);
        setFrom(d.toISOString().slice(0, 10));
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): PenjualanObatQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    jenis: cari.jenis ? Number(cari.jenis) : undefined,
    status: cari.status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .penjualanObat(token, params(q))
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

  const fetchAll = async (q: RemoteQuery): Promise<PenjualanObat[]> => {
    if (!token) return [];
    const res = await api.penjualanObat(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<PenjualanObat>[] = [
    { key: "tgl", header: "Tanggal", value: (r) => r.tgl },
    { key: "obat", header: "Obat", value: (r) => r.obat, className: "dt-wrap" },
    { key: "jenis", header: "Jenis", value: (r) => r.jenis || "-" },
    { key: "satuan", header: "Satuan", value: (r) => r.satuan || "-" },
    { key: "harga", header: "Harga", value: (r) => plain.format(r.harga), sortValue: (r) => r.harga, className: "dt-num" },
    { key: "byk", header: "Banyak", value: (r) => plain.format(r.byk), sortValue: (r) => r.byk, className: "dt-num" },
    // Not on the legacy report; shown under "Column visibility" and in exports when visible.
    { key: "jumlah", header: "Jumlah", value: (r) => plain.format(r.jumlah), sortValue: (r) => r.jumlah, className: "dt-num", hidden: true },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    {
      key: "status",
      header: "Status",
      value: (r) => STATUS_LABEL[r.status],
      render: (r) => <span className={`sale-status ${r.status}`}>{STATUS_LABEL[r.status]}</span>,
    },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.jenis ? 1 : 0) + (cari.status ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Penjualan Obat</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="penjualan-obat-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="penjualan-obat-cari"
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
            <FieldShell id="cari-obat" label="Nama Obat">
              <input id="cari-obat" value={draft.nama} placeholder="Input nama obat" onChange={(e) => setDraft({ ...draft, nama: e.target.value })} />
            </FieldShell>
            <FieldShell id="cari-jenis" label="Jenis Obat">
              <select id="cari-jenis" className="select" value={draft.jenis} onChange={(e) => setDraft({ ...draft, jenis: e.target.value })}>
                <option value="">Semua jenis</option>
                {options.jenis.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <FieldShell id="cari-status" label="Status">
              <select id="cari-status" className="select" value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value as PengobatanStatus | "" })}>
                <option value="">Semua status</option>
                {(Object.keys(STATUS_LABEL) as PengobatanStatus[]).map((s) => (
                  <option key={s} value={s}>
                    {STATUS_LABEL[s]}
                  </option>
                ))}
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search drug sales" title="Search">
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
        <NoBranchNotice what="drug sales" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.jenis}|${cari.status}`}
          title={`Penjualan Obat ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => `${r.tgl}|${r.idObat}|${r.harga}|${r.cabang}|${r.status}`}
          initialSort={{ key: "obat", dir: "asc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
