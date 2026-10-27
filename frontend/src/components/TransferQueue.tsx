import { useCallback, useEffect, useRef, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";

type Transfer = { id: number; direction: number; localPath: string; remotePath: string; size: number; transferred: number; state: string; error: string };

const MIN_W = 220;
const MAX_W = 600;
const W_KEY = "xlink.queueWidth";

/** TransferQueue：500ms 轮询 ListTransfers（分块进度不走事件，规则 2）。
 *  左缘拖拽条可调整队列面板宽度（持久化）。 */
export default function TransferQueue() {
  const [list, setList] = useState<Transfer[]>([]);
  const [width, setWidth] = useState(() => {
    const w = Number(localStorage.getItem(W_KEY));
    return w >= MIN_W && w <= MAX_W ? w : 300;
  });
  const dragRef = useRef<{ startX: number; startW: number } | null>(null);

  const onResizerDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragRef.current = { startX: e.clientX, startW: width };
      const move = (ev: MouseEvent) => {
        if (!dragRef.current) return;
        const w = Math.min(MAX_W, Math.max(MIN_W, dragRef.current.startW + (dragRef.current.startX - ev.clientX)));
        setWidth(w);
      };
      const up = () => {
        dragRef.current = null;
        window.removeEventListener("mousemove", move);
        window.removeEventListener("mouseup", up);
        document.body.style.cursor = "";
        document.body.style.userSelect = "";
        setWidth((w) => {
          localStorage.setItem(W_KEY, String(w));
          return w;
        });
      };
      document.body.style.cursor = "col-resize";
      document.body.style.userSelect = "none";
      window.addEventListener("mousemove", move);
      window.addEventListener("mouseup", up);
    },
    [width],
  );

  useEffect(() => {
    let alive = true;
    const poll = async () => {
      try {
        const l = ((await SftpService.ListTransfers()) as never[]) || [];
        if (alive) setList(l as Transfer[]);
      } catch {
        /* 忽略 */
      }
    };
    poll();
    const timer = setInterval(poll, 500);
    const onChange = () => poll();
    window.addEventListener("sftp-queue-changed", onChange);
    return () => {
      alive = false;
      clearInterval(timer);
      window.removeEventListener("sftp-queue-changed", onChange);
    };
  }, []);

  const act = async (fn: Promise<unknown>) => {
    await fn;
    setList(((await SftpService.ListTransfers()) as never[]) || []);
  };

  if (list.length === 0) return null;

  return (
    <>
      <div className="queue-resizer" title="拖动调整宽度" onMouseDown={onResizerDown} />
      <div className="transfer-queue" style={{ width, flexShrink: 0 }}>
        <div className="queue-head">传输队列</div>
        <ul>
          {list.map((t) => {
            const pct = t.size > 0 ? Math.min(100, Math.round((t.transferred / t.size) * 100)) : 0;
            const arrow = t.direction === 1 ? "↑" : "↓";
            return (
              <li key={String(t.id)}>
                <span className={"dot " + (t.state === "success" ? "connected" : t.state === "running" ? "connecting" : "error")} />
                <span className="t-arrow">{arrow}</span>
                <span className="t-name" title={t.localPath + " → " + t.remotePath}>
                  {(t.direction === 1 ? t.remotePath : t.localPath).split(/[\\/]/).pop()}
                </span>
                <progress value={pct} max={100} />
                <span className="t-pct">{pct}%</span>
                <span className="t-state">{t.state}</span>
                {t.error && <span className="t-err" title={t.error}>!</span>}
                <span className="t-actions">
                  {t.state === "running" && (
                    <button onClick={() => act(SftpService.Pause(t.id))}>暂停</button>
                  )}
                  {(t.state === "paused" || t.state === "canceled" || t.state === "error") && (
                    <button onClick={() => act(SftpService.Resume(t.id))}>继续</button>
                  )}
                  {(t.state === "running" || t.state === "paused") && (
                    <button onClick={() => act(SftpService.Cancel(t.id))}>取消</button>
                  )}
                  {t.state === "success" && (
                    <button onClick={() => act(SftpService.Retry(t.id))}>重传</button>
                  )}
                </span>
              </li>
            );
          })}
        </ul>
      </div>
    </>
  );
}