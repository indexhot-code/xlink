import { useEffect, useState } from "react";
import * as SftpService from "../../bindings/xlink/backend/sftp/service.js";

type Settings = { chunkSizeKB: number; resumeMode: string; overwrite: string };

/** SFTPSettings：全局传输设置（分块/续传模式/覆盖策略），存 DB 不硬编码。 */
export default function SFTPSettings({ onClose }: { onClose: () => void }) {
  const [set, setSet] = useState<Settings>({ chunkSizeKB: 512, resumeMode: "size_mtime", overwrite: "overwrite" });
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    SftpService.GetSettings().then((s) => setSet(s as never)).catch(console.error);
  }, []);

  const save = async () => {
    try {
      await SftpService.SaveSettings(set);
      onClose();
    } catch (e) {
      setError(String(e));
    }
  };

  return (
    <div className="dialog-backdrop">
      <div className="dialog">
        <h2>SFTP 传输设置</h2>
        <div className="form-grid">
          <label>分块大小 (KB)</label>
          <input
            type="number"
            value={set.chunkSizeKB}
            onChange={(e) => setSet({ ...set, chunkSizeKB: Number(e.target.value) })}
          />
          <label>续传校验</label>
          <select value={set.resumeMode} onChange={(e) => setSet({ ...set, resumeMode: e.target.value })}>
            <option value="size_mtime">大小 + mtime</option>
            <option value="sha256">SHA256 前缀校验</option>
            <option value="off">关闭</option>
          </select>
          <label>覆盖策略</label>
          <select value={set.overwrite} onChange={(e) => setSet({ ...set, overwrite: e.target.value })}>
            <option value="overwrite">覆盖重来</option>
            <option value="skip">报错跳过</option>
          </select>
        </div>
        {error && <p className="error">{error}</p>}
        <div className="dialog-actions">
          <button className="secondary" onClick={onClose}>取消</button>
          <button onClick={save}>保存</button>
        </div>
      </div>
    </div>
  );
}
