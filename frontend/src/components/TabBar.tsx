import { useTabs, activateTab, closeTab } from "../stores/tabs";

/** 多标签条：标题、激活态、关闭。 */
export default function TabBar() {
  const { tabs, activeKey } = useTabs();
  if (tabs.length === 0) return null;

  return (
    <div className="tabbar">
      {tabs.map((t) => (
        <div
          key={t.key}
          className={"tab" + (t.key === activeKey ? " active" : "")}
          onClick={() => activateTab(t.key)}
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
    </div>
  );
}
