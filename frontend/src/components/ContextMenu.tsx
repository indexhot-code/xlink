import { useEffect, useRef } from "react";

export interface MenuItem {
  label: string;
  onClick: () => void;
  danger?: boolean;
}

/** ContextMenu：轻量右键菜单（点击外部/Esc 关闭，自动防屏幕溢出）。 */
export default function ContextMenu({
  x,
  y,
  items,
  onClose,
}: {
  x: number;
  y: number;
  items: MenuItem[];
  onClose: () => void;
}) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    };
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("mousedown", onDown);
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("mousedown", onDown);
      window.removeEventListener("keydown", onKey);
    };
  }, [onClose]);

  const w = 150;
  const h = items.length * 34 + 8;
  return (
    <div
      ref={ref}
      className="ctx-menu"
      style={{
        left: Math.min(x, window.innerWidth - w - 8),
        top: Math.min(y, window.innerHeight - h - 8),
      }}
    >
      {items.map((it) => (
        <button
          key={it.label}
          className={it.danger ? "danger" : ""}
          onClick={() => {
            onClose();
            it.onClick();
          }}
        >
          {it.label}
        </button>
      ))}
    </div>
  );
}
