import type { Nota, NotaLine } from "../api";
import { printDoc } from "../components/printPdf";

/** The Print menu's formats. */
export type NotaFormat = "a5" | "a5bold" | "thermal" | "summary";

export const NOTA_FORMATS: { value: NotaFormat; label: string }[] = [
  { value: "a5", label: "A5 Reguler" },
  { value: "a5bold", label: "A5 Bold" },
  { value: "thermal", label: "Thermal" },
  { value: "summary", label: "A5 Summary" },
];

const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** "2026-11-20 09:37:58" -> "20/11/2026 09:37". */
function fmtTgl(t: string | null): string {
  if (!t) return "-";
  return `${t.slice(8, 10)}/${t.slice(5, 7)}/${t.slice(0, 4)} ${t.slice(11, 16)}`;
}

/** "BYE BYE FEVER" -> "Bye Bye Fever", as the legacy bills print names. */
function title(t: string): string {
  return t.toLowerCase().replace(/(^|[\s(./-])(\p{L})/gu, (_, p: string, c: string) => p + c.toUpperCase());
}

/** A branch field, empty when the master holds "-". */
function field(t: string): string {
  return t.trim() === "-" ? "" : t.trim();
}

/** The branch phone as printed: the master often already says "Telp ... Fax-". */
function telp(n: Nota): string {
  const t = field(n.telpCabang);
  return !t || /^telp/i.test(t) ? t : `Telp ${t}`;
}

function sections(n: Nota): { title: string; lines: NotaLine[]; total: number }[] {
  return [
    { title: "Obat", lines: n.obat, total: n.totObat },
    { title: "Tindakan", lines: n.tindakan, total: n.totTindakan },
    { title: "Penunjang", lines: n.penunjang, total: n.totPenunjang },
  ].filter((s) => s.lines.length > 0);
}

/** Total, discount and payment rows, in print order. */
function payRows(n: Nota): [string, string][] {
  const rows: [string, string][] = [["Total", money.format(n.total)]];
  if (n.potongan) rows.push(["Diskon", money.format(n.potongan)]);
  rows.push(["Grand Total", money.format(n.grandTotal)], ["Bayar", money.format(n.pembayaran)]);
  if (n.kembalian) rows.push(["Kembalian", money.format(n.kembalian)]);
  if (n.piutang) rows.push(["Piutang", money.format(n.piutang)]);
  return rows;
}

/**
 * Builds the chosen format and prints it in tab (see printPdf.ts); without a
 * tab it downloads Nota_<format>_<id>.pdf.
 */
export async function printNota(n: Nota, format: NotaFormat, printedBy: string, printedNow = "", tab?: Window | null): Promise<void> {
  const [{ jsPDF }, { autoTable }] = await Promise.all([import("jspdf"), import("jspdf-autotable")]);
  // Server time of this print ("YYYY-MM-DD HH:MM:SS.mmm"), or the browser's.
  const now = printedNow || new Date().toLocaleString("sv-SE", { timeZone: "Asia/Jakarta" }) + ".000";
  const printedAt = now.slice(0, 16);
  const label = NOTA_FORMATS.find((f) => f.value === format)?.label ?? format;

  if (format === "thermal") {
    // 80 mm roll: the page is as long as the content, in a fixed-width font.
    const width = 80;
    const left = 4;
    const right = width - 4;
    const items = sections(n).reduce((c, s) => c + 1 + s.lines.length * 2 + 1, 0);
    const height = 70 + items * 4 + payRows(n).length * 4 + 20;
    const doc = new jsPDF({ unit: "mm", format: [width, height] });
    let y = 6;
    const line = () => {
      doc.setLineDashPattern([0.8, 0.8], 0);
      doc.line(left, y, right, y);
      doc.setLineDashPattern([], 0);
      y += 4;
    };
    const lr = (l: string, r: string, bold = false) => {
      doc.setFont("courier", bold ? "bold" : "normal");
      doc.text(l, left, y);
      doc.text(r, right, y, { align: "right" });
      y += 4;
    };
    doc.setFont("courier", "bold");
    doc.setFontSize(10);
    doc.text(n.namaCabang || "", width / 2, y, { align: "center" });
    y += 4;
    doc.setFont("courier", "normal");
    doc.setFontSize(7.5);
    for (const t of [field(n.alamatCabang), telp(n)].filter(Boolean)) {
      for (const w of doc.splitTextToSize(t, right - left) as string[]) {
        doc.text(w, width / 2, y, { align: "center" });
        y += 3.5;
      }
    }
    y += 1;
    line();
    lr("No", n.noKwi);
    lr("Tgl", fmtTgl(n.tgl));
    lr("Pasien", `${n.noRm} ${n.pasien}`.slice(0, 30));
    lr("Dokter", n.dokter.slice(0, 30));
    lr("Bayar", n.jenisBayar);
    line();
    for (const s of sections(n)) {
      doc.setFont("courier", "bold");
      doc.text(s.title.toUpperCase(), left, y);
      y += 4;
      doc.setFont("courier", "normal");
      for (const l of s.lines) {
        doc.text((doc.splitTextToSize(l.nama, right - left) as string[])[0], left, y);
        y += 4;
        lr(`  ${num.format(l.byk)} x ${num.format(l.harga)}`, num.format(l.jumlah));
      }
      lr(`Subtotal ${s.title}`, num.format(s.total), true);
    }
    line();
    payRows(n).forEach(([k, v]) => lr(k, v, k === "Grand Total"));
    line();
    doc.text(`Kasir ${n.kasir} · ${printedAt}`, width / 2, y, { align: "center" });
    y += 4;
    doc.text("Semoga lekas sembuh", width / 2, y, { align: "center" });
    printDoc(doc, `Nota_Thermal_${n.id}.pdf`, tab);
    return;
  }

  if (format === "a5" || format === "a5bold") {
    buktiPembayaran(new jsPDF({ orientation: "portrait", unit: "mm", format: "a5" }), n, format === "a5bold", now, tab);
    return;
  }

  // A5 Summary.
  const doc = new jsPDF({ orientation: "portrait", unit: "mm", format: "a5" });
  const left = 10;
  const right = 138;
  const font = "normal";
  const size = 9;
  doc.setFont("helvetica", "bold");
  doc.setFontSize(12);
  doc.text(n.namaCabang || "", left, 13);
  doc.setFont("helvetica", "normal");
  doc.setFontSize(8);
  doc.text([field(n.alamatCabang), telp(n)].filter(Boolean).join(" · "), left, 18);
  doc.setFont("helvetica", "bold");
  doc.setFontSize(11);
  doc.text("RINGKASAN PENGOBATAN", right, 13, { align: "right" });
  doc.setLineWidth(0.4);
  doc.line(left, 21, right, 21);

  doc.setFontSize(size);
  // Info block: two columns per row; the patient gets a full-width row.
  const rows: [string, string][][] = [
    [
      ["No Kwitansi", n.noKwi],
      ["Tanggal", fmtTgl(n.tgl)],
    ],
    [["Pasien", `${n.noRm} · ${n.pasien}`]],
    [
      ["Dokter", n.dokter],
      ["Pelayanan", n.pelayanan],
    ],
    [["Pembayaran", n.jenisBayar + (n.jamkes && n.jamkes !== "NON ASURANSI" ? ` · ${n.jamkes}` : "")]],
  ];
  let y = 27;
  for (const row of rows) {
    row.forEach(([k, v], i) => {
      const x = i === 0 ? left : 75;
      doc.setFont("helvetica", "normal");
      doc.text(k, x, y);
      doc.setFont("helvetica", font);
      // Fit the value in its space so it never runs into the next label.
      const room = (row.length === 2 && i === 0 ? 75 - 2 : right) - (x + 22);
      let text = `: ${v}`;
      while (text.length > 3 && doc.getTextWidth(text) > room) text = text.slice(0, -2) + "…";
      doc.text(text, x + 22, y);
    });
    y += 5;
  }
  y += 1;

  const tableStyle = {
    theme: "plain" as const,
    styles: { font: "helvetica", fontStyle: font as "bold" | "normal", fontSize: size - 0.5, cellPadding: 1.2 },
    headStyles: { fontStyle: "bold" as const, fillColor: [235, 235, 235] as [number, number, number] },
    margin: { left, right: 148 - right },
  };
  autoTable(doc, {
    ...tableStyle,
    startY: y,
    head: [["Rincian", { content: "Banyak item", styles: { halign: "right" } }, { content: "Jumlah", styles: { halign: "right" } }]],
    body: [
      ["Biaya Obat", String(n.obat.length), money.format(n.totObat)],
      ["Biaya Tindakan", String(n.tindakan.length), money.format(n.totTindakan)],
      ["Biaya Penunjang", String(n.penunjang.length), money.format(n.totPenunjang)],
    ],
    columnStyles: { 1: { halign: "right" }, 2: { halign: "right" } },
  });
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  y = ((doc as any).lastAutoTable?.finalY ?? y) + 5;
  doc.setFont("helvetica", "normal");
  for (const [k, v] of payRows(n)) {
    doc.setFont("helvetica", k === "Grand Total" ? "bold" : font);
    doc.text(k, 90, y);
    doc.text(v, right, y, { align: "right" });
    y += 5;
  }
  doc.setFont("helvetica", "normal");
  doc.setFontSize(7.5);
  doc.text(`Kasir: ${n.kasir} · Dicetak ${printedAt} oleh ${printedBy} · ${label}`, left, 202);
  printDoc(doc, `Nota_${label.replace(/\s+/g, "")}_${n.id}.pdf`, tab);
}

/**
 * A5 Reguler / A5 Bold: the legacy "BUKTI PEMBAYARAN" receipt, a narrow column
 * at the top left of the A5 sheet.
 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
function buktiPembayaran(doc: any, n: Nota, bold: boolean, now: string, tab?: Window | null): void {
  const L = 8;
  const R = 100;
  const C = (L + R) / 2;
  const style = bold ? "bold" : "normal";
  const lh = bold ? 3.5 : 3.3;
  let y = 8;
  const rule = (w = 0.5) => {
    doc.setLineWidth(w);
    doc.setDrawColor(120);
    doc.line(L, y, R, y);
    y += 4;
  };
  const center = (t: string, size: number, b: boolean) => {
    doc.setFont("helvetica", b ? "bold" : style);
    doc.setFontSize(size);
    for (const w of doc.splitTextToSize(t, R - L) as string[]) {
      doc.text(w, C, y, { align: "center" });
      y += size * 0.42;
    }
  };
  /** Cuts t until it fits in room mm. */
  const fit = (t: string, room: number) => {
    while (t.length > 1 && doc.getTextWidth(t) > room) t = t.slice(0, -2) + "…";
    return t;
  };

  // Header: print number, clinic, company, address.
  doc.setFont("helvetica", "bold");
  doc.setFontSize(6.5);
  if (n.noCetak) doc.text(n.noCetak, R, y, { align: "right" });
  y += 3.5;
  center(n.namaCabang, 8.5, true);
  if (field(n.namaPt)) center(field(n.namaPt), 6.5, true);
  y += 0.3;
  for (const t of [field(n.alamatCabang), telp(n)].filter(Boolean)) center(t, 6.5, false);
  y += 1;
  rule(0.7);
  center("BUKTI PEMBAYARAN", 8, true);
  y += 0.5;
  rule(0.7);

  // Sale info: label, ": value", and an optional second pair on the same row.
  doc.setFontSize(bold ? 7.5 : 7);
  const tgl = n.tgl ? `${n.tgl.slice(8, 10)}/${n.tgl.slice(5, 7)}/${n.tgl.slice(0, 4)}` : "-";
  const info: [string, string, string?, string?][] = [
    ["No Transaksi", `${n.noKwi}/${tgl}/${n.id}`],
    ["Nama Pasien", title(n.pasien), "No RM", n.noRm],
    ["Penanggung", title(n.penanggung || "-"), "Kasir", n.kasir],
    ["Asuransi", title(n.jamkes || "-")],
    ["Pelayanan", title(n.pelayanan), "Dokter", title(n.dokter)],
  ];
  const x2 = L + 50; // second pair's label
  const vx = bold ? 19 : 17; // first value, past the widest label
  for (const [k1, v1, k2, v2] of info) {
    doc.setFont("helvetica", style);
    doc.text(k1, L, y);
    doc.text(fit(`: ${v1}`, (k2 ? x2 : R) - (L + vx) - 1), L + vx, y);
    let rows = 1;
    if (k2) {
      doc.text(k2, x2, y);
      // A long value (the doctor) wraps under itself rather than being cut.
      const wrapped = doc.splitTextToSize(`: ${v2 ?? ""}`, R - (x2 + 11)) as string[];
      wrapped.slice(0, 2).forEach((w, i) => doc.text(i ? `  ${w}` : w, x2 + 11, y + i * lh));
      rows = Math.min(wrapped.length, 2);
    }
    y += lh * rows;
  }
  y += 1;
  rule(0.7);

  // Lines: name, then ": Rp." and the amount right-aligned.
  const xRp = L + 62;
  const amount = (k: string, v: number, b = false) => {
    doc.setFont("helvetica", b ? "bold" : style);
    doc.text(fit(k, xRp - L - 2), L + 1, y);
    doc.setFont("helvetica", style);
    doc.text(": Rp.", xRp, y);
    doc.text(money.format(v), R, y, { align: "right" });
    y += lh;
  };
  doc.setFont("helvetica", style);
  doc.text("Perincian Biaya Pengobatan :", L, y);
  y += lh;
  for (const s of sections(n)) for (const l of s.lines) amount(title(l.nama), l.jumlah);
  y += 0.5;
  doc.setLineWidth(0.3);
  doc.setDrawColor(120);
  doc.line(L, y - 2, R, y - 2);
  y += 1;

  amount("Sub Total", n.total, true);
  if (n.potongan) amount("Diskon", n.potongan);
  amount("Grand Total", n.grandTotal);
  amount("Pembayaran", n.pembayaran);
  amount("Kembalian", n.kembalian);
  if (n.piutang) amount("Piutang", n.piutang);
  y += lh * 0.5;
  amount("Total Point", n.point);
  y += 0.5;
  rule(0.7);

  center("SEMOGA LEKAS SEMBUH", 7, false);
  center("TERMAKASIH ATAS KUNJUNGAN ANDA", 7, false);
  center(`Tanggal Cetak : ${now}`, 7, false);
  printDoc(doc, `Nota_A5${bold ? "Bold" : "Reguler"}_${n.id}.pdf`, tab);
}
