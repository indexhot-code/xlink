import { useEffect, useState } from "react";
import * as ConnectionService from "../bindings/xlink/backend/connection/service.js";
import TabBar from "./components/TabBar";
import HostList from "./components/HostList";
import SessionArea from "./components/SessionArea";
import StatusBar from "./components/StatusBar";
import SettingsDialog from "./components/SettingsDialog";
import VaultGate from "./components/VaultGate";
import { restoreSnapshot } from "./stores/tabs";
import { useVault } from "./stores/vault";
import type { Host } from "../bindings/xlink/backend/connection/models.js";

/** 应用布局：侧栏（主机）+ 标签条 + 会话区 + 状态栏。启动时按快照恢复标签。 */
export default function App() {
  return (
    <VaultGate>
      <Workspace />
    </VaultGate>
  );
}

function Workspace() {
  const vault = useVault();
  const [hostsLoaded, setHostsLoaded] = useState(false);
  const [showSettings, setShowSettings] = useState(false);

  useEffect(() => {
    if (!vault.unlocked || hostsLoaded) return;
    (async () => {
      try {
        const hosts = ((await ConnectionService.ListHosts()) as never[]) || [];
        await restoreSnapshot(hosts as Host[], vault.unlocked);
      } finally {
        setHostsLoaded(true);
      }
    })();
  }, [vault.unlocked, hostsLoaded]);

  return (
    <div className="app-shell">
      <div className="workspace">
        <HostList />
        <div className="main-col">
          <TabBar />
          <SessionArea />
        </div>
      </div>
      <StatusBar onOpenSettings={() => setShowSettings(true)} />
      {showSettings && <SettingsDialog onClose={() => setShowSettings(false)} />}
    </div>
  );
}
