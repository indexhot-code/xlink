import { useState } from "react";
import { useTabs, activateTab, closeTab, cloneTab, type Tab } from "../stores/tabs";
import ContextMenu from "./ContextMenu";

/** 多标签条：标题、激活态、关闭；右键 → 克隆连接/关闭。 */
export default function TabBar() {
  const { tabs, activeKey } = useTabs();
  const [menu, setMenu] = useState<{ x: number; y: number; tab: Tab } | null>(null);
  if (tabs.length === 0) return null;

  return (
    <div className="tabbar">
      {tabs.map((t) => (
        <div
          key={t.key}
          className={"tab" + (t.key === activeKey ? " active" : "")}
          onClick={() => activateTab(t.key)}
          onContextMenu={(e) => {
            e.preventDefault();
            setMenu({ x: e.clientX, y: e.clientY, tab: t });
          }}
        >
          <span className={"dot " + t.status} />
          <span className="tab-title">
            {t.host.user}@{t.host.host}
          </span>
          <button
            className="tab-close"
            onClick={(e) => {
              e.stopPropagation();
              closeTab(t.key);
            }}
          >
            ×
          </button>
        </div>
      ))}
      {menu && (
        <ContextMenu
          x={menu.x}
          y={menu.y}
          onClose={() => setMenu(null)}
          items={[
            { label: "克隆连接", onClick: () => cloneTab(menu.tab.key) },
            { label: "关闭标签", danger: true, onClick: () => closeTab(menu.tab.key) },
          ]}
        />
      )}
    </div>
  );
}
