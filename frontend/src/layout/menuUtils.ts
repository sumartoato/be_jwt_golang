import type { MenuItem } from "../api";

function normalize(path: string): string {
  return path.length > 1 ? path.replace(/\/+$/, "") : path;
}

/** Returns the chain of items from a root down to the item whose path matches, or []. */
export function findTrail(items: MenuItem[], pathname: string): MenuItem[] {
  const target = normalize(pathname);
  for (const item of items) {
    if (item.path !== null && normalize(item.path) === target) return [item];
    const sub = findTrail(item.children, target);
    if (sub.length > 0) return [item, ...sub];
  }
  return [];
}

/** Sub-pages of a menu page (its path plus one segment) and their breadcrumb label. */
const SUB_PAGES: Record<string, string> = { new: "Add" };

/**
 * Like findTrail, but also accepts a sub-page such as /transaksi-klinik/pengobatan/new:
 * it returns the parent page's trail plus the sub-page's label. Access follows the parent menu.
 */
export function findPageTrail(items: MenuItem[], pathname: string): { items: MenuItem[]; sub?: string } {
  const exact = findTrail(items, pathname);
  if (exact.length > 0) return { items: exact };
  const path = normalize(pathname);
  const cut = path.lastIndexOf("/");
  const label = SUB_PAGES[path.slice(cut + 1)];
  if (cut > 0 && label) {
    const parent = findTrail(items, path.slice(0, cut));
    if (parent.length > 0) return { items: parent, sub: label };
  }
  return { items: [] };
}
