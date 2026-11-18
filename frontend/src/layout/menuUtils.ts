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
