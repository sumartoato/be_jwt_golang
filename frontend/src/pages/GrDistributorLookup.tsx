import { useEffect, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, PenerimaanDistributor } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { useModal } from "../components/FormKit";

/**
 * The Penerimaan Barang form's "Data Distributor" lookup (refdistributorobat),
 * by code like the legacy one. Picking a row sets Distributor.
 */
export default function GrDistributorLookup({ onPick, onClose }: { onPick: (d: PenerimaanDistributor) => void; onClose: () => void }) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<PenerimaanDistributor[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .penerimaanDistributor(token)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load distributors");
      });
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<PenerimaanDistributor>[] = [
    { key: "id", header: "Kode", value: (d) => String(d.id), sortValue: (d) => d.id },
    { key: "nama", header: "Nama Distributor", value: (d) => d.nama || "-" },
    { key: "alamat", header: "Alamat", value: (d) => d.alamat || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <button type="button" className="row-action" aria-label={`Pilih distributor ${d.nama}`} title="Pilih" onClick={() => onPick(d)}>
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="gr-distributor-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="gr-distributor-title">Data Distributor</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        {error ? (
          <p className="alert error" role="alert">
            {error}
          </p>
        ) : !rows ? (
          <p className="muted">Loading…</p>
        ) : (
          <DataTable
            title="Data Distributor"
            columns={columns}
            rows={rows}
            rowKey={(d) => d.id}
            initialSort={{ key: "id", dir: "asc" }}
            pageSizes={[5, 10, 20, 50]}
            defaultPageSize={5}
            exports={false}
          />
        )}
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}
