import { useCallback, useEffect, useRef, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";
import { getPane, setLocalPath, setRemotePath } from "../stores/sftp";
import type { RemoteEntry } from "../../bindings/xlink/backend/sftp/models.js";

type LocalEntry = RemoteEntry;

/** 地址栏：上级 / 可编辑路径（回车跳转）/ 刷新 / 附加控件（盘符下拉等）。 */
function AddressBar({
  path,
  onGo,
  extra,
}: {
  path: string;
  onGo: (p: string) => void;
  extra?: React.ReactNode;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(path);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (!editing) setDraft(path);
  }, [path, editing]);

  const commit = () => {
    const p = draft.trim();
    setEditing(false);
    if (p && p !== path) onGo(p);
  };

  const up = () => {
    const norm = path.replace(/[\\/]+$/, "");
    const idx = Math.max(norm.lastIndexOf("\\"), norm.lastIndexOf("/"));
    if (idx < 0) return; // 已在根，无法再上
    if (idx <= 2 && norm[1] === ":") { onGo(norm.slice(0, 3)); return; } // 盘符根 "C:\"
    onGo(norm.slice(0, idx) || "/");
  };

  return (
    <div className="pane-head">
      <button title="上级目录" onClick={up}>↑</button>
      {editing ? (
        <input
          ref={inputRef}
          className="path-input"
          value={draft}
          autoFocus
          onChange={(e) => setDraft(e.target.value)}
          onBlur={commit}
          onKeyDown={(e) => {
            if (e.key === "Enter") commit();
            if (e.key === "Escape") setEditing(false);
          }}
        />
      ) : (
        <span className="path" title="点击编辑路径" onClick={() => setEditing(true)}>
          {path}
        </span>
      )}
      <button title="刷新" onClick={() => onGo(path)}>⟳</button>
      {extra}
    </div>
  );
}

/**
 * SFTPDualPane：双栏文件管理器（本地 + 远程）。
 * 导航：盘符下拉 / 可编辑地址栏 / 上级 / 刷新；双击进目录，单击文件入队。
 * 拖拽：本地行 → 远程栏 = 上传；远程行 → 本地栏 = 下载。目录递归入队。
 */
