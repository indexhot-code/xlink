import { useEffect, useState } from "react";
import * as ConnectionService from "../../bindings/xlink/backend/connection/service.js";
import type { Host } from "../../bindings/xlink/backend/connection/models.js";
import { openHost } from "../stores/tabs";
import HostForm from "./HostForm";

/** 主机列表侧栏：点击连接，编辑/删除。 */
export default function HostList() {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [editing, setEditing] = useState<Host | "new" | null>(null);

  const reload = async () => {
    try {
      setHosts(((await ConnectionService.ListHosts()) as never[]) || []);
    } catch (e) {
      console.error(e);
    }
  };

  useEffect(() => {
    reload();
  }, []);

  return (
    <aside className="hostlist">
      <div className="hostlist-head">
        <span>主机</span>
        <button onClick={() => setEditing("new")}>+ 新建</button>
      </div>
      <ul>
        {hosts.map((h) => (
          <li key={String(h.id)}>
            <button className="host-item" onClick={() => openHost(h)} title={h.host}>
              <span className="host-name">{h.name}</span>
              <span className="host-addr">
                {h.user}@{h.host}
              </span>
            </button>
            <button className="host-edit" title="编辑" onClick={() => setEditing(h)}>
              ⚙
            </button>
          </li>
        ))}
        {hosts.length === 0 && <li className="empty">暂无主机</li>}
      </ul>
      {editing && (
        <HostForm
          host={editing === "new" ? null : editing}
          onDone={() => {
            setEditing(null);
            reload();
          }}
          onCancel={() => setEditing(null)}
        />
      )}
    </aside>
  );
}
