import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, Info, RotateCcw, Search, X } from "lucide-react";
import { AllStok, AllStokOptions, AllStokQuery, api, ApiError } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; jenis: string; movingPo: string };
const NO_CARI: Cari = { nama: "", jenis: "", movingPo: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/** The day before a YYYY-MM-DD date. */
function dayBefore(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - 1);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > All Stok (legacy /ReportAllStock): every drug at the user's
 * branches with its stock at the start and end of the period and the drugs sold
 * in it, the dearest first. Stok Akhir is today's stock plus what was sold and
 * less what was received after the period; Stok Awal adds the period's sales and
 * takes off its receipts (stock adjustments are not recorded yet).
 */
export default function AllStokPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<AllStokOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: AllStok[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .allStokOptions(token)
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
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): AllStokQuery => ({
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
    movingPo: cari.movingPo ? Number(cari.movingPo) : undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .allStok(token, params(q))
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

  const fetchAll = async (q: RemoteQuery): Promise<AllStok[]> => {
    if (!token) return [];
    const res = await api.allStok(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Pick a branch or use Cari to export the rest.`,
      });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const periode = `${from} - ${to}`;
  const num = (key: "hargaBeli" | "hargaJual" | "stokAwal" | "jual" | "stokAkhir", header: string): Column<AllStok> => ({
    key,
    header,
    value: (r) => plain.format(r[key]),
    sortValue: (r) => r[key],
    className: "dt-num",
  });
  const columns: Column<AllStok>[] = [
    { key: "periode", header: "Periode", value: () => periode, sortable: false },
    { key: "idObat", header: "Kode", value: (r) => String(r.idObat), sortValue: (r) => r.idObat },
    { key: "obat", header: "Obat", value: (r) => r.obat, className: "dt-wrap" },
    { key: "satuanBeli", header: "Satuan Beli", value: (r) => r.satuanBeli || "-" },
    { key: "isiPerBox", header: "Isi Per Box", value: (r) => r.isiPerBox || "-" },
    { key: "satuanJual", header: "Satuan Jual", value: (r) => r.satuanJual || "-" },
    num("hargaBeli", "Harga Beli"),
    num("hargaJual", "Harga Jual"),
    { key: "jenis", header: "Jenis", value: (r) => r.jenis || "-" },
    { key: "principal", header: "Principal", value: (r) => r.principal || "-", className: "dt-wrap-sm" },
    { key: "distributor", header: "Distributor", value: (r) => r.distributor || "-", className: "dt-wrap-sm" },
    { key: "movingPo", header: "Kode Moving PO", value: (r) => r.movingPo || "-" },
    num("stokAwal", "Stok Awal"),
    num("jual", "Jual Obat"),
    num("stokAkhir", "Stok Akhir"),
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.jenis ? 1 : 0) + (cari.movingPo ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>All Stok</h1>
        <div className="head-tools">
          <label className="date-range" title="Periode stok">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="all-stok-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="all-stok-cari"
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
            <FieldShell id="cari-moving" label="Kode Moving PO">
              <select id="cari-moving" className="select" value={draft.movingPo} onChange={(e) => setDraft({ ...draft, movingPo: e.target.value })}>
                <option value="">Semua kode</option>
                {options.movingPo.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search stock" title="Search">
                <Search aria-hidden="true" />
              </button>
              <button type="button" className="icon-round" aria-label="Reset search" title="Reset" onClick={resetCari}>
                <RotateCcw aria-hidden="true" />
              </button>
            </div>
          </div>
        </form>
      )}
      <p className="alert info" role="note">
        <Info aria-hidden="true" /> Stok Akhir = stok saat ini + obat terjual setelah periode − barang diterima setelah periode; Stok Awal = Stok Akhir +
        Jual Obat − barang diterima dalam periode. Penyesuaian stok belum dicatat aplikasi ini, jadi belum dihitung.
      </p>
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
        <NoBranchNotice what="stock" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.jenis}|${cari.movingPo}`}
          title={`All Stok ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => `${r.cabang}|${r.idObat}`}
          initialSort={{ key: "hargaJual", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
