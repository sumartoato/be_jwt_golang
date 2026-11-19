import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router-dom";
import { CalendarDays, ChevronDown, Plus, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, Pengobatan, PengobatanOptions, PengobatanQuery, PengobatanStatus } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const STATUS_LABEL: Record<PengobatanStatus, string> = { open: "Open", close: "Close", cancel: "Cancel" };

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; pelayanan: string; statusKunjungan: string };
const NO_CARI: Cari = { nama: "", pelayanan: "", statusKunjungan: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/** Transaksi Klinik > Pengobatan: treatment sales at the user's branches, newest first. */
export default function PengobatanPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<PengobatanOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [filterOpen, setFilterOpen] = useState(false);
  const [pembayaran, setPembayaran] = useState("");
  const [status, setStatus] = useState<PengobatanStatus | "">("");
  const [page, setPage] = useState<{ rows: Pengobatan[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .pengobatanOptions(token)
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
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): PengobatanQuery => ({
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
    pembayaran: pembayaran ? Number(pembayaran) : undefined,
    status: status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pengobatan(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load treatments" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, cari, pembayaran, status], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<Pengobatan[]> => {
    if (!token) return [];
    const res = await api.pengobatan(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} treatments. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<Pengobatan>[] = [
    { key: "id", header: "No", value: (r) => String(r.id) },
    {
      key: "tgl",
      header: "Tanggal",
      value: (r) => r.tgl ?? "-",
      render: (r) =>
        r.tgl ? (
          <span className="dt-datetime">
            {r.tgl.slice(0, 10)}
            <br />
            {r.tgl.slice(11)}
          </span>
        ) : (
          "-"
        ),
    },
    { key: "noKwi", header: "No Kuitansi", value: (r) => r.noKwi || "-" },
    {
      key: "pasien",
      header: "Pasien",
      value: (r) => r.pasien || "(pasien tidak ditemukan)",
      className: "dt-wrap-sm",
      render: (r) => <span title={r.noRm ? `No RM ${r.noRm}` : undefined}>{r.pasien || "(pasien tidak ditemukan)"}</span>,
    },
    { key: "dokter", header: "Dokter", value: (r) => r.dokter || "-", className: "dt-wrap-sm" },
    { key: "jenisRawat", header: "Jenis Rawat", value: (r) => r.jenisRawat || "-" },
    { key: "pembayaran", header: "Pembayaran", value: (r) => r.pembayaran || "-" },
    {
      key: "total",
      header: "Total",
      value: (r) => money.format(r.total),
      sortValue: (r) => r.total,
      className: "dt-num",
      render: (r) => (
        <span
          title={`Obat ${money.format(r.totalObat)} · Tindakan ${money.format(r.totalTindakan)} · Penunjang ${money.format(r.totalPenunjang)}${r.potongan ? ` · Potongan ${money.format(r.potongan)}` : ""}`}
        >
          {money.format(r.total)}
        </span>
      ),
    },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    {
      key: "status",
      header: "Status",
      value: (r) => STATUS_LABEL[r.status],
      render: (r) => (
        <span className={`sale-status ${r.status}`} title={r.cancelReason ? `${r.cancelReason} (${r.cancelBy}, ${r.cancelAt})` : undefined}>
          {STATUS_LABEL[r.status]}
        </span>
      ),
    },
  ];

  const branches = options?.branches ?? [];
  const filterCount = (pembayaran ? 1 : 0) + (status ? 1 : 0);
  const cariCount = (cari.nama ? 1 : 0) + (cari.pelayanan ? 1 : 0) + (cari.statusKunjungan ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pengobatan</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="pengobatan-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
          <div className="filter-menu">
            <button type="button" className="btn-add" aria-expanded={filterOpen} onClick={() => setFilterOpen((o) => !o)}>
              Filter{filterCount ? ` (${filterCount})` : ""} <ChevronDown aria-hidden="true" />
            </button>
            {filterOpen && options && (
              <div className="filter-pop" role="group" aria-label="Filter">
                <FieldShell id="filter-pembayaran" label="Pembayaran">
                  <select id="filter-pembayaran" className="select" value={pembayaran} onChange={(e) => setPembayaran(e.target.value)}>
                    <option value="">Semua pembayaran</option>
                    {options.pembayaran.map((o) => (
                      <option key={o.id} value={o.id}>
                        {o.name}
                      </option>
                    ))}
                  </select>
                </FieldShell>
                <FieldShell id="filter-status" label="Status">
                  <select id="filter-status" className="select" value={status} onChange={(e) => setStatus(e.target.value as PengobatanStatus | "")}>
                    <option value="">Semua status</option>
                    {(Object.keys(STATUS_LABEL) as PengobatanStatus[]).map((s) => (
                      <option key={s} value={s}>
                        {STATUS_LABEL[s]}
                      </option>
                    ))}
                  </select>
                </FieldShell>
                <div className="form-actions">
                  <button type="button" className="btn-small" onClick={() => (setPembayaran(""), setStatus(""))}>
                    Reset
                  </button>
                  <button type="button" className="btn-small btn-primary" onClick={() => setFilterOpen(false)}>
                    Close
                  </button>
                </div>
              </div>
            )}
          </div>
          <Link to="/transaksi-klinik/pengobatan/new" className="btn-add btn-link">
            <Plus aria-hidden="true" /> Add
          </Link>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="pengobatan-cari"
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
            <FieldShell id="cari-pelayanan" label="Jenis Pelayanan">
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
              <button type="submit" className="icon-round" aria-label="Search treatments" title="Search">
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
        <NoBranchNotice what="treatments" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.pelayanan}|${cari.statusKunjungan}|${pembayaran}|${status}`}
          title={`Pengobatan ${from}${to !== from ? " - " + to : ""}`}
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
