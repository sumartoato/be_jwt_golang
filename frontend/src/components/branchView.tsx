import type { Column } from "./DataTable";

/** Helpers for pages with values per branch (stock, prices) limited to the user's branches. */

export type BranchRef = { kode: string; nama: string };

/** Branch filter value meaning "every branch of the user". */
export const ALL_BRANCHES = "__all__";

const num = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
/** Formats a number the Indonesian way (175.000), "-" when empty. */
export const fmt = (v: number | null | undefined) => (v === null || v === undefined ? "-" : num.format(v));
export const orDash = (s: string | undefined) => (s ? s : "-");

/** Initial filter: the only branch, or all of them (like the original screens). */
export const initialBranch = (branches: BranchRef[]) => (branches.length === 1 ? branches[0].kode : ALL_BRANCHES);

/**
 * A column showing one value per branch: a plain value when one branch is
 * selected, one "KODE value" line per branch for "all". Export and search use
 * "KODE:value; KODE:value".
 */
export function perBranchColumn<T, S extends { cabang: string }>(opts: {
  key: string;
  header: string;
  rowsOf: (row: T) => S[];
  branch: string;
  pick: (s: S) => string;
  sortPick?: (s: S) => number | null;
  hidden?: boolean;
}): Column<T> {
  const { key, header, rowsOf, branch, pick, sortPick, hidden = false } = opts;
  const shown = (row: T) => (branch === ALL_BRANCHES ? rowsOf(row) : rowsOf(row).filter((s) => s.cabang === branch));
  return {
    key,
    header,
    hidden,
    value: (row) => {
      const rows = shown(row);
      if (branch !== ALL_BRANCHES) return rows[0] ? pick(rows[0]) : "-";
      return rows.map((s) => `${s.cabang}:${pick(s)}`).join("; ");
    },
    render: (row) => {
      const rows = shown(row);
      if (branch !== ALL_BRANCHES) return rows[0] ? pick(rows[0]) : <span className="muted">-</span>;
      if (rows.length === 0) return <span className="muted">-</span>;
      return (
        <ul className="branch-vals">
          {rows.map((s) => (
            <li key={s.cabang}>
              <span className="branch-vals-kode">{s.cabang}</span> {pick(s)}
            </li>
          ))}
        </ul>
      );
    },
    // Sorting only makes sense for one branch; with all branches it sorts by the first.
    sortValue: sortPick ? (row) => (shown(row)[0] ? sortPick(shown(row)[0]) : null) : undefined,
  };
}

/** "Cabang" dropdown for the table toolbar (a badge when the user has one branch). */
export function BranchFilter({
  branches,
  value,
  onChange,
}: {
  branches: BranchRef[];
  value: string;
  onChange: (kode: string) => void;
}) {
  if (branches.length === 0) return null;
  if (branches.length === 1) {
    return (
      <span className="dt-length">
        Cabang <span className="chip">{branches[0].kode}</span>
      </span>
    );
  }
  return (
    <label className="dt-length">
      <span>Cabang</span>
      <select className="select dt-branch" aria-label="Cabang" value={value} onChange={(e) => onChange(e.target.value)}>
        <option value={ALL_BRANCHES}>Semua cabang ({branches.length})</option>
        {branches.map((b) => (
          <option key={b.kode} value={b.kode}>
            {b.kode} · {b.nama}
          </option>
        ))}
      </select>
    </label>
  );
}

/** Notice for users who are not mapped to any branch. */
export function NoBranchNotice({ what }: { what: string }) {
  return (
    <p className="alert info" role="status">
      You are not assigned to any branch, so {what} per branch are hidden. An admin can assign branches under
      Management › User.
    </p>
  );
}
