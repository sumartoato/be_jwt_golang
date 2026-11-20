import { useCallback, useRef, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, PengobatanIcd } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { useModal } from "../components/FormKit";

/**
 * The Diagnosa tab's "Data Diagnosa" lookup: the ICD-X codes (18.5k), paged and
 * searched on the server, by ID like the original screen. Ticking a code adds it.
 */
export default function DiagnosaLookup({
  chosen,
  onPick,
  onClose,
}: {
  /** Codes already in the list; picking one again is refused. */
  chosen: Set<number>;
  onPick: (d: PengobatanIcd) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [page, setPage] = useState<{ rows: PengobatanIcd[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const seq = useRef(0);

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pengobatanIcd(token, { q: q.search, page: q.page, size: q.pageSize, sort: q.sort?.key, dir: q.sort?.dir })
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
          else setError(err instanceof Error ? err.message : "Failed to load ICD-X codes");
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const pick = (d: PengobatanIcd) => {
    if (chosen.has(d.id)) {
      setError(`${d.kode} ${d.nama} sudah ada di daftar diagnosa.`);
      return;
    }
    setError("");
    onPick(d);
  };

  const columns: Column<PengobatanIcd>[] = [
    { key: "id", header: "ID", value: (d) => String(d.id) },
    { key: "kode", header: "Kode ICD-X", value: (d) => d.kode },
    { key: "nama", header: "Nama Penyakit", value: (d) => d.nama, className: "dt-wrap-sm" },
    { key: "jenis", header: "Jenis ICD-X", value: (d) => d.jenis || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <button
          type="button"
          className="row-action"
          aria-label={`Pilih diagnosa ${d.kode}`}
          title={chosen.has(d.id) ? "Sudah dipilih" : "Pilih"}
          disabled={chosen.has(d.id)}
          onClick={() => pick(d)}
        >
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="icd-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="icd-lookup-title">Data Diagnosa</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        {error && (
          <p className="alert error" role="alert">
            {error}
          </p>
        )}
        <DataTable
          title="Data Diagnosa"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(d) => d.id}
          initialSort={{ key: "id", dir: "asc" }}
          pageSizes={[5, 10, 20, 50]}
          defaultPageSize={10}
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
