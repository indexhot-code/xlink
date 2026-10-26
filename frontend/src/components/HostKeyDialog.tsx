import { useState } from "react";
import * as ConnectionService from "../../bindings/xlink/backend/connection/service.js";

/**
 * HostKeyDialog：主机密钥不匹配告警（潜在中间人）。
 * 用户显式批准后重写 known_hosts；拒绝则保持阻断（架构规则 5）。
 */

export interface HostKeyPrompt {
  host: string;
  keyType: string;
  fingerprint: string;
}

export default function HostKeyDialog({
  prompt,
  onApproved,
  onRejected,
}: {
  prompt: HostKeyPrompt;
  onApproved: () => void;
  onRejected: () => void;
}) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const approve = async () => {
    setBusy(true);
    setError(null);
    try {
      await ConnectionService.ApproveHostKey(prompt.host, prompt.keyType, prompt.fingerprint);
      onApproved();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dialog-backdrop">
      <div className="dialog">
        <h2>主机密钥不匹配</h2>
        <p>
          主机 <b>{prompt.host}</b> 的密钥与已知记录不一致。这可能是服务器重装，
          也可能是<b>中间人攻击</b>。
        </p>
        <dl className="keyinfo">
          <dt>密钥类型</dt>
          <dd>{prompt.keyType}</dd>
          <dt>新指纹</dt>
          <dd className="mono">{prompt.fingerprint}</dd>
        </dl>
        {error && <p className="error">{error}</p>}
        <div className="dialog-actions">
          <button className="secondary" onClick={onRejected} disabled={busy}>
            拒绝并断开
          </button>
          <button onClick={approve} disabled={busy}>
            {busy ? "更新中…" : "我已确认，更新记录"}
          </button>
        </div>
      </div>
    </div>
  );
}

/** 从 CreateSession 错误中判断是否主机密钥不匹配，并取出待批准信息。 */
export async function resolveHostKeyPrompt(err: unknown, host: string): Promise<HostKeyPrompt | null> {
  const msg = String(err ?? "");
  if (!msg.includes("host key mismatch")) return null;
  try {
    const info = await ConnectionService.GetPendingHostKeyInfo(host);
    return { host: info.host, keyType: info.keyType, fingerprint: info.fingerprint };
  } catch {
    return null;
  }
}
