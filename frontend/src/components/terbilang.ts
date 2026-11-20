/**
 * Indonesian amount in words for receipts, in title case like the legacy
 * receipt: 50000 -> "Lima Puluh Ribu Rupiah". Cents are dropped (rounded).
 */

const ONES = ["", "satu", "dua", "tiga", "empat", "lima", "enam", "tujuh", "delapan", "sembilan", "sepuluh", "sebelas"];

function below1000(n: number): string {
  if (n < 12) return ONES[n];
  if (n < 20) return `${ONES[n - 10]} belas`;
  if (n < 100) return `${ONES[Math.floor(n / 10)]} puluh ${below1000(n % 10)}`.trim();
  if (n < 200) return `seratus ${below1000(n - 100)}`.trim();
  return `${ONES[Math.floor(n / 100)]} ratus ${below1000(n % 100)}`.trim();
}

const SCALES: [number, string][] = [
  [1e12, "triliun"],
  [1e9, "miliar"],
  [1e6, "juta"],
  [1e3, "ribu"],
];

/** Words for a whole number, lower case ("seribu", "dua ribu lima ratus"). */
export function terbilangAngka(value: number): string {
  let n = Math.round(Math.abs(value));
  if (n === 0) return "nol";
  const parts: string[] = [];
  for (const [size, name] of SCALES) {
    if (n >= size) {
      const count = Math.floor(n / size);
      n %= size;
      // 1000 is "seribu", not "satu ribu".
      parts.push(size === 1e3 && count === 1 ? "seribu" : `${terbilangAngka(count)} ${name}`);
    }
  }
  if (n > 0) parts.push(below1000(n));
  return (value < 0 ? "minus " : "") + parts.join(" ");
}

/** Receipt form: "Lima Puluh Ribu Rupiah". */
export function terbilang(value: number): string {
  const words = `${terbilangAngka(value)} rupiah`;
  return words.replace(/\b\p{L}/gu, (c) => c.toUpperCase());
}