export default function SFTPDualPane({ sessionId, tabKey }: { sessionId: string; tabKey: string }) {
  const pane = getPane(tabKey);
  const [localEntries, setLocalEntries] = useState<LocalEntry[]>([]);
  const [remoteEntries, setRemoteEntries] = useState<RemoteEntry[]>([]);
  const [drives, setDrives] = useState<string[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dropHint, setDropHint] = useState<"left" | "right" | null>(null);
  const draggedLocal = useRef<{ path: string; isDir: boolean; name: string } | null>(null);

  const loadLocal = useCallback(async (p: string) => {
    try {
      setLocalEntries(((await SftpService.ListLocalDir(p)) as never[]) || []);
      setLocalPath(tabKey, p);
      setError(null);
    } catch (e) {
      setError(String(e));
    }
  }, [tabKey]);

  const loadRemote = useCallback(async (p: string) => {
    try {
      setRemoteEntries(((await SftpService.ListDir(sessionId, p)) as never[]) || []);
      setRemotePath(tabKey, p);
      setError(null);
    } catch (e) {
      setError(String(e));
    }
  }, [sessionId, tabKey]);

  useEffect(() => {
    SftpService.ListDrives()
      .then((d: string[] | null) => setDrives(d || []))
      .catch(() => setDrives([]));
    loadLocal(pane.localPath);
    loadRemote(pane.remotePath);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  const localJoin = (name: string) =>
    pane.localPath.replace(/[\\/]+$/, "") + (pane.localPath.includes("\\") ? "\\" : "/") + name;

  const upload = async (localPath: string, isDir: boolean, name: string) => {
    setBusy(true);
    setError(null);
    try {
      const target = pane.remotePath.replace(/\/+$/, "") + "/" + name;
      await SftpService.Enqueue(1, sessionId, localPath, target);
      window.dispatchEvent(new CustomEvent("sftp-queue-changed"));
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const download = async (remotePath: string, isDir: boolean, name: string) => {
    setBusy(true);
    setError(null);
    try {
      const sep = pane.localPath.includes("\\") ? "\\" : "/";
      const target = pane.localPath.replace(/[\\/]+$/, "") + sep + name;
      await SftpService.Enqueue(2, sessionId, target, remotePath);
      window.dispatchEvent(new CustomEvent("sftp-queue-changed"));
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const driveSelect = (
    <select
      className="drive-select"
      title="切换盘符"
      value={drives.includes(pane.localPath.slice(0, 3)) ? pane.localPath.slice(0, 3) : ""}
      onChange={(e) => loadLocal(e.target.value)}
    >
      {!drives.includes(pane.localPath.slice(0, 3)) && <option value="">盘符</option>}
      {drives.map((d) => (
        <option key={d} value={d}>{d}</option>
      ))}
    </select>
  );

  return (
    <div className="dualpane">
      {error && <p className="error">{error}</p>}
      <div className="dual-cols">
        <div
          className={"pane" + (dropHint === "left" ? " drop" : "")}
          onDragOver={(e) => { e.preventDefault(); setDropHint("left"); }}
          onDragLeave={() => setDropHint(null)}
          onDrop={(e) => {
            e.preventDefault();
            setDropHint(null);
            const rp = e.dataTransfer.getData("application/x-remote");
            if (!rp) return;
            const isDir = e.dataTransfer.getData("application/x-remote-dir") === "1";
            download(rp, isDir, rp.split("/").pop() || "file");
          }}
        >
          <AddressBar path={pane.localPath} onGo={loadLocal} extra={driveSelect} />
          <ul>
            {localEntries.map((f) => (
              <li
                key={f.name}
                draggable
                onDragStart={(e) => {
                  const p = localJoin(f.name);
                  draggedLocal.current = { path: p, isDir: f.isDir, name: f.name };
                  e.dataTransfer.setData("text/plain", f.name);
                  e.dataTransfer.setData("application/x-local-dir", f.isDir ? "1" : "0");
                }}
                onDragEnd={() => { draggedLocal.current = null; }}
                onDoubleClick={() => f.isDir && loadLocal(localJoin(f.name))}
                onClick={() => {
                  if (!f.isDir) upload(localJoin(f.name), false, f.name);
                }}
              >
                {f.isDir ? "📁" : "📄"} {f.name}
              </li>
            ))}
          </ul>
        </div>

        <div
          className={"pane" + (dropHint === "right" ? " drop" : "")}
          onDragOver={(e) => { e.preventDefault(); setDropHint("right"); }}
          onDragLeave={() => setDropHint(null)}
          onDrop={(e) => {
            e.preventDefault();
            setDropHint(null);
            const d = draggedLocal.current;
            if (d) {
              draggedLocal.current = null;
              upload(d.path, d.isDir, d.name);
              return;
            }
            const name = e.dataTransfer.getData("text/plain");
            if (name) upload(localJoin(name), false, name);
          }}
        >
          <AddressBar
            path={pane.remotePath}
            onGo={loadRemote}
            extra={
              <button title="回到根目录" onClick={() => loadRemote("/")}>/</button>
            }
          />
          <ul>
            {remoteEntries.map((f) => (
              <li
                key={f.name}
                draggable
                onDragStart={(e) => {
                  e.dataTransfer.setData("application/x-remote", pane.remotePath.replace(/\/+$/, "") + "/" + f.name);
                  e.dataTransfer.setData("application/x-remote-dir", f.isDir ? "1" : "0");
                }}
                onDoubleClick={() => f.isDir && loadRemote(pane.remotePath.replace(/\/+$/, "") + "/" + f.name)}
                onClick={() => {
                  if (!f.isDir) download(pane.remotePath.replace(/\/+$/, "") + "/" + f.name, false, f.name);
                }}
              >
                {f.isDir ? "📁" : "📄"} {f.name}
              </li>
            ))}
          </ul>
        </div>
      </div>
      {busy && <p className="hint-inline">入队中…</p>}
    </div>
  );
}
