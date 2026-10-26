/**
 * 回环 WebSocket 客户端（架构规则 1）：
 *   - 连接 ws://127.0.0.1:<port>/pty/<sessionId>?token=<t>；
 *   - 二进制帧 = PTY 原始字节（双向）；
 *   - 文本帧 = JSON 控制消息（resize/exit/state）。
 * Token 仅保存在 SessionInfo 对象（前端内存），不落盘、不进 localStorage。
 */

export interface PtyControlResize {
  type: "resize";
  cols: number;
  rows: number;
}

export interface PtyControlExit {
  type: "exit";
  code: number;
}

export interface PtyControlState {
  type: "state";
  connected: boolean;
}

export type PtyControl = PtyControlResize | PtyControlExit | PtyControlState;

export interface PtySessionInfo {
  sessionId: string;
  wsPort: number;
  token: string;
}

export class PtySocket {
  private ws: WebSocket;
  private encoder = new TextEncoder();

  onBinary?: (data: Uint8Array) => void;
  onControl?: (msg: PtyControl) => void;
  onClose?: (err: Event | null) => void;

  constructor(info: PtySessionInfo) {
    const url = `ws://127.0.0.1:${info.wsPort}/pty/${info.sessionId}?token=${info.token}`;
    this.ws = new WebSocket(url);
    this.ws.binaryType = "arraybuffer";
    this.ws.onmessage = (ev) => {
      if (typeof ev.data === "string") {
        try {
          this.onControl?.(JSON.parse(ev.data) as PtyControl);
        } catch {
          // 忽略坏帧
        }
        return;
      }
      this.onBinary?.(new Uint8Array(ev.data as ArrayBuffer));
    };
    this.ws.onclose = (ev) => this.onClose?.(ev);
    this.ws.onerror = () => this.onClose?.(null);
  }

  /** 发送用户输入（二进制帧 = PTY 原始字节）。 */
  sendInput(data: string): void {
    if (this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(this.encoder.encode(data));
  }

  /** 发送控制消息（文本帧 = JSON）。 */
  sendControl(msg: PtyControl): void {
    if (this.ws.readyState !== WebSocket.OPEN) return;
    this.ws.send(JSON.stringify(msg));
  }

  close(): void {
    if (this.ws.readyState === WebSocket.OPEN || this.ws.readyState === WebSocket.CONNECTING) {
      this.ws.close();
    }
  }
}
