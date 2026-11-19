import { useCallback, useRef, useState } from "react";
import { RotateCcw, Search, SquareCheck, X } from "lucide-react";
import { api, ApiError, PengobatanOptions, PengobatanPendaftaran } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { addDays, age } from "../components/dates";
import { useModal } from "../components/FormKit";

type Filters = { nama: string; status: string; pelayanan: string; from: string; to: string };

/**
 * The Pengobatan form's "Data Pendaftaran" lookup: registrations of one branch
 * that a treatment can still be recorded for (DAFTAR, PROSES or PAYMENT, not
 * billed yet). Picking one fills the form.
 */
export default function PendaftaranLookup({
  cabang,
  options,
  onPick,
  onClose,
}: {
  cabang: string;
  options: PengobatanOptions;
  onPick: (r: PengobatanPendaftaran) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  // Like the original screen: yesterday and today.
  const initial: Filters = { nama: "", status: "", pelayanan: "", from: addDays(options.today, -1), to: options.today };
  const [draft, setDraft] = useState<Filters>(initial);
  const [applied, setApplied] = useState<Filters>(initial);
  const [page, setPage] = useState<{ rows: PengobatanPendaftaran[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const seq = useRef(0);
  const branchName = new Map(options.branches.map((b) => [b.kode, b.nama]));

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pengobatanPendaftaran(token, {
          cabang,
          from: applied.from,
          to: applied.to,
          page: q.page,
          size: q.pageSize,
          q: q.search,
          sort: q.sort?.key,
          dir: q.sort?.dir,
          nama: applied.nama || undefined,
          status: applied.status ? Number(applied.status) : undefined,
          pelayanan: applied.pelayanan ? Number(applied.pelayanan) : undefined,
        })
        .then((res) => {
          if (mine !== seq.current) return;
          setPage(res);
          setError("");
        })
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
          else setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load registrations");
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, cabang, applied], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const apply = () => {
    if (!draft.from || !draft.to || draft.to < draft.from) {
      setError("Isi tanggal Dari dan Sampai (Dari ≤ Sampai).");
      return;
    }
    setError("");
    setApplied({ ...draft, nama: draft.nama.trim() });
  };
  const reset = () => {
    setDraft(initial);
    setApplied(initial);
    setError("");
  };

  const columns: Column<PengobatanPendaftaran>[] = [
    { key: "id", header: "No Pen", value: (r) => String(r.id) },
    { key: "tglMasuk", header: "Tanggal / Waktu", value: (r) => r.tglMasuk ?? "-" },
    { key: "pelayanan", header: "Pelayanan", value: (r) => r.pelayanan || "-" },
    { key: "status", header: "Status Kun", value: (r) => r.status || "-" },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "nama", header: "Nama Pasien", value: (r) => r.nama || "(pasien tidak ditemukan)", className: "dt-wrap-sm" },
    { key: "umur", header: "Umur", value: (r) => String(age(r.tglLahir) ?? "-") },
    { key: "alamat", header: "Alamat", value: (r) => r.alamat || "-", className: "dt-wrap-sm" },
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    {
      key: "dokter",
      header: "Dokter PJ",
      value: (r) => r.dokter || "Belum di Pilih",
      render: (r) => (r.dokter ? r.dokter : <span className="muted">Belum di Pilih</span>),
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => (
        <button
          type="button"
          className="row-action"
          aria-label={`Pilih pendaftaran ${r.id}`}
          title="Pilih"
          disabled={!r.noRm}
          onClick={() => onPick(r)}
        >
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="reg-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="reg-lookup-title">Data Pendaftaran · {branchName.get(cabang) ?? cabang}</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        <form
          className="lookup-filters"
          onSubmit={(e) => {
            e.preventDefault();
            apply();
          }}
        >
          <label>
            Nama Pasien
            <input value={draft.nama} placeholder="Nama Pasien" onChange={(e) => setDraft({ ...draft, nama: e.target.value })} />
          </label>
          <label>
            Proses
            <select className="select" value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value })}>
              <option value="">All</option>
              {options.proses.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Layanan
            <select className="select" value={draft.pelayanan} onChange={(e) => setDraft({ ...draft, pelayanan: e.target.value })}>
              <option value="">All</option>
              {options.pelayanan.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            Dari
            <input type="date" value={draft.from} max={draft.to || undefined} onChange={(e) => setDraft({ ...draft, from: e.target.value })} />
          </label>
          <label>
            Sampai
            <input type="date" value={draft.to} min={draft.from || undefined} onChange={(e) => setDraft({ ...draft, to: e.target.value })} />
          </label>
          <div className="search-panel-actions">
            <button type="submit" className="icon-round" aria-label="Cari pendaftaran" title="Cari">
              <Search aria-hidden="true" />
            </button>
            <button type="button" className="icon-round" aria-label="Reset pencarian pendaftaran" title="Reset" onClick={reset}>
              <RotateCcw aria-hidden="true" />
            </button>
          </div>
        </form>
        {error && (
          <p className="alert error" role="alert">
            {error}
          </p>
        )}
        <DataTable
          key={`${applied.nama}|${applied.status}|${applied.pelayanan}|${applied.from}|${applied.to}`}
          title="Data Pendaftaran"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.id}
          initialSort={{ key: "id", dir: "desc" }}
          pageSizes={[5, 10, 20, 50]}
          defaultPageSize={5}
          exports={false}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage }}
        />
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}
