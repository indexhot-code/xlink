import { useEffect, useRef, useState } from "react";
import { PtySocket, type PtyControl, type PtySessionInfo } from "../lib/ws";
import { createTerminal } from "../lib/terminal";
import { getSettings } from "../stores/settings";
import ContextMenu, { type MenuItem } from "./ContextMenu";

interface Props {
  session: PtySessionInfo | null;
  onExit?: (code: number) => void;
}

/**
 * TerminalView 挂载 xterm.js 并通过回环 WebSocket 与 PTY 会话交换字节流。
 * 字节流绝不经过 Wails Binding/Events（架构规则 1、2）。
 */
export default function TerminalView({ session, onExit }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const ptyRef = useRef<PtySocket | null>(null);
  const handleRef = useRef<ReturnType<typeof createTerminal> | null>(null);
  // 智能右键：有选区→复制；无选区→粘贴；Shift+右键→完整菜单。
  const [menu, setMenu] = useState<{ x: number; y: number } | null>(null);
  const menuItems = (): MenuItem[] => [
    { label: "复制", onClick: () => handleRef.current && navigator.clipboard.writeText(handleRef.current.term.getSelection()).catch(() => {}) },
    { label: "粘贴", onClick: doPaste },
    { label: "全选", onClick: () => handleRef.current?.term.selectAll() },
    { label: "清屏", onClick: () => handleRef.current?.term.clear(), danger: true },
  ];

  // doPaste 读剪贴板并写入 PTY stdin（与 Ctrl+V 行为一致，不加多行防护）。
  function doPaste() {
    navigator.clipboard
      .readText()
      .then((text) => ptyRef.current?.sendInput(text))
      .catch(() => {});
  }

  useEffect(() => {
    if (!session || !containerRef.current) return;

    const container = containerRef.current;
    const s = getSettings();
    const handle = createTerminal(container, {
      fontSize: s.terminal.fontSize,
      scrollback: s.terminal.scrollback,
      cursorBlink: s.terminal.cursorBlink,
    });
    const pty = new PtySocket(session);
    ptyRef.current = pty;
    handleRef.current = handle;

    // PTY 输出 → 终端
    pty.onBinary = (data) => handle.term.write(data);

    // 用户键入 → PTY stdin（二进制帧）
    const dataSub = handle.term.onData((d) => pty.sendInput(d));

    // 终端尺寸变化 → resize 控制帧（文本帧）
    const resizeSub = handle.term.onResize(({ cols, rows }) => {
      pty.sendControl({ type: "resize", cols, rows });
    });

    // 容器尺寸变化 → 重新 fit（触发 onResize → resize 控制帧）
    const ro = new ResizeObserver(() => handle.fit.fit());
    ro.observe(container);

    // 智能右键：接管 xterm 区域的 contextmenu，屏蔽 WebView2 默认菜单。
    const term = handle.term;
    const onContextMenu = (e: MouseEvent) => {
      e.preventDefault();
      if (e.shiftKey) {
        setMenu({ x: e.clientX, y: e.clientY });
      } else if (term.hasSelection()) {
        navigator.clipboard.writeText(term.getSelection()).catch(() => {});
      } else {
        doPaste();
      }
    };
    container.addEventListener("contextmenu", onContextMenu);

    // 远端会话退出
    pty.onControl = (msg: PtyControl) => {
      if (msg.type === "exit") onExit?.(msg.code);
    };
    pty.onClose = () => onExit?.(-1);

    return () => {
      ro.disconnect();
      container.removeEventListener("contextmenu", onContextMenu);
      resizeSub.dispose();
      dataSub.dispose();
      pty.close();
      handle.dispose();
      ptyRef.current = null;
      handleRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session?.sessionId]);

  return (
    <div ref={containerRef} className="terminal-container">
      {menu && <ContextMenu x={menu.x} y={menu.y} items={menuItems()} onClose={() => setMenu(null)} />}
    </div>
  );
}
