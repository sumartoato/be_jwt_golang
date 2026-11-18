import { useState } from "react";
import { FileText, Folder, FolderOpen, SquareMinus, SquarePlus } from "lucide-react";
import type { MenuItem } from "../api";

/**
 * Read-only menu tree with folder icons and connector lines.
 * Groups start expanded (defaultOpen); remount it with a new key to expand or collapse all.
 */
export default function MenuTree({ items, defaultOpen = true }: { items: MenuItem[]; defaultOpen?: boolean }) {
  return (
    <ul className="tree" role="tree">
      {items.map((it) => (
        <TreeNode key={it.id} item={it} defaultOpen={defaultOpen} />
      ))}
    </ul>
  );
}

function TreeNode({ item, defaultOpen }: { item: MenuItem; defaultOpen: boolean }) {
  const [open, setOpen] = useState(defaultOpen);
  const isGroup = item.children.length > 0;
  const Icon = isGroup ? (open ? FolderOpen : Folder) : item.path ? FileText : Folder;

  return (
    <li role="treeitem" aria-expanded={isGroup ? open : undefined} aria-selected={false}>
      <div className="tree-row">
        {isGroup ? (
          <button
            type="button"
            className="tree-toggle"
            aria-label={`${open ? "Collapse" : "Expand"} ${item.title}`}
            onClick={() => setOpen((o) => !o)}
          >
            {open ? <SquareMinus aria-hidden="true" /> : <SquarePlus aria-hidden="true" />}
          </button>
        ) : (
          <span className="tree-toggle-spacer" />
        )}
        <Icon aria-hidden="true" className={isGroup ? "tree-icon folder" : "tree-icon page"} />
        <span className="tree-title">{item.title}</span>
        {isGroup && <span className="tree-count">{countPages(item)}</span>}
      </div>
      {isGroup && open && (
        <ul role="group">
          {item.children.map((c) => (
            <TreeNode key={c.id} item={c} defaultOpen={defaultOpen} />
          ))}
        </ul>
      )}
    </li>
  );
}

/** Number of pages (items with a path) under a node, including itself. */
export function countPages(item: MenuItem): number {
  return (item.path ? 1 : 0) + item.children.reduce((n, c) => n + countPages(c), 0);
}
