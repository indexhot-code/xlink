import { useEffect, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";
import * as AppSettingsService from "../../bindings/xlink/backend/appsettings/service.js";
import { useTabs } from "../stores/tabs";
import { useVault } from "../stores/vault";

type Transfer = { state: string };

/** StatusBar：底部状态栏 — 版本、会话/传输/保险库状态。 */
export default function StatusBar({ onOpenSettings }: { onOpenSettings: () => void }) {
  const { tabs } = useTabs();
  const vault = useVault();
  const [version, setVersion] = useState("");
  const [activeTransfers, setActiveTransfers] = useState(0);

  useEffect(() => {
    AppSettingsService.GetAppInfo()
      .then((v: { version: string }) => setVersion(v.version))
      .catch(() => setVersion(""));
  }, []);

  useEffect(() => {
    let alive = true;
    const poll = async () => {
      try {
        const l = ((await SftpService.ListTransfers()) as never[]) || [];
        if (alive) {
          setActiveTransfers(
            (l as Transfer[]).filter((t) => t.state === "running" || t.state === "pending").length,
          );
        }
      } catch {
        /* 忽略 */
      }
    };
    poll();
    const timer = setInterval(poll, 1000);
    const onChange = () => poll();
    window.addEventListener("sftp-queue-changed", onChange);
    return () => {
      alive = false;
      clearInterval(timer);
      window.removeEventListener("sftp-queue-changed", onChange);
    };
  }, []);

  const connected = tabs.filter((t) => t.status === "connected").length;

  return (
    <footer className="statusbar">
      <span className="sb-item">{version && `XLink v${version}`}</span>
      <span className="sb-sep" />
      <span className="sb-item">
        会话 <b>{connected}</b>
      </span>
      <span className="sb-sep" />
      <span className="sb-item">
        传输 <b>{activeTransfers}</b> 进行中
      </span>
      <span className="sb-flex" />
      <span className={"sb-item sb-vault " + (vault.unlocked ? "ok" : "warn")}>
        {vault.unlocked ? "保险库已解锁" : "保险库已锁定"}
      </span>
      <button className="sb-settings" title="设置" onClick={onOpenSettings}>
        设置
      </button>
    </footer>
  );
}
