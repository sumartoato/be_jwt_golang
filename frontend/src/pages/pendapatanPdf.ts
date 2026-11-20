import type { ReportPendapatan } from "../api";
import { tanggalCetak } from "./kuitansiPdf";

const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** "2026-11-19" -> "19/11/2026". */
const dmy = (d: string) => `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}`;

/**
 * Builds the legacy "Laporan Pendapatan Klinik" (A4 portrait) and downloads it
 * as Report_Income.pdf: the branch header, the filters, one row per sale, then
 * the patient counts, the totals per group and the income.
 */
export async function downloadPendapatan(r: ReportPendapatan, layanan: string, dokter: string): Promise<void> {
  const [{ jsPDF }, { autoTable }] = await Promise.all([import("jspdf"), import("jspdf-autotable")]);
  const doc = new jsPDF({ orientation: "portrait", unit: "mm", format: "a4" });
  const left = 14;
  const right = 196;
  const center = (left + right) / 2;

  // Header: "Klinik", the branch with its phone, a double rule.
  doc.setFont("times", "normal");
  doc.setFontSize(11);
  doc.text("Klinik", center, 13, { align: "center" });
  doc.setFontSize(15);
  doc.text([r.namaCabang, r.telpCabang].filter(Boolean).join(" "), center, 20, { align: "center", maxWidth: right - left });
  doc.setLineWidth(0.3);
  doc.line(left, 24, right, 24);
  doc.line(left, 25, right, 25);
  doc.setFontSize(12);
  doc.text("Laporan Pendapatan Klinik", center, 32, { align: "center" });

  doc.setFontSize(9);
  doc.text(`Nama Poli : ${layanan}`, left, 38);
  doc.text(`Dokter  : ${dokter}`, 95, 38);
  doc.text(`Tanggal Posting : ${dmy(r.today)}`, 150, 38);
  doc.text(`Petugas     : ${r.petugas}`, left, 43);
  doc.text(`Periode : ${dmy(r.from)} sd ${dmy(r.to)}`, 95, 43);

  const sum = (k: "obat" | "tindakan" | "penunjang" | "total" | "potongan") => r.rows.reduce((t, x) => t + x[k], 0);
  autoTable(doc, {
    startY: 46,
    theme: "grid",
    // As in the legacy PDF: the header only on the first page, and no row cut across pages.
    showHead: "firstPage",
    rowPageBreak: "avoid",
    styles: { font: "times", fontSize: 8, cellPadding: 0.8, textColor: 0, lineColor: 0, lineWidth: 0.15 },
    headStyles: { fillColor: [255, 255, 255], textColor: 0, fontStyle: "normal" },
    margin: { left, right: 210 - right },
    head: [["No", "Tanggal / Waktu", "No Kwt", "Nama Pasien", "Obat", "Tindakan", "Penunjang", "Total"]],
    body: r.rows.map((x, i) => [String(i + 1), x.tgl, x.noKwi, x.pasien, num.format(x.obat), num.format(x.tindakan), num.format(x.penunjang), num.format(x.total)]),
    columnStyles: {
      0: { halign: "right", cellWidth: 9 },
      1: { cellWidth: 36 },
      2: { halign: "right", cellWidth: 22 },
      4: { halign: "right", cellWidth: 18 },
      5: { halign: "right", cellWidth: 18 },
      6: { halign: "right", cellWidth: 18 },
      7: { halign: "right", cellWidth: 20 },
    },
  });

  // Summary: signature (left), patient counts (middle), totals (right).
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let y = ((doc as any).lastAutoTable?.finalY ?? 46) + 8;
  if (y > 250) {
    doc.addPage();
    y = 20;
  }
  const baru = r.rows.filter((x) => x.baru).length;
  doc.setFontSize(9);
  doc.text(`Jakarta, ${tanggalCetak(r.today)}`, 50, y, { align: "center" });
  doc.text("Mengetahui,", 50, y + 5, { align: "center" });
  doc.text(`(${r.petugas})`, 50, y + 30, { align: "center" });
  doc.text(`Jumlah Pasien : ${r.rows.length}`, 92, y);
  doc.text(`Pasien Baru  : ${baru}`, 92, y + 5);
  doc.text(`Pasien Lama  : ${r.rows.length - baru}`, 92, y + 10);

  const amount = (label: string, v: number, at: number) => {
    doc.text(label, 140, at);
    doc.text("= Rp.", 168, at);
    doc.text(num.format(v), right, at, { align: "right" });
  };
  amount("Total Obat", sum("obat"), y);
  amount("Total Tindakan", sum("tindakan"), y + 5);
  amount("Total Penunjang", sum("penunjang"), y + 10);
  doc.setLineWidth(0.4);
  doc.line(140, y + 13, right, y + 13);
  amount("Total Tagihan", sum("total"), y + 18);
  amount("Potongan", sum("potongan"), y + 23);
  doc.line(140, y + 26, right, y + 26);
  amount("Total Pendapatan", sum("total") - sum("potongan"), y + 31);

  doc.save("Report_Income.pdf");
}
