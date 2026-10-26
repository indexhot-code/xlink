import { useEffect, useRef } from "react";
import { PtySocket, type PtyControl, type PtySessionInfo } from "../lib/ws";
import { createTerminal } from "../lib/terminal";
import { getSettings } from "../stores/settings";

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

    // 远端会话退出
    pty.onControl = (msg: PtyControl) => {
      if (msg.type === "exit") onExit?.(msg.code);
    };
    pty.onClose = () => onExit?.(-1);

    return () => {
      ro.disconnect();
      resizeSub.dispose();
      dataSub.dispose();
      pty.close();
      handle.dispose();
      ptyRef.current = null;
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [session?.sessionId]);

  return <div ref={containerRef} className="terminal-container" />;
}
