import { FormEvent, useState } from "react";
import { X } from "lucide-react";
import { api, ApiError, Pengobatan } from "../api";
import { useAuth } from "../auth/AuthContext";
import { useModal } from "../components/FormKit";

/**
 * The legacy "Cancel Pengobatan" dialog: "Apakah data No Trans <id> akan di
 * cancel?" with a required Alasan (max 100 characters). Yes cancels the sale.
 */
export default function CancelPengobatanDialog({
  sale,
  onDone,
  onClose,
}: {
  sale: Pengobatan;
  onDone: (msg: string) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [alasan, setAlasan] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!alasan.trim()) {
      setError("Isi Alasan dulu.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.cancelPengobatan(token, sale.id, alasan.trim());
      onDone(`No Trans ${sale.id} (${sale.noKwi}) sudah di-cancel.`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return logout("Your session has expired. Please log in again.");
      setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Gagal cancel transaksi");
    } finally {
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialogRef} className="modal cancel-dialog" aria-labelledby="cancel-title" onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <header className="modal-head">
          <h2 id="cancel-title">Cancel Pengobatan</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">
          <p>
            Apakah data No Trans <strong>{sale.id}</strong> akan di cancel?
          </p>
          <div className="cancel-row">
            <label htmlFor="cancel-alasan">Alasan</label>
            <input
              id="cancel-alasan"
              maxLength={100}
              placeholder="Masukan Alasan"
              autoFocus
              aria-invalid={error && !alasan.trim() ? "true" : undefined}
              value={alasan}
              onChange={(e) => setAlasan(e.target.value)}
            />
          </div>
          {error && (
            <p className="alert error" role="alert">
              {error}
            </p>
          )}
        </div>
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-small btn-primary" disabled={busy}>
            {busy ? "Memproses…" : "Yes"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}
