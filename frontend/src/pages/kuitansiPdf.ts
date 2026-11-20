import { terbilang } from "../components/terbilang";
import { printDoc } from "../components/printPdf";

/** What the printed "KWITANSI PENGOBATAN" shows (the legacy Kuitansi_<id>.pdf layout). */
export interface KuitansiPrint {
  id: number;
  namaCabang: string;
  noKwi: string;
  noRm: string;
  sudahTerimaDari: string;
  pasien: string;
  untukPembayaran: string;
  jumlah: number;
  /** YYYY-MM-DD, the date in the dialog. */
  tanggal: string;
  /** Printed in brackets under the date, e.g. "( ato )". */
  user: string;
  /** "Tanggal Cetak": the server time of this print. */
  printedAt: string;
  noCetak: string;
}

const CITY = "Jakarta";
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"];
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** "2026-11-20" -> "20 Nov 2026". */
export function tanggalCetak(day: string): string {
  const [y, m, d] = day.split("-").map(Number);
  return `${d} ${MONTHS[m - 1] ?? ""} ${y}`;
}

/** Builds the receipt PDF (A4 portrait) and prints it in tab; without a tab it downloads Kuitansi_<id>.pdf. */
export async function printKuitansi(k: KuitansiPrint, tab?: Window | null): Promise<void> {
  const { jsPDF } = await import("jspdf");
  const doc = new jsPDF({ orientation: "portrait", unit: "mm", format: "a4" });
  const left = 25;
  const right = 185;
  doc.setFont("times", "normal");

  // Header: clinic name (left) and the print number (right), above a rule.
  doc.setFontSize(11);
  doc.setFont("times", "bold");
  doc.text(k.namaCabang || "", left, 22);
  doc.setFont("times", "normal");
  doc.setFontSize(9);
  if (k.noCetak) doc.text(`No Cetak : ${k.noCetak}`, right, 22, { align: "right" });
  doc.setLineWidth(0.6);
  doc.setDrawColor(120);
  doc.line(left, 32, right, 32);

  doc.setFont("times", "bold");
  doc.setFontSize(12);
  doc.text("KWITANSI PENGOBATAN", 105, 41, { align: "center" });

  doc.setFont("times", "normal");
  doc.setFontSize(11);
  // Receipt and patient number, right column.
  const rx = 120;
  doc.text("No Kwit", rx, 53);
  doc.text(`:   ${k.noKwi}`, rx + 30, 53);
  doc.text("No Pasien", rx, 59);
  doc.text(`:   ${k.noRm || "-"}`, rx + 30, 59);

  const label = (y: number, name: string, value: string) => {
    doc.text(name, left, y);
    doc.text(`:   ${value}`, left + 38, y);
  };
  label(65, "Sudah Terima Dari", k.sudahTerimaDari);
  label(71, "Untuk Pasien", k.pasien || "-");
  label(77, "Untuk Pembayaran", k.untukPembayaran);
  label(89, "Jumlah Pembayaran", money.format(k.jumlah));
  const words = doc.splitTextToSize(terbilang(k.jumlah), right - left - 45) as string[];
  label(95, "Terbilang", words[0] ?? "");
  words.slice(1).forEach((w, i) => doc.text(w, left + 43, 101 + i * 6));

  const dateY = 101 + Math.max(0, words.length - 1) * 6 + 6;
  doc.text(`${CITY} , ${tanggalCetak(k.tanggal)}`, 158, dateY, { align: "center" });
  doc.text(`( ${k.user} )`, 158, dateY + 24, { align: "center" });

  const footY = dateY + 32;
  doc.line(left, footY, right, footY);
  doc.text("SEMOGA LEKAS SEMBUH", 105, footY + 9, { align: "center" });
  doc.text("TERMAKASIH ATAS KUNJUNGAN ANDA", 105, footY + 15, { align: "center" });
  doc.text(`Tanggal Cetak : ${k.printedAt}`, 105, footY + 21, { align: "center" });

  printDoc(doc, `Kuitansi_${k.id}.pdf`, tab);
}
