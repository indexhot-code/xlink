import { Window } from "@wailsio/runtime";
import { useVault, lockVault } from "../stores/vault";
import { lockTabs } from "../stores/tabs";

/** TitleBar：无边框窗口的自定义标题栏（深色不透明，拖拽区 + 窗口控制按钮 + 锁定入口）。 */
export default function TitleBar() {
  const vault = useVault();

  return (
    <div className="titlebar" onDoubleClick={() => Window.ToggleMaximise()}>
      <span className="tb-title">XLink</span>
      <span className="tb-flex" />
      {vault.unlocked && (
        <button
          className="tb-btn tb-lock"
          title="锁定保险库"
          onClick={() => lockVault().then(() => lockTabs())}
        >
          锁定
        </button>
      )}
      <button className="tb-btn" title="最小化" onClick={() => Window.Minimise()}>
        &#x2500;
      </button>
      <button className="tb-btn" title="最大化 / 还原" onClick={() => Window.ToggleMaximise()}>
        &#x25A1;
      </button>
      <button className="tb-btn tb-close" title="关闭" onClick={() => Window.Close()}>
        &#x2715;
      </button>
    </div>
  );
}
