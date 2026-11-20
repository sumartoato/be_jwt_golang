import { useEffect, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { ApiError, PengobatanTindakan } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { useModal } from "../components/FormKit";

const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Parses an amount typed as 1750000, 1.750.000 or 1.750.000,50. */
export function parseAmount(s: string): number {
  const t = s.trim();
  if (!t) return NaN;
  // Indonesian format: "." groups thousands and "," marks decimals.
  const normal = /[.,]/.test(t) && /,\d{1,2}$|\.\d{3}/.test(t) ? t.replace(/\./g, "").replace(",", ".") : t;
  return Number(normal);
}

/**
 * A priced-item lookup of the Pengobatan form, used by two tabs:
 *   - Tindakan: "Data Tindakan Dokter : <doctor>", the Dokter PJ's procedures at the branch;
 *   - Penunjang: "Data Tindakan", the supporting examinations at the branch.
 * A price marked editable can be changed before adding; type Banyak and press Enter, or tick.
 */
export default function TindakanLookup({
  title,
  load,
  onPick,
  onClose,
}: {
  title: string;
  /** Fetches the rows; isEdit marks an editable price. */
  load: (token: string) => Promise<{ rows: PengobatanTindakan[] }>;
  onPick: (t: PengobatanTindakan, harga: number, byk: number) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<PengobatanTindakan[] | null>(null);
  const [qty, setQty] = useState<Record<number, string>>({});
  const [price, setPrice] = useState<Record<number, string>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    load(token)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load");
      });
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const pick = (t: PengobatanTindakan) => {
    const byk = Number((qty[t.id] ?? "1").trim().replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0) {
      setError(`Banyak untuk ${t.nama} harus lebih dari 0.`);
      return;
    }
    const harga = t.isEdit && price[t.id] !== undefined ? parseAmount(price[t.id]) : t.harga;
    if (!Number.isFinite(harga) || harga < 0) {
      setError(`Harga untuk ${t.nama} tidak valid.`);
      return;
    }
    setError("");
    onPick(t, harga, byk);
  };

  const onEnter = (t: PengobatanTindakan) => (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      e.preventDefault();
      pick(t);
    }
  };

  const columns: Column<PengobatanTindakan>[] = [
    { key: "id", header: "ID", value: (t) => String(t.id), sortValue: (t) => t.id },
    { key: "nama", header: "Nama Tindakan", value: (t) => t.nama, className: "dt-wrap-sm" },
    { key: "jenis", header: "Jenis", value: (t) => t.jenis || "-" },
    {
      key: "harga",
      header: "Harga",
      value: (t) => money.format(t.harga),
      sortValue: (t) => t.harga,
      className: "dt-num",
      render: (t) =>
        t.isEdit ? (
          <input
            className="price-input"
            inputMode="decimal"
            aria-label={`Harga ${t.nama}`}
            title="Harga bisa diubah"
            value={price[t.id] ?? money.format(t.harga)}
            onChange={(e) => setPrice({ ...price, [t.id]: e.target.value })}
            onFocus={(e) => e.target.select()}
            onKeyDown={onEnter(t)}
          />
        ) : (
          money.format(t.harga)
        ),
    },
    {
      key: "banyak",
      header: "Banyak",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-num",
      render: (t) => (
        <input
          className="qty-input"
          type="number"
          min={0}
          step="any"
          inputMode="decimal"
          aria-label={`Banyak ${t.nama}`}
          value={qty[t.id] ?? "1"}
          onChange={(e) => setQty({ ...qty, [t.id]: e.target.value })}
          onKeyDown={onEnter(t)}
        />
      ),
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (t) => (
        <button type="button" className="row-action" aria-label={`Pilih tindakan ${t.nama}`} title="Pilih" onClick={() => pick(t)}>
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="tindakan-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="tindakan-lookup-title">{title}</h2>
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
        {!rows ? (
          !error && <p className="muted">Loading…</p>
        ) : (
          <DataTable
            title={title}
            columns={columns}
            rows={rows}
            rowKey={(t) => t.id}
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
