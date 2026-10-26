import { useEffect, useState } from "react";
import * as AppSettingsService from "../../bindings/xlink/backend/appsettings/service.js";
import { loadSettings, saveSettings, useSettings, type AppSettings } from "../stores/settings";

/** SettingsDialog：应用设置（终端行为 / 窗口行为）。终端项对新建终端生效。 */
export default function SettingsDialog({ onClose }: { onClose: () => void }) {
  const { settings, loaded } = useSettings();
  const [draft, setDraft] = useState<AppSettings | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [version, setVersion] = useState("");

  useEffect(() => {
    loadSettings();
  }, []);

  useEffect(() => {
    if (loaded && !draft) setDraft({ ...settings });
  }, [loaded, settings, draft]);

  useEffect(() => {
    AppSettingsService.GetAppInfo()
      .then((v: { version: string }) => setVersion(v.version))
      .catch(() => setVersion(""));
  }, []);

  if (!draft) return null;

  const setTerm = (patch: Partial<AppSettings["terminal"]>) =>
    setDraft({ ...draft, terminal: { ...draft.terminal, ...patch } });

  const doSave = async () => {
    setBusy(true);
    setError(null);
    try {
      await saveSettings(draft);
      onClose();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dialog-backdrop" onClick={onClose}>
      <div className="dialog" onClick={(e) => e.stopPropagation()}>
        <h2>设置</h2>

        <div className="settings-group">
          <div className="settings-title">终端（对新建终端生效）</div>
          <label className="settings-row">
            <span>字号</span>
            <select
              value={draft.terminal.fontSize}
              onChange={(e) => setTerm({ fontSize: Number(e.target.value) })}
            >
              {[10, 11, 12, 13, 14, 15, 16, 18, 20, 22, 24].map((n) => (
                <option key={n} value={n}>{n} px</option>
              ))}
            </select>
          </label>
          <label className="settings-row">
            <span>回滚行数</span>
            <input
              type="number"
              min={100}
              max={100000}
              step={100}
              value={draft.terminal.scrollback}
              onChange={(e) => setTerm({ scrollback: Number(e.target.value) })}
            />
          </label>
          <label className="settings-row">
            <span>光标闪烁</span>
            <input
              type="checkbox"
              checked={draft.terminal.cursorBlink}
              onChange={(e) => setTerm({ cursorBlink: e.target.checked })}
            />
          </label>
        </div>

        <div className="settings-group">
          <div className="settings-title">窗口</div>
          <label className="settings-row">
            <span>关闭时最小化到托盘</span>
            <input
              type="checkbox"
              checked={draft.closeToTray}
              onChange={(e) => setDraft({ ...draft, closeToTray: e.target.checked })}
            />
          </label>
        </div>

        {error && <p className="error">{error}</p>}
        <div className="dialog-actions">
          <span className="settings-version">{version && `XLink v${version}`}</span>
          <button className="secondary" onClick={onClose}>取消</button>
          <button onClick={doSave} disabled={busy}>
            {busy ? "保存中…" : "保存"}
          </button>
        </div>
      </div>
    </div>
  );
}
