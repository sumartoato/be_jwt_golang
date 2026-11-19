/**
 * Treatment drug lines and their totals, following the legacy rules found in
 * the lab data (trxjualobat / trxjualtotal):
 *   - Jumlah = Harga x Banyak;
 *   - PPN = 10% of Jumlah for drugs with kena_pajak = 1, otherwise 0;
 *   - "di bulatkan": each line is rounded up to a multiple of 500, then summed.
 */

export const PPN_RATE = 0.1;
export const ROUND_TO = 500;

export interface ObatLine {
  idObat: number;
  nama: string;
  jenis: string;
  satuan: string;
  harga: number;
  byk: number;
  kenaPajak: boolean;
  /** Branch stock when the line was added; Banyak may not exceed it. */
  stok: number;
  dosis: string;
}

/** Rounds up to a multiple of ROUND_TO, ignoring floating-point dust (0.1 + 0.2). */
export function roundUp(x: number): number {
  const cents = Math.round(x * 100) / 100;
  return Math.ceil(cents / ROUND_TO) * ROUND_TO;
}

export const jumlah = (l: ObatLine) => l.harga * l.byk;
export const ppn = (l: ObatLine) => (l.kenaPajak ? jumlah(l) * PPN_RATE : 0);

export interface ObatTotals {
  total: number;
  totalRounded: number;
  ppn: number;
  ppnRounded: number;
  /** Lines the patient discount applies to; for now every line. */
  kenaDiskon: number;
  kenaDiskonRounded: number;
  ppnKenaDiskon: number;
  ppnKenaDiskonRounded: number;
  /** What the drugs add to the bill: rounded total plus rounded PPN. */
  bill: number;
}

export function obatTotals(lines: ObatLine[]): ObatTotals {
  let total = 0;
  let totalRounded = 0;
  let tax = 0;
  let taxRounded = 0;
  for (const l of lines) {
    total += jumlah(l);
    totalRounded += roundUp(jumlah(l));
    tax += ppn(l);
    taxRounded += roundUp(ppn(l));
  }
  return {
    total,
    totalRounded,
    ppn: tax,
    ppnRounded: taxRounded,
    kenaDiskon: total,
    kenaDiskonRounded: totalRounded,
    ppnKenaDiskon: tax,
    ppnKenaDiskonRounded: taxRounded,
    bill: totalRounded + taxRounded,
  };
}
