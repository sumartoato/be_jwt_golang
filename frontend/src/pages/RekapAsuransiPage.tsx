import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, PengobatanStatus, RekapAsuransi, RekapAsuransiOptions, RekapAsuransiQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";
import { fmtDate } from "../components/dates";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

const STATUS_LABEL: Record<PengobatanStatus, string> = { open: "Open", close: "Close", cancel: "Cancel" };

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; asuransi: string; status: PengobatanStatus | "" };
const NO_CARI: Cari = { nama: "", asuransi: "", status: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/**
 * One table row: a sale's first row carries its header cells (date, patient,
 * insurer, ...); then come its items per group (an empty group still has one
 * row with zeros, as in the legacy report) and its Total row.
 */
type Line = {
  key: string;
  sale: RekapAsuransi;
  first: boolean;
  /** Keterangan: Obat, Tindakan, Penunjang or Total, on a group's first row. */
  ket: string;
  total: boolean;
  details: string;
  byk: number | null;
  harga: number | null;
  jumlah: number;
};

const GROUPS = [
  ["Obat", "obat"],
  ["Tindakan", "tindakan"],
  ["Penunjang", "penunjang"],
] as const;

function lines(sales: RekapAsuransi[]): Line[] {
  const out: Line[] = [];
  for (const s of sales) {
    const base = { sale: s, total: false };
    for (const [label, key] of GROUPS) {
      const items = s[key];
      if (items.length === 0) {
        out.push({ ...base, key: `${s.idJual}-${key}`, first: out.length === 0 || out[out.length - 1].sale !== s, ket: label, details: "", byk: 0, harga: 0, jumlah: 0 });
        continue;
      }
      items.forEach((l, i) =>
        out.push({
          ...base,
          key: `${s.idJual}-${key}-${i}`,
          first: out.length === 0 || out[out.length - 1].sale !== s,
          ket: i === 0 ? label : "",
          details: l.nama,
          byk: l.byk,
          harga: l.harga,
          jumlah: l.jumlah,
        }),
      );
    }
    out.push({ ...base, key: `${s.idJual}-total`, first: false, ket: "Total", total: true, details: "", byk: null, harga: null, jumlah: s.total });
  }
  return out;
}

/** n days before a YYYY-MM-DD date. */
function daysBefore(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() - n);
  return d.toISOString().slice(0, 10);
}

/**
 * Report Klinik > Rekap Asuransi (legacy /ReportAsuransi): the insured sales at
 * the user's branches, newest first, each with its drugs, procedures and
 * examinations and its total. Paging counts sales; the export lists every row.
 */
export default function RekapAsuransiPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<RekapAsuransiOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: RekapAsuransi[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .rekapAsuransiOptions(token)
      .then((o) => {
        setOptions(o);
        // Default: yesterday and today (H-1 to H).
        setFrom(daysBefore(o.today, 1));
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): RekapAsuransiQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    asuransi: cari.asuransi ? Number(cari.asuransi) : undefined,
    status: cari.status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .rekapAsuransi(token, params(q))
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

  const fetchAll = async (q: RemoteQuery): Promise<Line[]> => {
    if (!token) return [];
    const res = await api.rekapAsuransi(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} sales. Narrow the dates to export the rest.` });
    }
    return lines(res.rows);
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  // Header cells only on a sale's first row; item columns cannot be sorted.
  const head = (f: (s: RekapAsuransi) => string) => (r: Line) => (r.first ? f(r.sale) : "");
  const columns: Column<Line>[] = [
    { key: "tgl", header: "Tanggal", value: head((s) => fmtDate(s.tgl)) },
    { key: "noRm", header: "No RM", value: head((s) => s.noRm) },
    { key: "noKartu", header: "Card Number", value: head((s) => s.noKartu) },
    { key: "perusahaan", header: "Perusahaan", value: head((s) => s.perusahaan), className: "dt-wrap-sm" },
    { key: "pasien", header: "Pasien", value: head((s) => s.pasien), className: "dt-wrap-sm" },
    { key: "dokter", header: "Dokter", value: head((s) => s.dokter), className: "dt-wrap-sm" },
    { key: "diagnosa", header: "Diagnosa", value: head((s) => s.diagnosa.join(", ")), sortable: false, className: "dt-wrap-sm" },
    {
      key: "ket",
      header: "Keterangan",
      value: (r) => r.ket,
      sortable: false,
      render: (r) => (r.total ? <strong>{r.ket}</strong> : r.ket),
    },
    { key: "details", header: "Details", value: (r) => r.details, sortable: false, className: "dt-wrap-sm" },
    { key: "byk", header: "Banyak", value: (r) => (r.byk === null ? "" : plain.format(r.byk)), sortable: false, className: "dt-num" },
    { key: "harga", header: "Harga", value: (r) => (r.harga === null ? "" : money.format(r.harga)), sortable: false, className: "dt-num" },
    {
      key: "total",
      header: "Total",
      value: (r) => money.format(r.jumlah),
      sortable: false,
      className: "dt-num",
      render: (r) => (r.total ? <strong>{money.format(r.jumlah)}</strong> : money.format(r.jumlah)),
    },
    { key: "cabang", header: "Cabang", value: head((s) => branchName.get(s.cabang) ?? s.cabang), className: "dt-wrap-sm" },
    {
      key: "status",
      header: "Status",
      value: head((s) => STATUS_LABEL[s.status]),
      render: (r) => (r.first ? <span className={`sale-status ${r.sale.status}`}>{STATUS_LABEL[r.sale.status]}</span> : ""),
    },
  ];

  const rows = useMemo(() => lines(page?.rows ?? []), [page]);
  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.asuransi ? 1 : 0) + (cari.status ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Rekap Asuransi</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal transaksi">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="rekap-asuransi-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="rekap-asuransi-cari"
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
            <FieldShell id="cari-asuransi" label="Perusahaan">
              <select id="cari-asuransi" className="select" value={draft.asuransi} onChange={(e) => setDraft({ ...draft, asuransi: e.target.value })}>
                <option value="">Semua perusahaan</option>
                {options.asuransi.map((o) => (
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
              <button type="submit" className="icon-round" aria-label="Search insured sales" title="Search">
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
        <NoBranchNotice what="insured sales" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.asuransi}|${cari.status}`}
          title={`Rekap Asuransi ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={rows}
          rowKey={(r) => r.key}
          initialSort={{ key: "tgl", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll, pageEntries: page?.rows.length ?? 0 }}
        />
      )}
    </section>
  );
}
