import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, PengobatanStatus, TransaksiHarian, TransaksiHarianOptions, TransaksiHarianQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";
import { fmtDate } from "../components/dates";

const EXPORT_LIMIT = 5000;
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const STATUS_LABEL: Record<PengobatanStatus, string> = { open: "Open", close: "Close", cancel: "Cancel" };

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; pembayaran: string; status: PengobatanStatus | "" };
const NO_CARI: Cari = { nama: "", pembayaran: "", status: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > Transaksi Harian (legacy /ReportHarian): every sale at the
 * user's branches, newest first, with its cost per group (Obat, Tindakan,
 * Penunjang), total and discount. Pelayanan sits in the toolbar, as in the
 * legacy screen.
 */
export default function TransaksiHarianPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<TransaksiHarianOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [pelayanan, setPelayanan] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: TransaksiHarian[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .transaksiHarianOptions(token)
      .then((o) => {
        setOptions(o);
        // Like the legacy report: yesterday and today.
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
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): TransaksiHarianQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    pelayanan: pelayanan ? Number(pelayanan) : undefined,
    nama: cari.nama || undefined,
    pembayaran: cari.pembayaran ? Number(cari.pembayaran) : undefined,
    status: cari.status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .transaksiHarian(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the report" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, pelayanan, cari], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<TransaksiHarian[]> => {
    if (!token) return [];
    const res = await api.transaksiHarian(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} sales. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const amount = (key: "obat" | "tindakan" | "penunjang" | "total" | "diskon", header: string): Column<TransaksiHarian> => ({
    key,
    header,
    value: (r) => money.format(r[key]),
    sortValue: (r) => r[key],
    className: "dt-num",
  });
  const columns: Column<TransaksiHarian>[] = [
    { key: "tgl", header: "Tanggal", value: (r) => fmtDate(r.tgl), sortValue: (r) => r.tgl },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "pasien", header: "Pasien", value: (r) => r.pasien || "(pasien tidak ditemukan)", className: "dt-wrap-sm" },
    { key: "pelayanan", header: "Pelayanan", value: (r) => r.pelayanan || "-" },
    { key: "dokter", header: "Dokter", value: (r) => r.dokter || "-", className: "dt-wrap-sm" },
    { key: "pembayaran", header: "Pembayaran", value: (r) => r.pembayaran || "-" },
    amount("obat", "Obat"),
    amount("tindakan", "Tindakan"),
    amount("penunjang", "Penunjang"),
    amount("total", "Total"),
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    {
      key: "status",
      header: "Status",
      value: (r) => STATUS_LABEL[r.status],
      render: (r) => <span className={`sale-status ${r.status}`}>{STATUS_LABEL[r.status]}</span>,
    },
    amount("diskon", "Diskon"),
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.pembayaran ? 1 : 0) + (cari.status ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Transaksi Harian</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          {options && (
            <label className="inline-filter">
              <span>Pelayanan</span>
              <select className="select" aria-label="Pelayanan" value={pelayanan} onChange={(e) => setPelayanan(e.target.value)}>
                <option value="">ALL</option>
                {options.pelayanan.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="transaksi-harian-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="transaksi-harian-cari"
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
            <FieldShell id="cari-pembayaran" label="Pembayaran">
              <select id="cari-pembayaran" className="select" value={draft.pembayaran} onChange={(e) => setDraft({ ...draft, pembayaran: e.target.value })}>
                <option value="">Semua pembayaran</option>
                {options.pembayaran.map((o) => (
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
              <button type="submit" className="icon-round" aria-label="Search sales" title="Search">
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
        <NoBranchNotice what="sales" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${pelayanan}|${cari.nama}|${cari.pembayaran}|${cari.status}`}
          title={`Transaksi Harian ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.idJual}
          initialSort={{ key: "tgl", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
