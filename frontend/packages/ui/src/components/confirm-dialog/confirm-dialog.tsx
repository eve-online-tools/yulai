import { useEffect, useId, useRef, type ReactNode } from "react";
import { Button } from "@xaroth.nl/design/react";
import styles from "./confirm-dialog.module.scss";

export type ConfirmDialogProps = {
  open: boolean;
  onClose: () => void;
  onConfirm: () => void;
  title: string;
  children: ReactNode;
  confirmLabel: string;
  cancelLabel?: string;
  tone?: "danger" | "default";
  /** Disables both buttons and dismissal while the action runs. */
  pending?: boolean;
};

// Stopgap until @xaroth.nl/design ships one with this API: https://github.com/Xaroth/design/issues/11
export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  children,
  confirmLabel,
  cancelLabel = "Cancel",
  tone = "default",
  pending = false,
}: ConfirmDialogProps) {
  const ref = useRef<HTMLDialogElement>(null);
  const id = useId();

  useEffect(() => {
    const d = ref.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  return (
    <dialog
      ref={ref}
      role="alertdialog"
      aria-labelledby={`${id}-title`}
      aria-describedby={`${id}-body`}
      className={styles.dialog}
      // Escape fires cancel; keep the dialog controlled by open.
      onCancel={(e) => {
        e.preventDefault();
        if (!pending) onClose();
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget && !pending) onClose();
      }}
    >
      <div className={styles.inner}>
        <h2 id={`${id}-title`} className={styles.title}>
          {title}
        </h2>
        <div id={`${id}-body`}>{children}</div>
        <div className={styles.actions}>
          {/* Danger confirms start on Cancel so Enter does not destroy anything. */}
          <Button variant="tertiary" disabled={pending} autoFocus={tone === "danger"} onClick={onClose}>
            {cancelLabel}
          </Button>
          <Button tone={tone === "danger" ? "danger" : undefined} disabled={pending} autoFocus={tone !== "danger"} onClick={onConfirm}>
            {confirmLabel}
          </Button>
        </div>
      </div>
    </dialog>
  );
}
