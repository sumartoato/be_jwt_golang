import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { CalendarDays, Eye, Plus, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, PenerimaanBarang, PenerimaanBarangOptions, PenerimaanBarangQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";
import { fmtDate } from "../components/dates";

const EXPORT_LIMIT = 5000;
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { noGr: string; supplier: string };
const NO_CARI: Cari = { noGr: "", supplier: "" };
/** The supplier filter's value for receipts without a supplier (id_supp 0). */
const NO_SUPPLIER = "0";

type Notice = { kind: "info" | "error"; text: string } | null;

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Transaksi Klinik > Penerimaan Barang (legacy /GoodReceive): the goods
 * receipts at the user's branches, newest first. Add opens the receipt form;
 * View opens a saved receipt read-only (receipts are not edited).
 */
export default function PenerimaanBarangPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<PenerimaanBarangOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: PenerimaanBarang[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);
  const navigate = useNavigate();

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .penerimaanBarangOptions(token)
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
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): PenerimaanBarangQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    noGr: cari.noGr || undefined,
    supplier: cari.supplier === "" ? undefined : Number(cari.supplier),
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .penerimaanBarang(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load receipts" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, cari], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<PenerimaanBarang[]> => {
    if (!token) return [];
    const res = await api.penerimaanBarang(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} receipts. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, noGr: draft.noGr.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<PenerimaanBarang>[] = [
    { key: "id", header: "No", value: (r) => String(r.id), sortValue: (r) => r.id },
    { key: "noGr", header: "No GR", value: (r) => r.noGr || "-" },
    { key: "tglTrans", header: "Tgl Trans", value: (r) => fmtDate(r.tglTrans), sortValue: (r) => r.tglTrans },
    { key: "tglGr", header: "Tgl GR", value: (r) => fmtDate(r.tglGr), sortValue: (r) => r.tglGr },
    { key: "supplier", header: "Supplier", value: (r) => r.supplier || "-", className: "dt-wrap-sm" },
    { key: "total", header: "Total", value: (r) => money.format(r.total), sortValue: (r) => r.total, className: "dt-num" },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => (
        <span className="row-actions">
          <button
            type="button"
            className="row-action"
            aria-label={`View penerimaan ${r.id}`}
            title="View"
            onClick={() => navigate(`/transaksi-klinik/penerimaan-barang/view/${r.id}`)}
          >
            <Eye aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.noGr ? 1 : 0) + (cari.supplier !== "" ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Penerimaan Barang</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="penerimaan-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
          <Link to="/transaksi-klinik/penerimaan-barang/new" className="btn-add btn-link">
            <Plus aria-hidden="true" /> Add
          </Link>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="penerimaan-cari"
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
            <FieldShell id="cari-nogr" label="No GR">
              <input id="cari-nogr" value={draft.noGr} placeholder="Input No GR" onChange={(e) => setDraft({ ...draft, noGr: e.target.value })} />
            </FieldShell>
            <FieldShell id="cari-supplier" label="Supplier">
              <select id="cari-supplier" className="select" value={draft.supplier} onChange={(e) => setDraft({ ...draft, supplier: e.target.value })}>
                <option value="">Semua supplier</option>
                <option value={NO_SUPPLIER}>- (tanpa supplier)</option>
                {options.suppliers.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search receipts" title="Search">
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
        <NoBranchNotice what="goods receipts" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.noGr}|${cari.supplier}`}
          title={`Penerimaan Barang ${from}${to !== from ? " - " + to : ""}`}
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
