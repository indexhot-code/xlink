import { useState } from "react";
import * as ConnectionService from "../../bindings/xlink/backend/connection/service.js";
import * as VaultService from "../../bindings/xlink/backend/vault/service.js";
import type { Host } from "../../bindings/xlink/backend/connection/models.js";

/** 主机表单：认证方式三选；密码/私钥口令存入保险库（生成 credentialId）。 */
export default function HostForm({
  host,
  onDone,
  onCancel,
}: {
  host: Host | null;
  onDone: () => void;
  onCancel: () => void;
}) {
  const [name, setName] = useState(host?.name ?? "");
  const [addr, setAddr] = useState(host?.host ?? "");
  const [port, setPort] = useState(host?.port ?? "22");
  const [user, setUser] = useState(host?.user ?? "");
  const [auth, setAuth] = useState<number>(host?.auth ?? 0);
  const [secret, setSecret] = useState("");
  const [keyPath, setKeyPath] = useState(host?.keyPath ?? "");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (!addr || !user) {
      setError("主机与用户必填");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      let credentialId = Number(host?.credentialId) || 0;
      if (auth !== 2 && secret) {
        const authType = auth === 0 ? "password" : "keyfile";
        const credName = (name || addr) + ":" + user;
        credentialId = await VaultService.SaveCredential({
          name: credName,
          auth: authType,
          secret,
          keyPath: "",
        });
      }
      await ConnectionService.SaveHost({
        id: Number(host?.id) || 0,
        name,
        host: addr,
        port,
        user,
        auth,
        credentialId,
        keyPath: auth === 1 ? keyPath : "",
      } as never);
      onDone();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className="dialog-backdrop">
      <div className="dialog">
        <h2>{host ? "编辑主机" : "新建主机"}</h2>
        <div className="form-grid">
          <label>名称</label>
          <input value={name} onChange={(e) => setName(e.target.value)} placeholder="可选" />
          <label>主机</label>
          <input value={addr} onChange={(e) => setAddr(e.target.value)} />
          <label>端口</label>
          <input value={port} onChange={(e) => setPort(e.target.value)} />
          <label>用户</label>
          <input value={user} onChange={(e) => setUser(e.target.value)} />
          <label>认证</label>
          <select value={auth} onChange={(e) => setAuth(Number(e.target.value))}>
            <option value={0}>密码</option>
            <option value={1}>密钥文件</option>
            <option value={2}>Agent / Pageant</option>
          </select>
          {auth === 0 && (
            <>
              <label>密码</label>
              <input
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder={host?.credentialId ? "已保存（留空保持不变）" : "密码"}
              />
            </>
          )}
          {auth === 1 && (
            <>
              <label>私钥路径</label>
              <input
                value={keyPath}
                onChange={(e) => setKeyPath(e.target.value)}
                placeholder="C:\Users\you\.ssh\id_ed25519"
              />
              <label>私钥口令</label>
              <input
                type="password"
                value={secret}
                onChange={(e) => setSecret(e.target.value)}
                placeholder="可空"
              />
            </>
          )}
          {auth === 2 && (
            <>
              <label></label>
              <span className="hint-inline">使用系统 SSH Agent / Pageant 中的密钥</span>
            </>
          )}
        </div>
        {error && <p className="error">{error}</p>}
        <div className="dialog-actions">
          <button className="secondary" onClick={onCancel} disabled={busy}>
            取消
          </button>
          <button onClick={save} disabled={busy}>
            {busy ? "保存中…" : "保存"}
          </button>
        </div>
      </div>
    </div>
  );
}
