import { FormEvent, ReactNode, RefObject, useEffect, useRef } from "react";
import { X } from "lucide-react";

/** Small building blocks for the add/edit dialogs of master-data pages. */

/** Ref for a <dialog> that opens as a modal on mount and closes on unmount. */
export function useModal() {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const d = ref.current;
    d?.showModal();
    return () => d?.close();
  }, []);
  return ref;
}

/** Modal form with a title, close button, error line and Cancel/Save footer. */
export function Modal({
  dialogRef,
  title,
  wide,
  onClose,
  onSubmit,
  error,
  busy,
  children,
}: {
  dialogRef: RefObject<HTMLDialogElement>;
  title: string;
  /** true: 820px; "xl": nearly the full window, for dialogs holding a table. */
  wide?: boolean | "xl";
  onClose: () => void;
  onSubmit: (e: FormEvent) => void;
  error: string;
  busy: boolean;
  children: ReactNode;
}) {
  return (
    <dialog ref={dialogRef} className={wide === "xl" ? "modal modal-xl" : wide ? "modal modal-wide" : "modal"} aria-labelledby="form-dialog-title" onClose={onClose}>
      <form onSubmit={onSubmit} noValidate>
        <header className="modal-head">
          <h2 id="form-dialog-title">{title}</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">{children}</div>
        {error && (
          <p className="alert error modal-alert" role="alert">
            {error}
          </p>
        )}
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-small btn-primary" disabled={busy}>
            {busy ? "Saving…" : "Save"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}

/** Label + control + error message. */
export function FieldShell({
  id,
  label,
  error,
  hint,
  wide,
  required,
  children,
}: {
  id: string;
  label: string;
  error?: string;
  hint?: string;
  wide?: boolean;
  required?: boolean;
  children: ReactNode;
}) {
  return (
    <div className={wide ? "field wide" : "field"}>
      <label htmlFor={id}>
        {label}
        {required && <span aria-hidden="true"> *</span>}
      </label>
      {children}
      {error ? (
        <span id={`${id}-error`} className="field-error">
          {error}
        </span>
      ) : (
        hint && <span className="field-hint">{hint}</span>
      )}
    </div>
  );
}

/** Number fields are edited as text so "" can mean empty (NULL). */
export const numText = (v: number | null) => (v === null ? "" : String(v));
export const textNum = (s: string) => (s.trim() === "" ? null : Number(s));

/**
 * Converts text fields to numbers. Returns the numbers, or the first field
 * that is not a number from 0.
 */
export function parseAmounts<K extends string>(vals: Record<K, string>): { ok: Record<K, number | null> } | { bad: K } {
  const ok = {} as Record<K, number | null>;
  for (const k of Object.keys(vals) as K[]) {
    const v = textNum(vals[k]);
    if (v !== null && (Number.isNaN(v) || v < 0)) return { bad: k };
    ok[k] = v;
  }
  return { ok };
}

/** Number input for an amount (from 0, decimals allowed). */
export function AmountInput({
  id,
  value,
  invalid,
  onChange,
}: {
  id: string;
  value: string;
  invalid?: boolean;
  onChange: (v: string) => void;
}) {
  return (
    <input
      id={id}
      type="number"
      inputMode="decimal"
      min={0}
      step="any"
      value={value}
      aria-invalid={invalid ? "true" : undefined}
      onChange={(e) => onChange(e.target.value)}
    />
  );
}
