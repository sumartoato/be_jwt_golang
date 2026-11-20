/**
 * Printing a jsPDF document in the browser instead of downloading it.
 *
 * openPrintTab() must run directly in the click handler, before any await:
 * popup blockers only allow window.open during the click itself. The PDF is
 * built afterwards and shown in that tab with the print dialog open.
 */

/** Opens the tab the document will be printed from, with a "preparing" note. */
export function openPrintTab(): Window | null {
  const w = window.open("", "_blank");
  if (w) {
    w.document.title = "Menyiapkan cetakan…";
    w.document.body.innerHTML = '<p style="font:14px system-ui,sans-serif;padding:16px">Menyiapkan cetakan…</p>';
  }
  return w;
}

interface PrintableDoc {
  autoPrint(): unknown;
  output(type: "bloburl"): URL | string;
  save(filename: string): unknown;
  setProperties(p: { title: string }): unknown;
}

/**
 * Shows doc in tab with the print dialog open. Without a tab (blocked, closed,
 * or outside a browser) it downloads filename instead.
 */
export function printDoc(doc: PrintableDoc, filename: string, tab?: Window | null): void {
  if (!tab || tab.closed) {
    doc.save(filename);
    return;
  }
  doc.setProperties({ title: filename.replace(/\.pdf$/, "") });
  doc.autoPrint();
  tab.location.href = String(doc.output("bloburl"));
}
