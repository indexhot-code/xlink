import { useCallback, useEffect, useRef, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";

export interface TreeRoot {
  label: string;
  path: string;
}

interface DirNode {
  name: string;
  path: string;
}

/**
 * DirTree：资源管理器式树形目录导航。
 * - 根节点（快速访问/盘符/主目录）+ 懒加载展开子目录
 * - 点击节点名导航；点击箭头展开/收起
 * - currentPath 变化时自动逐级展开到当前目录并高亮
 */
export default function DirTree({ sessionId, side, roots, current, onNavigate, width }: {
  sessionId?: string;
  side: "local" | "remote";
  roots: TreeRoot[];
  current: string;
  width?: number;
  onNavigate: (p: string) => void;
}) {
  const [expanded, setExpanded] = useState<Set<string>>(new Set());
  const [childrenMap, setChildrenMap] = useState<Record<string, DirNode[]>>({});
  const [failed, setFailed] = useState<Set<string>>(new Set());
  const childrenRef = useRef(childrenMap);
  childrenRef.current = childrenMap;

  const join = (base: string, name: string) =>
    base.endsWith("\\") || base.endsWith("/")
      ? base + name
      : base + (base.includes("\\") ? "\\" : "/") + name;

  const loadChildren = useCallback(async (p: string): Promise<DirNode[]> => {
    const entries =
      side === "local"
        ? ((await SftpService.ListLocalDir(p)) as never[]) || []
        : ((await SftpService.ListDir(sessionId || "", p)) as never[]) || [];
    return (entries as { name: string; isDir: boolean }[])
      .filter((e) => e.isDir && e.name !== "." && e.name !== "..")
      .map((e) => ({ name: e.name, path: join(p, e.name) }))
      .sort((a, b) => a.name.localeCompare(b.name));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [side, sessionId]);

  // 展开某节点（确保子目录已加载）
  const expandNode = useCallback(async (p: string) => {
    setExpanded((prev) => new Set(prev).add(p));
    if (!childrenRef.current[p]) {
      try {
        const ch = await loadChildren(p);
        setChildrenMap((prev) => ({ ...prev, [p]: ch }));
        return ch;
      } catch {
        setFailed((prev) => new Set(prev).add(p));
        return [];
      }
    }
    return childrenRef.current[p];
  }, [loadChildren]);

  const toggle = (p: string) => {
    if (expanded.has(p)) {
      setExpanded((prev) => {
        const n = new Set(prev);
        n.delete(p);
        return n;
      });
    } else {
      expandNode(p);
    }
  };

  // 当前路径变化：逐级展开祖先链（资源管理器行为）
  useEffect(() => {
    let alive = true;
    (async () => {
      const segs = crumbsOf(current);
      for (const seg of segs) {
        if (!alive) return;
        await expandNode(seg.path);
      }
    })();
    return () => { alive = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [current]);

  const isActive = (p: string) =>
    current.replace(/[\\/]+$/, "").toLowerCase() === p.replace(/[\\/]+$/, "").toLowerCase();

  const renderNodes = (nodes: DirNode[], depth: number): React.ReactNode =>
    nodes.map(({ name, path }) => {
      const isOpen = expanded.has(path);
      const kids = childrenMap[path];
      return (
        <div key={path}>
          <div
            className={"tree-item" + (isActive(path) ? " active" : "")}
            style={{ paddingLeft: depth * 13 + 6 }}
            onClick={() => onNavigate(path)}
          >
            <span
              className={"tree-caret" + (failed.has(path) ? " failed" : "")}
              onClick={(e) => { e.stopPropagation(); toggle(path); }}
            >
              {failed.has(path) ? "✕" : isOpen ? "▾" : "▸"}
            </span>
            <span className="tree-label">{isOpen ? "📂" : "📁"} {name}</span>
          </div>
          {isOpen && kids && kids.length > 0 && renderNodes(kids, depth + 1)}
        </div>
      );
    });

  return (
    <div className="dir-tree" style={width ? { width, flexShrink: 0 } : undefined}>
      {roots.map((r) => {
        const isOpen = expanded.has(r.path);
        const kids = childrenMap[r.path];
        return (
          <div key={r.label + r.path}>
            <div
              className={"tree-item root" + (isActive(r.path) ? " active" : "")}
              onClick={() => onNavigate(r.path)}
            >
              <span
                className={"tree-caret" + (failed.has(r.path) ? " failed" : "")}
                onClick={(e) => { e.stopPropagation(); toggle(r.path); }}
              >
                {failed.has(r.path) ? "✕" : isOpen ? "▾" : "▸"}
              </span>
              <span className="tree-label">{isOpen ? "📂" : "📁"} {r.label}</span>
            </div>
            {isOpen && kids && kids.length > 0 && renderNodes(kids, 1)}
          </div>
        );
      })}
    </div>
  );
}

// 面包屑式路径分段（与 SFTPDualPane 内逻辑一致）
function crumbsOf(p: string): { name: string; path: string }[] {
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