import { ReactNode, useDeferredValue, useEffect, useId, useMemo, useRef, useState } from "react";
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
  /** Header content for a non-sortable column (e.g. a select-all checkbox); header stays the text for exports. */
  headerNode?: ReactNode;
}

/** What a server-side table asks the server for. */
export interface RemoteQuery {
  page: number;
  pageSize: number;
  search: string;
  sort: { key: string; dir: "asc" | "desc" } | null;
}

/** Server-side mode: rows is the current page; paging, search and sort happen on the server. */
export interface RemoteOptions<T> {
  /** Rows matching the search. */
  total: number;
  /** Rows without a search (for "filtered from N"). */
  unfilteredTotal: number;
  loading?: boolean;
  /** Called (search debounced) whenever the page, size, search or sort changes. */
  onQuery: (q: RemoteQuery) => void;
  /** Rows to export: everything matching the search (the server may cap it). Defaults to the current page. */
  fetchAll?: (q: RemoteQuery) => Promise<T[]>;
}

interface Props<T> {
  columns: Column<T>[];
  rows: T[];
  rowKey: (row: T) => string | number;
  /** Used as the export file name and print title. */
  title: string;
  initialSort?: { key: string; dir: "asc" | "desc" };
  /** Choices for "records"; ALL_ROWS (0) shows every row on one page. */
  pageSizes?: number[];
  defaultPageSize?: number;
  /** Extra controls shown next to "records", e.g. a filter dropdown. */
  toolbarStart?: ReactNode;
  /** Server-side paging/search/sort for tables too large to load at once. */
  remote?: RemoteOptions<T>;
  /** Show the Copy/Excel/CSV/PDF/Print/Column visibility buttons (default true). */
  exports?: boolean;
}

const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

const num = new Intl.NumberFormat("en-US");

/** Page size meaning "show all rows". */
export const ALL_ROWS = 0;

/** Table with paging, search, sorting, column visibility and Copy/Excel/CSV/PDF/Print exports. */
export default function DataTable<T>({
  columns,
  rows,
  rowKey,
  title,
  initialSort,
  pageSizes = [5, 10, 20, 50, 100, ALL_ROWS],
  defaultPageSize = 20,
  toolbarStart,
  remote,
  exports = true,
}: Props<T>) {
  const [query, setQuery] = useState("");
  // The table filters on a deferred copy, so typing stays responsive even when
  // "All" renders thousands of rows.
  const deferredQuery = useDeferredValue(query);
  const [sort, setSort] = useState(initialSort ?? null);
  const [pageSize, setPageSize] = useState(defaultPageSize);
  const [page, setPage] = useState(1);
  const [hidden, setHidden] = useState<Set<string>>(() => new Set(columns.filter((c) => c.hidden).map((c) => c.key)));
  const [notice, setNotice] = useState("");
  const [colMenuOpen, setColMenuOpen] = useState(false);
  const colMenuRef = useRef<HTMLDivElement>(null);
  const id = useId();

  // Server mode: send the search after the user pauses typing.
  const [remoteSearch, setRemoteSearch] = useState("");
  useEffect(() => {
    if (!remote) return;
    const t = setTimeout(() => setRemoteSearch(query.trim()), 350);
    return () => clearTimeout(t);
  }, [query, !!remote]); // eslint-disable-line react-hooks/exhaustive-deps

  const visible = columns.filter((c) => !hidden.has(c.key));

  const filtered = useMemo(() => {
    if (remote) return rows; // the server already filtered and sorted
    const q = deferredQuery.trim().toLowerCase();
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
  }, [rows, columns, hidden, deferredQuery, sort, !!remote]); // eslint-disable-line react-hooks/exhaustive-deps

  const matching = remote ? remote.total : filtered.length;
  const size = pageSize === ALL_ROWS ? Math.max(1, matching) : pageSize;
  const pageCount = Math.max(1, Math.ceil(matching / size));
  const current = Math.min(page, pageCount);
  const start = (current - 1) * size;
  const pageRows = remote ? rows : filtered.slice(start, start + size);
  const allRows = remote ? remote.unfilteredTotal : rows.length;
  const sizes = remote ? pageSizes.filter((n) => n !== ALL_ROWS) : pageSizes;

  useEffect(() => setPage(1), [query, pageSize, remote ? sort : null]); // eslint-disable-line react-hooks/exhaustive-deps

  const remoteQuery: RemoteQuery = { page: current, pageSize: size, search: remoteSearch, sort };
  // onQuery may change identity on every parent render; call the latest one.
  const onQueryRef = useRef(remote?.onQuery);
  onQueryRef.current = remote?.onQuery;
  useEffect(() => {
    onQueryRef.current?.(remoteQuery);
  }, [remote ? current : 0, remote ? size : 0, remoteSearch, remote ? sort?.key : "", remote ? sort?.dir : ""]); // eslint-disable-line react-hooks/exhaustive-deps

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

  /** Exports every matching row (not just this page) in the current order. */
  const sheet = async (): Promise<Sheet> => {
    const cols = visible.filter((c) => c.exportable !== false);
    const data = remote ? (remote.fetchAll ? await remote.fetchAll(remoteQuery) : rows) : filtered;
    return { title, head: cols.map((c) => c.header), body: data.map((r) => cols.map((c) => c.value(r))) };
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
          <select
            className="select"
            aria-label="Records per page"
            value={pageSize}
            onChange={(e) => setPageSize(Number(e.target.value))}
          >
            {sizes.map((n) => (
              <option key={n} value={n}>
                {n === ALL_ROWS ? "All" : n}
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
          {exports && <div className="dt-buttons" role="group" aria-label="Export">
            <button type="button" onClick={() => run("Copied to the clipboard", async () => copyToClipboard(await sheet()))}>
              Copy
            </button>
            <button type="button" onClick={() => run("Excel file downloaded", async () => downloadXlsx(await sheet()))}>
              Excel
            </button>
            <button type="button" onClick={() => run("CSV file downloaded", async () => downloadCsv(await sheet()))}>
              CSV
            </button>
            <button type="button" onClick={() => run("PDF file downloaded", async () => downloadPdf(await sheet()))}>
              PDF
            </button>
            <button
              type="button"
              onClick={() =>
                run("Print dialog opened", async () => {
                  if (!printSheet(await sheet())) throw new Error("popup blocked");
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
          </div>}
        </div>
      </div>
      <p className="dt-notice" role="status" aria-live="polite">
        {notice}
      </p>

      <div className="table-wrap">
        <table className={remote?.loading ? "data-table dt-table dt-loading" : "data-table dt-table"} aria-busy={remote?.loading || undefined}>
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
                      (c.headerNode ?? c.header)
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
                  {remote?.loading ? "Loading…" : allRows === 0 ? "No data available" : "No matching records found"}
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
          {matching === 0
            ? "Showing 0 to 0 of 0 entries"
            : `Showing ${num.format(start + 1)} to ${num.format(start + pageRows.length)} of ${num.format(matching)} entries`}
          {matching !== allRows && ` (filtered from ${num.format(allRows)} total entries)`}
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
