import { useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";
import DirTree from "./DirTree";

/** RemotePickDialog：「上传到…」目标目录选择器（内嵌远端目录树，单选）。 */
export default function RemotePickDialog({ sessionId, initialPath, onPick, onCancel }: {
  sessionId: string;
  initialPath: string;
  onPick: (dir: string) => void;
  onCancel: () => void;
}) {
  const [target, setTarget] = useState(initialPath);

  return (
    <div className="dialog-backdrop" onClick={onCancel}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>上传到…</h2>
        <div className="pick-tree">
          <DirTree
            sessionId={sessionId}
            side="remote"
            roots={[{ label: "/", path: "/" }]}
            current={target}
            onNavigate={(p) => setTarget(p)}
          />
        </div>
        <p className="confirm-msg">目标目录：{target}</p>
        <div className="dialog-actions">
          <button className="secondary" onClick={onCancel}>
            取消
          </button>
          <button onClick={() => onPick(target)}>上传到此处</button>
        </div>
      </div>
    </div>
  );
}

/** 待上传项（供 onPick 后批量入队）。 */
export type PendingUpload = { paths: string[] };

export async function enqueueUploads(
  sessionId: string,
  paths: string[],
  targetDir: string,
): Promise<void> {
  for (const p of paths) {
    const name = p.split(/[\\/]/).filter(Boolean).pop() || "file";
    const target = targetDir.replace(/\/+$/, "") + "/" + name;
    await SftpService.Enqueue(1, sessionId, p, target);
  }
  window.dispatchEvent(new CustomEvent("sftp-queue-changed"));
}
