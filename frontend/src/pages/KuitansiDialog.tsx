import { FormEvent, useEffect, useState } from "react";
import { Printer, X } from "lucide-react";
import { api, ApiError, Kuitansi } from "../api";
import { useAuth } from "../auth/AuthContext";
import { useModal } from "../components/FormKit";
import { printKuitansi } from "./kuitansiPdf";
import { openPrintTab } from "../components/printPdf";

const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/**
 * The "Kwitansi" dialog of a saved sale: Tanggal, Sudah terima dari and Untuk
 * Pembayaran can be adjusted; Print records the print (the first one gets a
 * print number) and downloads Kuitansi_<id>.pdf.
 */
export default function KuitansiDialog({ saleId, onClose }: { saleId: number; onClose: () => void }) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [k, setK] = useState<Kuitansi | null>(null);
  const [tanggal, setTanggal] = useState("");
  const [dari, setDari] = useState("");
  const [untuk, setUntuk] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [done, setDone] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .kuitansi(token, saleId)
      .then((res) => {
        setK(res);
        setTanggal(res.tglKwi ?? "");
        setDari(res.pasien);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setError(err instanceof Error ? err.message : "Gagal memuat kwitansi");
      });
  }, [token, saleId]); // eslint-disable-line react-hooks/exhaustive-deps

  async function print(e: FormEvent) {
    e.preventDefault();
    if (!token || !k) return;
    const missing = [!tanggal && "Tanggal", !dari.trim() && "Sudah terima dari", !untuk.trim() && "Untuk Pembayaran"].filter(Boolean);
    if (missing.length) {
      setError(`Isi ${missing.join(", ")}.`);
      return;
    }
    // Open the print tab now, during the click: popup blockers allow it only here.
    const tab = openPrintTab();
    setBusy(true);
    setError("");
    try {
      const res = await api.cetakKuitansi(token, k.id);
      setK(res);
      await printKuitansi({
        id: res.id,
        namaCabang: res.namaCabang,
        noKwi: res.noKwi,
        noRm: res.noRm,
        sudahTerimaDari: dari.trim(),
        pasien: res.pasien,
        untukPembayaran: untuk.trim(),
        jumlah: res.jumlah,
        tanggal,
        user: res.printedUser ?? res.printedBy,
        printedAt: res.printedNow ?? "",
        noCetak: res.noCetak,
      }, tab);
      setDone(tab ? `Kuitansi dibuka untuk dicetak · No Cetak ${res.noCetak}` : `Kuitansi_${res.id}.pdf diunduh · No Cetak ${res.noCetak}`);
    } catch (err) {
      tab?.close();
      if (err instanceof ApiError && err.status === 401) return expired();
      setError(err instanceof Error ? err.message : "Gagal mencetak kwitansi");
    } finally {
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialogRef} className="modal kwt-dialog" aria-labelledby="kwt-title" onClose={onClose}>
      <form onSubmit={print} noValidate>
        <header className="modal-head">
          <h2 id="kwt-title">Kwitansi</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">
          {!k ? (
            error ? null : <p className="muted">Loading…</p>
          ) : (
            <div className="kwt-grid">
              <label htmlFor="kwt-tanggal">
                Tanggal<span className="req"> *</span>
              </label>
              <input id="kwt-tanggal" type="date" value={tanggal} onChange={(e) => setTanggal(e.target.value)} />
              <span>No Kwitansi</span>
              <strong>{k.noKwi}</strong>
              <span>No Rekam Medik / Name</span>
              <span className="kwt-pair">
                <span>{k.noRm || "-"}</span>
                <span>{k.pasien || "(pasien tidak ditemukan)"}</span>
              </span>
              <hr />
              <label htmlFor="kwt-dari">
                Sudah terima dari<span className="req"> *</span>
              </label>
              <input id="kwt-dari" maxLength={100} value={dari} onChange={(e) => setDari(e.target.value)} />
              <label htmlFor="kwt-untuk">
                Untuk Pembayaran<span className="req"> *</span>
              </label>
              <input id="kwt-untuk" maxLength={100} value={untuk} onChange={(e) => setUntuk(e.target.value)} autoFocus />
              <span>Biaya Pengobatan</span>
              <span>: Rp {plain.format(k.biayaObat)}</span>
              <span>Biaya Tindakan</span>
              <span>: Rp {plain.format(k.biayaTindakan)}</span>
              <span>Biaya Penunjang</span>
              <span>: Rp {plain.format(k.biayaPenunjang)}</span>
              {k.potongan > 0 && (
                <>
                  <span>Diskon</span>
                  <span>: Rp {plain.format(k.potongan)}</span>
                </>
              )}
              <label htmlFor="kwt-jumlah">
                Banyak nya uang<span className="req"> *</span>
              </label>
              <input id="kwt-jumlah" readOnly value={plain.format(k.jumlah)} />
              {k.noCetak && (
                <>
                  <span>No Cetak</span>
                  <span className="muted">
                    {k.noCetak} (pertama dicetak {k.printedAt?.slice(0, 16)} oleh {k.printedBy})
                  </span>
                </>
              )}
            </div>
          )}
          {error && (
            <p className="alert error" role="alert">
              {error}
            </p>
          )}
          {done && (
            <p className="alert success" role="status">
              {done}
            </p>
          )}
        </div>
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Close
          </button>
          <button type="submit" className="btn-add" disabled={!k || busy}>
            {busy ? "Mencetak…" : "Print"} <Printer aria-hidden="true" />
          </button>
        </footer>
      </form>
    </dialog>
  );
}
