/**
 * xterm.js 5.x 封装：WebGL 渲染 + 自适应尺寸。
 * 输入/输出的字节流转运由调用方（TerminalView）与 PtySocket 完成。
 */
import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { WebglAddon } from "@xterm/addon-webgl";
import "@xterm/xterm/css/xterm.css";

export interface TerminalOptions {
  fontSize?: number;
  scrollback?: number;
  cursorBlink?: boolean;
}

export interface TerminalHandle {
  term: Terminal;
  fit: FitAddon;
  dispose(): void;
}

export function createTerminal(container: HTMLElement, opts: TerminalOptions = {}): TerminalHandle {
  const term = new Terminal({
    fontSize: opts.fontSize ?? 13,
    fontFamily: '"Cascadia Mono", Consolas, "Courier New", monospace',
    cursorBlink: opts.cursorBlink ?? true,
    scrollback: opts.scrollback ?? 5000,
    theme: {
      background: "rgba(0,0,0,0)", // 透出窗口材质
      foreground: "#f4f6fb",
    },
  });

  const fit = new FitAddon();
  term.loadAddon(fit);
  term.open(container);
  fit.fit();

  // WebGL 渲染失败（无 GPU/上下文丢失）时自动回退默认渲染器。
  try {
    term.loadAddon(new WebglAddon());
  } catch {
    console.warn("WebGL addon unavailable, falling back to canvas renderer");
  }

  return {
    term,
    fit,
    dispose: () => term.dispose(),
  };
}
