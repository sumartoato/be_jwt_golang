import { ReactNode, useEffect, useId, useMemo, useRef, useState } from "react";
import { ArrowDown, ArrowUp, ArrowUpDown, ChevronLeft, ChevronRight } from "lucide-react";
import { copyToClipboard, downloadCsv, downloadPdf, downloadXlsx, printSheet, Sheet } from "./exporters";

export interface Column<T> {
  key: string;
  header: string;
  /** Plain text used for sorting, searching and exports. */
  value: (row: T) => string;
  /** Cell content; defaults to value(row). */
  render?: (row: T) => ReactNode;
  sortable?: boolean; // default true
  exportable?: boolean; // default true; false for e.g. an action column
  /** Start hidden; the user can show it under "Column visibility". */
  hidden?: boolean;
  /** Sort key when value() is not right for ordering, e.g. a number shown as "1.225.000". */
  sortValue?: (row: T) => number | string | null;
  className?: string;
}

interface Props<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  /** Used as the export file name and print title. */
  title: string;
  initialSort?: { key: string; dir: "asc" | "desc" };
  pageSizes?: number[];
  defaultPageSize?: number;
  /** Extra controls shown next to "records", e.g. a filter dropdown. */
  toolbarStart?: ReactNode;
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

/** Table with paging, search, sorting, column visibility and Copy/Excel/CSV/PDF/Print exports. */
export default function DataTable<T>({
  columns,
  rows,
  rowKey,
  title,
  initialSort,
  pageSizes = [10, 20, 50, 100],
  defaultPageSize = 20,
  toolbarStart,
}: Props<T>) {
  const [query, setQuery] = useState("");
  const [sort, setSort] = useState(initialSort ?? null);
  const [pageSize, setPageSize] = useState(defaultPageSize);
  const [page, setPage] = useState(1);
  const [hidden, setHidden] = useState<Set<string>>(() => new Set(columns.filter((c) => c.hidden).map((c) => c.key)));
  const [notice, setNotice] = useState("");
  const [colMenuOpen, setColMenuOpen] = useState(false);
  const colMenuRef = useRef<HTMLDivElement>(null);
  const id = useId();

  const visible = columns.filter((c) => !hidden.has(c.key));

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    const searchCols = columns.filter((c) => !hidden.has(c.key));
    let out = q ? rows.filter((r) => searchCols.some((c) => c.value(r).toLowerCase().includes(q))) : rows;
    const col = sort && columns.find((c) => c.key === sort.key);
    if (col && sort) {
      const dir = sort.dir === "asc" ? 1 : -1;
      const key = col.sortValue ?? col.value;
      out = [...out].sort((a, b) => {
        const x = key(a);
        const y = key(b);
        // Empty values go last in both directions.
        if (x === null || x === "") return y === null || y === "" ? 0 : 1;
        if (y === null || y === "") return -1;
        if (typeof x === "number" && typeof y === "number") return dir * (x - y);
        return dir * collator.compare(String(x), String(y));
      });
    }
    return out;
  }, [rows, columns, hidden, query, sort]);

  const pageCount = Math.max(1, Math.ceil(filtered.length / pageSize));
  const current = Math.min(page, pageCount);
  const start = (current - 1) * pageSize;
  const pageRows = filtered.slice(start, start + pageSize);

  useEffect(() => setPage(1), [query, pageSize]);

  useEffect(() => {
    if (!notice) return;
    const t = setTimeout(() => setNotice(""), 2500);
    return () => clearTimeout(t);
  }, [notice]);

  // Close the column menu on outside click or Escape.
  useEffect(() => {
    if (!colMenuOpen) return;
    const onDown = (e: MouseEvent) => {
      if (!colMenuRef.current?.contains(e.target as Node)) setColMenuOpen(false);
    };
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && setColMenuOpen(false);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [colMenuOpen]);

  const toggleSort = (key: string) =>
    setSort((s) => (s?.key === key ? { key, dir: s.dir === "asc" ? "desc" : "asc" } : { key, dir: "asc" }));

  /** Exports every filtered row (not just this page) in the current order. */
  const sheet = (): Sheet => {
    const cols = visible.filter((c) => c.exportable !== false);
    return { title, head: cols.map((c) => c.header), body: filtered.map((r) => cols.map((c) => c.value(r))) };
  };

  const run = async (label: string, fn: () => unknown) => {
    try {
      await fn();
      setNotice(label);
    } catch {
      setNotice("Export failed. Please try again.");
    }
  };

  return (
    <div className="dt">
      <div className="dt-toolbar">
        <label className="dt-length">
          <select className="select" value={pageSize} onChange={(e) => setPageSize(Number(e.target.value))}>
            {pageSizes.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
          <span>records</span>
        </label>
        {toolbarStart}

        <div className="dt-tools">
          <label className="dt-search">
            <span>Search:</span>
            <input type="search" value={query} onChange={(e) => setQuery(e.target.value)} />
          </label>
          <div className="dt-buttons" role="group" aria-label="Export">
            <button type="button" onClick={() => run(`Copied ${filtered.length} rows`, () => copyToClipboard(sheet()))}>
              Copy
            </button>
            <button type="button" onClick={() => run("Excel file downloaded", () => downloadXlsx(sheet()))}>
              Excel
            </button>
            <button type="button" onClick={() => run("CSV file downloaded", () => downloadCsv(sheet()))}>
              CSV
            </button>
            <button type="button" onClick={() => run("PDF file downloaded", () => downloadPdf(sheet()))}>
              PDF
            </button>
            <button
              type="button"
              onClick={() =>
                run("Print dialog opened", () => {
                  if (!printSheet(sheet())) throw new Error("popup blocked");
                })
              }
            >
              Print
            </button>
            <div className="dt-colvis" ref={colMenuRef}>
              <button
                type="button"
                aria-expanded={colMenuOpen}
                aria-controls={`${id}-cols`}
                onClick={() => setColMenuOpen((o) => !o)}
              >
                Column visibility
              </button>
              {colMenuOpen && (
                <div className="dt-colvis-menu" id={`${id}-cols`}>
                  {columns.map((c) => (
                    <label key={c.key}>
                      <input
                        type="checkbox"
                        checked={!hidden.has(c.key)}
                        // Keep at least one column.
                        disabled={!hidden.has(c.key) && visible.length === 1}
                        onChange={() =>
                          setHidden((h) => {
                            const next = new Set(h);
                            if (next.has(c.key)) next.delete(c.key);
                            else next.add(c.key);
                            return next;
                          })
                        }
                      />
                      {c.header}
                    </label>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
      <p className="dt-notice" role="status" aria-live="polite">
        {notice}
      </p>

      <div className="table-wrap">
        <table className="data-table dt-table">
          <thead>
            <tr>
              {visible.map((c) => {
                const active = sort?.key === c.key;
                const sortable = c.sortable !== false;
                return (
                  <th
                    key={c.key}
                    scope="col"
                    className={c.className}
                    aria-sort={active ? (sort!.dir === "asc" ? "ascending" : "descending") : undefined}
                  >
                    {sortable ? (
                      <button type="button" className="dt-sort" onClick={() => toggleSort(c.key)}>
                        <span>{c.header}</span>
                        {active ? (
                          sort!.dir === "asc" ? <ArrowUp aria-hidden="true" /> : <ArrowDown aria-hidden="true" />
                        ) : (
                          <ArrowUpDown aria-hidden="true" className="dt-sort-idle" />
                        )}
                      </button>
                    ) : (
                      c.header
                    )}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody>
            {pageRows.length === 0 ? (
              <tr>
                <td colSpan={visible.length} className="dt-empty">
                  {rows.length === 0 ? "No data available" : "No matching records found"}
                </td>
              </tr>
            ) : (
              pageRows.map((r) => (
                <tr key={rowKey(r)}>
                  {visible.map((c) => (
                    <td key={c.key} className={c.className}>
                      {c.render ? c.render(r) : c.value(r)}
                    </td>
                  ))}
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      <div className="dt-footer">
        <p className="dt-info">
          {filtered.length === 0
            ? "Showing 0 to 0 of 0 entries"
            : `Showing ${start + 1} to ${start + pageRows.length} of ${filtered.length} entries`}
          {filtered.length !== rows.length && ` (filtered from ${rows.length} total entries)`}
        </p>
        <nav className="dt-pages" aria-label="Pagination">
          <button type="button" aria-label="Previous page" disabled={current === 1} onClick={() => setPage(current - 1)}>
            <ChevronLeft aria-hidden="true" />
          </button>
          {pageList(current, pageCount).map((p, i) =>
            p === "…" ? (
              <span key={`gap-${i}`} className="dt-gap">
                …
              </span>
            ) : (
              <button
                key={p}
                type="button"
                className={p === current ? "active" : undefined}
                aria-current={p === current ? "page" : undefined}
                onClick={() => setPage(p)}
              >
                {p}
              </button>
            ),
          )}
          <button
            type="button"
            aria-label="Next page"
            disabled={current === pageCount}
            onClick={() => setPage(current + 1)}
          >
            <ChevronRight aria-hidden="true" />
          </button>
        </nav>
      </div>
    </div>
  );
}

/** Page numbers to show: first, last, and the current page with its neighbours. */
function pageList(current: number, count: number): (number | "…")[] {
  const pages = new Set([1, count, current - 1, current, current + 1].filter((p) => p >= 1 && p <= count));
  const sorted = [...pages].sort((a, b) => a - b);
  const out: (number | "…")[] = [];
  sorted.forEach((p, i) => {
    if (i > 0 && p - sorted[i - 1] > 1) out.push("…");
    out.push(p);
  });
  return out;
}
