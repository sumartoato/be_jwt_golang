/** Date helpers shared by the patient and registration pages. Dates are YYYY-MM-DD strings. */

/** Today's date in the browser's time zone (the clinic's). */
export const today = () => {
  const d = new Date();
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
};

/** YYYY-MM-DD -> DD/MM/YYYY ("-" when empty). */
export const fmtDate = (d: string | null | undefined) => (d ? `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}` : "-");

/** Age in whole years today; null for no or an impossible birth date. */
export function age(birth: string | null | undefined): number | null {
  if (!birth || birth < "1900-01-01" || birth > today()) return null;
  const [y, m, d] = birth.split("-").map(Number);
  const now = new Date();
  let years = now.getFullYear() - y;
  if (now.getMonth() + 1 < m || (now.getMonth() + 1 === m && now.getDate() < d)) years--;
  return years;
}
