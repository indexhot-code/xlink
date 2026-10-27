import { useState } from "react";

/** ConfirmDialog：危险操作确认（如删除）。 */
export default function ConfirmDialog({
  title,
  message,
  items,
  confirmText = "确认",
  onConfirm,
  onCancel,
}: {
  title: string;
  message: string;
  items?: string[];
  confirmText?: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const doConfirm = async () => {
    setBusy(true);
    try {
      await onConfirm();
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="dialog-backdrop" onClick={onCancel}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>{title}</h2>
        <p className="confirm-msg">{message}</p>
        {items && items.length > 0 && (
          <ul className="confirm-items">
            {items.map((it) => (
              <li key={it}>{it}</li>
            ))}
          </ul>
        )}
        <div className="dialog-actions">
          <button className="secondary" onClick={onCancel}>
            取消
          </button>
          <button className="danger-btn" onClick={doConfirm} disabled={busy}>
            {busy ? "处理中…" : confirmText}
          </button>
        </div>
      </div>
    </div>
  );
}
