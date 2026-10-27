import { useState } from "react";
import { useTabs, connectTab } from "../stores/tabs";
import { toggleSftp, useSftp } from "../stores/sftp";
import TerminalView from "./TerminalView";
import SFTPDualPane from "./SFTPDualPane";
import TransferQueue from "./TransferQueue";
import SFTPSettings from "./SFTPSettings";

/** 会话区：每标签可切换 终端/SFTP 视图；底部传输队列；SFTP 设置入口。 */
export default function SessionArea() {
  const { tabs, activeKey } = useTabs();
  const { openFor } = useSftp();
  const [showSettings, setShowSettings] = useState(false);

  if (tabs.length === 0) {
    return (
      <main className="container">
        <h1>XLink</h1>
        <p className="subtitle">从左侧选择主机，或新建主机开始连接</p>
      </main>
    );
  }

  return (
    <div className="session-area">
      {tabs.map((t) => {
        const active = t.key === activeKey;
        const sftpOpen = openFor === t.key;
        return (
          <div key={t.key} style={{ display: active ? "flex" : "none" }} className="session-pane">
            <div className="pane-bar">
              <button onClick={() => toggleSftp(t.key)} disabled={t.status !== "connected"}>
                {sftpOpen ? "终端" : "SFTP"}
              </button>
              <button onClick={() => setShowSettings(true)}>传输设置</button>
            </div>
            {t.status === "connected" && t.session ? (
              sftpOpen ? (
                <SFTPDualPane sessionId={t.session.sessionId} tabKey={t.key} />
              ) : (
                <TerminalView session={t.session} onExit={() => connectTab(t.key)} />
              )
            ) : t.status === "connecting" ? (
              <div className="pane-status">连接中…</div>
            ) : t.status === "disconnected" ? (
              <div className="pane-status">保险库已锁定，解锁后自动恢复</div>
            ) : (
              <div className="pane-status">
                <p>{t.error || "未连接"}</p>
                <button onClick={() => connectTab(t.key)}>重连</button>
              </div>
            )}
          </div>
        );
      })}
      <TransferQueue />
      {showSettings && <SFTPSettings onClose={() => setShowSettings(false)} />}
    </div>
  );
}
