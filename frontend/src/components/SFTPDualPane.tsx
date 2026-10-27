import { useCallback, useEffect, useRef, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";
import { getPane, setLocalPath, setRemotePath } from "../stores/sftp";
import type { RemoteEntry } from "../../bindings/xlink/backend/sftp/models.js";
import DirTree from "./DirTree";
import ContextMenu, { type MenuItem } from "./ContextMenu";
import ConfirmDialog from "./ConfirmDialog";
import RemotePickDialog from "./RemotePickDialog";

type LocalEntry = RemoteEntry;

// ---- 导航历史（后退/前进，资源管理器惯例） ----
interface Hist { stack: string[]; idx: number }
const initHist = (p: string): Hist => ({ stack: [p], idx: 0 });
const histCurrent = (h: Hist): string => h.stack[h.idx];
const histGo = (h: Hist, p: string): Hist => {
  const stack = [...h.stack.slice(0, h.idx + 1), p];
  return { stack, idx: stack.length - 1 };
};
const histMove = (h: Hist, d: number): Hist => ({
  stack: h.stack,
  idx: Math.min(Math.max(h.idx + d, 0), h.stack.length - 1),
});

function crumbs(p: string): { name: string; path: string }[] {
  const isWin = p.includes("\\") || /^[A-Za-z]:/.test(p);
  const norm = p.replace(/[\\/]+$/, "");
  const segs = norm.split(/[\\/]/).filter(Boolean);
  const out: { name: string; path: string }[] = [];
  let acc = "";
  segs.forEach((s, i) => {
    if (i === 0 && isWin) { acc = s + "\\"; out.push({ name: s, path: acc }); return; }
    if (i === 0) { acc = "/"; out.push({ name: "/", path: "/" }); return; }
    acc = acc.endsWith("\\") || acc.endsWith("/") ? acc + s : acc + "/" + s;
    out.push({ name: s, path: acc });
  });
  return out.length ? out : [{ name: "/", path: "/" }];
}

function NavBar({ path, canBack, canFwd, onBack, onFwd, onGo, extra }: {
  path: string;
  canBack: boolean;
  canFwd: boolean;
  onBack: () => void;
  onFwd: () => void;
  onGo: (p: string) => void;
  extra?: React.ReactNode;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(path);
  useEffect(() => { if (!editing) setDraft(path); }, [path, editing]);
  const commit = () => {
    const p = draft.trim();
    setEditing(false);
    if (p && p !== path) onGo(p);
  };
  const up = () => {
    const norm = path.replace(/[\\/]+$/, "");
    const idx = Math.max(norm.lastIndexOf("\\"), norm.lastIndexOf("/"));
    if (idx < 0) return;
    if (idx <= 2 && norm[1] === ":") { onGo(norm.slice(0, 3)); return; }
    onGo(norm.slice(0, idx) || "/");
  };
  return (
    <div className="pane-head">
      <button title="后退 (Alt+←)" disabled={!canBack} onClick={onBack}>←</button>
      <button title="前进 (Alt+→)" disabled={!canFwd} onClick={onFwd}>→</button>
      <button title="上级 (Backspace)" onClick={up}>↑</button>
      {editing ? (
        <input
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
        <div className="crumbs" title="点击空白处编辑路径" onClick={() => setEditing(true)}>
          {crumbs(path).map((c) => (
            <span key={c.path} className="crumb" title={c.path}
              onClick={(e) => { e.stopPropagation(); onGo(c.path); }}>
              {c.name}<i>▸</i>
            </span>
          ))}
        </div>
      )}
      <button title="刷新" onClick={() => onGo(path)}>⟳</button>
      {extra}
    </div>
  );
}

// ---- 选择模型（资源管理器惯例：单击选中，Ctrl 增减，Shift 范围） ----
const toggleSel = (sel: Set<string>, path: string): Set<string> => {
  const n = new Set(sel);
  if (n.has(path)) n.delete(path);
  else n.add(path);
  return n;
};
const rangeSel = (names: string[], join: (n: string) => string, a: number, b: number): Set<string> => {
  const [x, y] = a <= b ? [a, b] : [b, a];
  const n = new Set<string>();
  for (let i = x; i <= y; i++) n.add(join(names[i]));
  return n;
};
/**
 * SFTPDualPane：双栏文件管理器。
 * 挑选：单击选中 / Ctrl+Shift 多选 / 右键菜单动作（上传、下载、重命名、删除、新建…）。
 */
export default function SFTPDualPane({ sessionId, tabKey }: { sessionId: string; tabKey: string }) {
  const pane = getPane(tabKey);
  const [localEntries, setLocalEntries] = useState<LocalEntry[]>([]);
  const [remoteEntries, setRemoteEntries] = useState<RemoteEntry[]>([]);
  const [drives, setDrives] = useState<string[]>([]);
  const [dirs, setDirs] = useState<{ desktop: string; downloads: string; documents: string } | null>(null);
  const [home, setHome] = useState("");
  const [localHist, setLocalHist] = useState<Hist>(() => initHist(pane.localPath));
  const [remoteHist, setRemoteHist] = useState<Hist>(() => initHist(pane.remotePath));
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [dropHint, setDropHint] = useState<"left" | "right" | null>(null);
  const draggedLocal = useRef<{ path: string; isDir: boolean; name: string } | null>(null);

  // 选择
  const [selL, setSelL] = useState<Set<string>>(new Set());
  const [selR, setSelR] = useState<Set<string>>(new Set());
  const anchorL = useRef(-1);
  const anchorR = useRef(-1);

  // 右键菜单 / 行内编辑 / 对话框
  const [ctxMenu, setCtxMenu] = useState<{ x: number; y: number; side: "local" | "remote" } | null>(null);
  const [renaming, setRenaming] = useState<{ side: "local" | "remote"; path: string; draft: string } | null>(null);
  const [creating, setCreating] = useState<{ side: "local" | "remote"; draft: string } | null>(null);
  const [confirmDel, setConfirmDel] = useState<{ side: "local" | "remote"; paths: string[] } | null>(null);
  const [pickUpload, setPickUpload] = useState<{ paths: string[] } | null>(null);

  // 树宽（持久化）
  const [treeW, setTreeW] = useState<{ local: number; remote: number }>(() => {
    const read = (k: string, d: number) => {
      const v = Number(localStorage.getItem(k));
      return v >= 120 && v <= 400 ? v : d;
    };
    return { local: read("xlink.treeWidth.local", 168), remote: read("xlink.treeWidth.remote", 168) };
  });
  const makeResizer = (side: "local" | "remote") => (e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = treeW[side];
    const move = (ev: MouseEvent) => {
      const w = Math.min(400, Math.max(120, startW + (ev.clientX - startX)));
      setTreeW((prev) => ({ ...prev, [side]: w }));
    };
    const up = () => {
      window.removeEventListener("mousemove", move);
      window.removeEventListener("mouseup", up);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      setTreeW((prev) => {
        localStorage.setItem("xlink.treeWidth." + side, String(prev[side]));
        return prev;
      });
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", move);
    window.addEventListener("mouseup", up);
  };

  const localHistRef = useRef(localHist);
  const remoteHistRef = useRef(remoteHist);
  localHistRef.current = localHist;
  remoteHistRef.current = remoteHist;

  const localCur = histCurrent(localHist);
  const remoteCur = histCurrent(remoteHist);

  const loadLocal = useCallback(async (p: string, push = true) => {
    try {
      setLocalEntries(((await SftpService.ListLocalDir(p)) as never[]) || []);
      setLocalPath(tabKey, p);
      setError(null);
      setSelL(new Set());
      anchorL.current = -1;
      setLocalHist((h) => (push ? histGo(h, p) : { ...h, stack: h.stack.map((x, i) => (i === h.idx ? p : x)) }));
      SftpService.SaveLastDirs({ local: p, remote: histCurrent(remoteHistRef.current) } as never).catch(() => {});
    } catch (e) {
      setError(String(e));
    }
  }, [tabKey]);

  const loadRemote = useCallback(async (p: string, push = true) => {
    try {
      setRemoteEntries(((await SftpService.ListDir(sessionId, p)) as never[]) || []);
      setRemotePath(tabKey, p);
      setError(null);
      setSelR(new Set());
      anchorR.current = -1;
      setRemoteHist((h) => (push ? histGo(h, p) : { ...h, stack: h.stack.map((x, i) => (i === h.idx ? p : x)) }));
      SftpService.SaveLastDirs({ local: histCurrent(localHistRef.current), remote: p } as never).catch(() => {});
    } catch (e) {
      setError(String(e));
    }
  }, [sessionId, tabKey]);

  useEffect(() => {
    SftpService.ListDrives()
      .then((d: string[] | null) => setDrives(d || []))
      .catch(() => setDrives([]));
    SftpService.UserDirs()
      .then((d: { desktop: string; downloads: string; documents: string }) => setDirs(d))
      .catch(() => setDirs(null));
    SftpService.RemoteHome(sessionId)
      .then((h: string) => setHome(h))
      .catch(() => setHome(""));
    (async () => {
      let ld: { local?: string; remote?: string } = {};
      try { ld = (await SftpService.GetLastDirs()) as never; } catch { /* 忽略 */ }
      loadLocal(ld.local || pane.localPath);
      loadRemote(ld.remote || pane.remotePath);
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId]);

  const localJoin = (name: string) =>
    localCur.replace(/[\\/]+$/, "") + (localCur.includes("\\") ? "\\" : "/") + name;
  const remoteJoin = (name: string) =>
    remoteCur.replace(/\/+$/, "") + "/" + name;

  const refreshAfterOp = (side: "local" | "remote") => {
    if (side === "local") loadLocal(localCur, false);
    else loadRemote(remoteCur, false);
  };
  // ---- 传输动作 ----
  const queueChanged = () => window.dispatchEvent(new CustomEvent("sftp-queue-changed"));

  const uploadPaths = async (paths: string[], targetDir?: string) => {
    setBusy(true);
    setError(null);
    try {
      const dir = (targetDir ?? remoteCur).replace(/\/+$/, "");
      for (const p of paths) {
        const name = p.split(/[\\/]/).filter(Boolean).pop() || "file";
        await SftpService.Enqueue(1, sessionId, p, dir + "/" + name);
      }
      queueChanged();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const downloadPaths = async (paths: string[], targetDir?: string) => {
    setBusy(true);
    setError(null);
    try {
      const sep = localCur.includes("\\") ? "\\" : "/";
      const dir = (targetDir ?? localCur).replace(/[\\/]+$/, "");
      for (const rp of paths) {
        const name = rp.split("/").filter(Boolean).pop() || "file";
        await SftpService.Enqueue(2, sessionId, dir + sep + name, rp);
      }
      queueChanged();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const pickFiles = async () => {
    try {
      const paths = (await SftpService.PickUploadFiles()) as string[] | null;
      if (paths && paths.length) await uploadPaths(paths);
    } catch (e) {
      setError(String(e));
    }
  };

  const pickFolder = async () => {
    try {
      const dir = (await SftpService.PickUploadFolder()) as unknown as string;
      if (dir) await uploadPaths([dir]);
    } catch (e) {
      setError(String(e));
    }
  };

  // ---- 删除 ----
  const doDelete = async () => {
    if (!confirmDel) return;
    const { side, paths } = confirmDel;
    setError(null);
    try {
      for (const p of paths) {
        if (side === "local") await SftpService.LocalDelete(p);
        else await SftpService.RemoteDelete(sessionId, p);
      }
      const norm = (x: string) => x.replace(/[\\/]+$/, "").toLowerCase();
      const cur = norm(side === "local" ? localCur : remoteCur);
      const hit = paths.some((p) => cur === norm(p) || cur.startsWith(norm(p) + (side === "local" ? "\\" : "/")));
      if (hit) {
        if (side === "local") {
          const i = Math.max(localCur.lastIndexOf("\\"), localCur.lastIndexOf("/"));
          if (i > 2) await loadLocal(localCur.slice(0, i));
        } else {
          const i = remoteCur.lastIndexOf("/");
          if (i > 0) await loadRemote(remoteCur.slice(0, i));
        }
      } else refreshAfterOp(side);
    } catch (e) {
      setError(String(e));
    } finally {
      setConfirmDel(null);
    }
  };

  const commitRename = async () => {
    if (!renaming) return;
    const { side, path, draft } = renaming;
    const name = draft.trim();
    setRenaming(null);
    if (!name || name === path.split(/[\\/]/).pop()) return;
    try {
      if (side === "local") await SftpService.LocalRename(path, name);
      else await SftpService.RemoteRename(sessionId, path, name);
      refreshAfterOp(side);
    } catch (e) {
      setError(String(e));
    }
  };

  const commitCreate = async () => {
    if (!creating) return;
    const { side, draft } = creating;
    const name = draft.trim();
    setCreating(null);
    if (!name) return;
    try {
      if (side === "local") await SftpService.LocalMkdir(localCur, name);
      else await SftpService.RemoteMkdir(sessionId, remoteCur, name);
      refreshAfterOp(side);
    } catch (e) {
      setError(String(e));
    }
  };
  // ---- 选择与右键 ----
  const clickSelect = (side: "local" | "remote", e: React.MouseEvent, idx: number, path: string) => {
    const names = (side === "local" ? localEntries : remoteEntries).map((f) => f.name);
    const join = side === "local" ? localJoin : remoteJoin;
    const setSel = side === "local" ? setSelL : setSelR;
    const anchor = side === "local" ? anchorL : anchorR;
    if (e.ctrlKey || e.metaKey) {
      setSel((prev) => toggleSel(prev, path));
      anchor.current = idx;
    } else if (e.shiftKey && anchor.current >= 0) {
      setSel(rangeSel(names, join, anchor.current, idx));
    } else {
      setSel(new Set([path]));
      anchor.current = idx;
    }
  };

  const itemContext = (side: "local" | "remote", e: React.MouseEvent, idx: number, path: string) => {
    e.preventDefault();
    e.stopPropagation();
    const sel = side === "local" ? selL : selR;
    const setSel = side === "local" ? setSelL : setSelR;
    if (!sel.has(path)) {
      setSel(new Set([path]));
      (side === "local" ? anchorL : anchorR).current = idx;
    }
    setCtxMenu({ x: e.clientX, y: e.clientY, side });
  };

  const blankContext = (side: "local" | "remote", e: React.MouseEvent) => {
    e.preventDefault();
    (side === "local" ? setSelL : setSelR)(new Set());
    setCtxMenu({ x: e.clientX, y: e.clientY, side });
  };

  const selPaths = (side: "local" | "remote"): string[] => [...(side === "local" ? selL : selR)];

  const itemMenu = (side: "local" | "remote", paths: string[]): MenuItem[] => {
    const count = paths.length > 1 ? ` (${paths.length})` : "";
    if (side === "local") {
      const items: MenuItem[] = [
        { label: "上传" + count, onClick: () => uploadPaths(paths) },
        { label: "上传到…", onClick: () => setPickUpload({ paths }) },
      ];
      if (paths.length === 1) {
        items.push({ label: "在资源管理器中显示", onClick: () => SftpService.LocalReveal(paths[0]).catch((e) => setError(String(e))) });
      }
      return items;
    }
    const items: MenuItem[] = [
      { label: "下载" + count, onClick: () => downloadPaths(paths) },
      { label: "下载到…", onClick: async () => {
          try {
            const dir = (await SftpService.PickDownloadFolder()) as unknown as string;
            if (dir) await downloadPaths(paths, dir);
          } catch (e) { setError(String(e)); }
        } },
    ];
    if (paths.length === 1) {
      items.push({ label: "重命名…", onClick: () => setRenaming({ side, path: paths[0], draft: paths[0].split(/[\\/]/).pop() || "" }) });
    }
    items.push({ label: "删除" + count, danger: true, onClick: () => setConfirmDel({ side, paths }) });
    if (paths.length === 1) {
      items.push({ label: "复制路径", onClick: () => navigator.clipboard.writeText(paths[0]).catch(() => {}) });
    }
    return items;
  };

  const blankMenu = (side: "local" | "remote"): MenuItem[] => {
    const items: MenuItem[] = [
      { label: "刷新", onClick: () => refreshAfterOp(side) },
      { label: "上传文件…", onClick: pickFiles },
      { label: "上传文件夹…", onClick: pickFolder },
      { label: "新建目录…", onClick: () => setCreating({ side, draft: "" }) },
    ];
    if (side === "local") {
      items.push({ label: "在资源管理器中打开", onClick: () => SftpService.LocalReveal(localCur).catch((e) => setError(String(e))) });
    }
    return items;
  };

  // 键盘：Backspace 上级 / Alt+←→ 历史 / Delete 删除 / F2 重命名 / Esc 清选
  const keyNav = (side: "local" | "remote") => (e: React.KeyboardEvent) => {
    const hist = side === "local" ? localHist : remoteHist;
    const setHist = side === "local" ? setLocalHist : setRemoteHist;
    const load = side === "local" ? loadLocal : loadRemote;
    const cur = histCurrent(hist);
    if (e.key === "Backspace" && !renaming && !creating) {
      e.preventDefault();
      const norm = cur.replace(/[\\/]+$/, "");
      const i = Math.max(norm.lastIndexOf("\\"), norm.lastIndexOf("/"));
      if (i < 0) return;
      if (i <= 2 && norm[1] === ":") load(norm.slice(0, 3));
      else load(norm.slice(0, i) || "/");
      return;
    }
    if (e.altKey && e.key === "ArrowLeft") { const h = histMove(hist, -1); setHist(h); load(histCurrent(h), false); }
    if (e.altKey && e.key === "ArrowRight") { const h = histMove(hist, 1); setHist(h); load(histCurrent(h), false); }
    if (e.key === "Delete" && selPaths(side).length > 0 && !renaming && !creating) {
      setConfirmDel({ side, paths: selPaths(side) });
    }
    if (e.key === "F2" && selPaths(side).length === 1) {
      const p0 = selPaths(side)[0];
      setRenaming({ side, path: p0, draft: p0.split(/[\\/]/).pop() || "" });
    }
    if (e.key === "Escape") { (side === "local" ? setSelL : setSelR)(new Set()); }
  };

  // 树根：快速访问 + 盘符 / 远端主目录与根
  const localRoots: { label: string; path: string }[] = [
    ...(dirs
      ? [
          { label: "桌面", path: dirs.desktop },
          { label: "下载", path: dirs.downloads },
          { label: "文档", path: dirs.documents },
        ].filter((x) => x.path)
      : []),
    ...drives.map((d) => ({ label: d, path: d })),
  ];
  const remoteRoots: { label: string; path: string }[] =
    home && home !== "/"
      ? [{ label: "主目录 ~", path: home }, { label: "/", path: "/" }]
      : [{ label: "/", path: "/" }];

const driveSelect = (
    <select
      className="drive-select"
      title="切换盘符"
      value={drives.includes(localCur.slice(0, 3)) ? localCur.slice(0, 3) : ""}
      onChange={(e) => loadLocal(e.target.value)}
    >
      {!drives.includes(localCur.slice(0, 3)) && <option value="">盘符</option>}
      {drives.map((d) => (
        <option key={d} value={d}>{d}</option>
      ))}
    </select>
  );

  // 列表项渲染（含行内重命名态）
  const renderRow = (side: "local" | "remote", f: RemoteEntry, idx: number) => {
    const join = side === "local" ? localJoin : remoteJoin;
    const path = join(f.name);
    const sel = side === "local" ? selL : selR;
    const isRenaming = renaming?.side === side && renaming.path === path;
    return (
      <li
        key={path}
        className={sel.has(path) ? "selected" : ""}
        draggable={!isRenaming}
        onDragStart={(e) => {
          if (side === "local") {
            draggedLocal.current = { path, isDir: f.isDir, name: f.name };
            e.dataTransfer.setData("text/plain", f.name);
            e.dataTransfer.setData("application/x-local-dir", f.isDir ? "1" : "0");
          } else {
            e.dataTransfer.setData("application/x-remote", path);
            e.dataTransfer.setData("application/x-remote-dir", f.isDir ? "1" : "0");
          }
        }}
        onDragEnd={() => { draggedLocal.current = null; }}
        onClick={(e) => clickSelect(side, e, idx, path)}
        onDoubleClick={() => {
          if (f.isDir) {
            if (side === "local") loadLocal(path);
            else loadRemote(path);
          }
        }}
        onContextMenu={(e) => itemContext(side, e, idx, path)}
      >
        {isRenaming ? (
          <input
            className="inline-edit"
            value={renaming.draft}
            autoFocus
            onClick={(e) => e.stopPropagation()}
            onChange={(e) => setRenaming({ ...renaming, draft: e.target.value })}
            onBlur={() => setRenaming(null)}
            onKeyDown={(e) => {
              if (e.key === "Enter") commitRename();
              if (e.key === "Escape") setRenaming(null);
            }}
          />
        ) : (
          <>{f.isDir ? "📁" : "📄"} {f.name}</>
        )}
      </li>
    );
  };

  return (
    <div className="dualpane">
      {error && <p className="error">{error}</p>}
      <div className="dual-cols">
        <div
          className={"pane" + (dropHint === "left" ? " drop" : "")}
          tabIndex={0}
          onKeyDown={keyNav("local")}
          onContextMenu={(e) => blankContext("local", e)}
          onDragOver={(e) => { e.preventDefault(); setDropHint("left"); }}
          onDragLeave={() => setDropHint(null)}
          onDrop={(e) => {
            e.preventDefault();
            setDropHint(null);
            const rp = e.dataTransfer.getData("application/x-remote");
            if (!rp) return;
            downloadPaths([rp]);
          }}
        >
          <NavBar
            path={localCur}
            canBack={localHist.idx > 0}
            canFwd={localHist.idx < localHist.stack.length - 1}
            onBack={() => { const h = histMove(localHist, -1); setLocalHist(h); loadLocal(histCurrent(h), false); }}
            onFwd={() => { const h = histMove(localHist, 1); setLocalHist(h); loadLocal(histCurrent(h), false); }}
            onGo={(p) => loadLocal(p)}
            extra={driveSelect}
          />
          <div className="pane-body">
          <DirTree sessionId={sessionId} side="local" roots={localRoots} current={localCur} onNavigate={(p) => loadLocal(p)} width={treeW.local} />
          <div className="tree-resizer" title="拖动调整目录树宽度" onMouseDown={makeResizer("local")} />
          <ul onClick={(e) => { if ((e.target as HTMLElement).tagName === "UL") { setSelL(new Set()); anchorL.current = -1; } }}>
            {creating?.side === "local" && (
              <li className="creating">
                <input
                  className="inline-edit"
                  value={creating.draft}
                  autoFocus
                  onChange={(e) => setCreating({ ...creating, draft: e.target.value })}
                  onBlur={() => setCreating(null)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") commitCreate();
                    if (e.key === "Escape") setCreating(null);
                  }}
                />
              </li>
            )}
            {localEntries.map((f, i) => renderRow("local", f, i))}
          </ul>
          </div>
        </div>        <div
          className={"pane" + (dropHint === "right" ? " drop" : "")}
          tabIndex={0}
          onKeyDown={keyNav("remote")}
          onContextMenu={(e) => blankContext("remote", e)}
          onDragOver={(e) => { e.preventDefault(); setDropHint("right"); }}
          onDragLeave={() => setDropHint(null)}
          onDrop={(e) => {
            e.preventDefault();
            setDropHint(null);
            const d = draggedLocal.current;
            if (d) {
              draggedLocal.current = null;
              uploadPaths([d.path]);
              return;
            }
            const name = e.dataTransfer.getData("text/plain");
            if (name) uploadPaths([localJoin(name)]);
          }}
        >
          <NavBar
            path={remoteCur}
            canBack={remoteHist.idx > 0}
            canFwd={remoteHist.idx < remoteHist.stack.length - 1}
            onBack={() => { const h = histMove(remoteHist, -1); setRemoteHist(h); loadRemote(histCurrent(h), false); }}
            onFwd={() => { const h = histMove(remoteHist, 1); setRemoteHist(h); loadRemote(histCurrent(h), false); }}
            onGo={(p) => loadRemote(p)}
          />
          <div className="pane-body">
          <DirTree sessionId={sessionId} side="remote" roots={remoteRoots} current={remoteCur} onNavigate={(p) => loadRemote(p)} width={treeW.remote} />
          <div className="tree-resizer" title="拖动调整目录树宽度" onMouseDown={makeResizer("remote")} />
          <ul onClick={(e) => { if ((e.target as HTMLElement).tagName === "UL") { setSelR(new Set()); anchorR.current = -1; } }}>
            {creating?.side === "remote" && (
              <li className="creating">
                <input
                  className="inline-edit"
                  value={creating.draft}
                  autoFocus
                  onChange={(e) => setCreating({ ...creating, draft: e.target.value })}
                  onBlur={() => setCreating(null)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") commitCreate();
                    if (e.key === "Escape") setCreating(null);
                  }}
                />
              </li>
            )}
            {remoteEntries.map((f, i) => renderRow("remote", f, i))}
          </ul>
          </div>
        </div>
      </div>

      {ctxMenu && (
        <ContextMenu
          x={ctxMenu.x}
          y={ctxMenu.y}
          items={ctxMenu.side === "local"
            ? (selPaths("local").length ? itemMenu("local", selPaths("local")) : blankMenu("local"))
            : (selPaths("remote").length ? itemMenu("remote", selPaths("remote")) : blankMenu("remote"))}
          onClose={() => setCtxMenu(null)}
        />
      )}

      {confirmDel && (
        <ConfirmDialog
          title="删除确认"
          message={"将永久删除 " + confirmDel.paths.length + " 项（含子内容），此操作不可恢复："}
          items={confirmDel.paths.length <= 8 ? confirmDel.paths : undefined}
          confirmText="永久删除"
          onConfirm={doDelete}
          onCancel={() => setConfirmDel(null)}
        />
      )}

      {pickUpload && (
        <RemotePickDialog
          sessionId={sessionId}
          initialPath={remoteCur}
          onPick={async (dir) => {
            const paths = pickUpload.paths;
            setPickUpload(null);
            await uploadPaths(paths, dir);
            loadRemote(remoteCur, false);
          }}
          onCancel={() => setPickUpload(null)}
        />
      )}
      {busy && <p className="hint-inline">入队中…</p>}
    </div>
  );
}