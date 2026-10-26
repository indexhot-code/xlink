import { useEffect, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";

type Transfer = { id: number; direction: number; localPath: string; remotePath: string; size: number; transferred: number; state: string; error: string };

/** TransferQueue：500ms 轮询 ListTransfers（分块进度不走事件，规则 2）。 */
export default function TransferQueue() {
  const [list, setList] = useState<Transfer[]>([]);

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
    <div className="transfer-queue">
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
  );
}
