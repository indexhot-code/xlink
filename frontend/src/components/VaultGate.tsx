import { useEffect, useState } from "react";
import {
  useVault,
  refreshVaultStatus,
  setupMasterPassword,
  unlockVault,
  lockVault,
} from "../stores/vault";
import { lockTabs, reconnectAfterUnlock } from "../stores/tabs";

/**
 * VaultGate：首次启动设置主口令 / 解锁对话框。
 * 状态未加载完成前显示加载态（避免误判为"首次使用"）；
 * 解锁前禁止任何连接操作。
 */
export default function VaultGate({ children }: { children: React.ReactNode }) {
  const vault = useVault();
  const [pw1, setPw1] = useState("");
  const [pw2, setPw2] = useState("");
  const [unlockPw, setUnlockPw] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    refreshVaultStatus();
  }, []);

  // 解锁后自动恢复锁定前连接着的标签
  useEffect(() => {
    if (vault.loaded && vault.unlocked) reconnectAfterUnlock();
  }, [vault.loaded, vault.unlocked]);

  if (!vault.loaded) {
    return (
      <main className="container">
        <h1>XLink</h1>
        <p className="subtitle">加载中…</p>
      </main>
    );
  }

  if (vault.unlocked) {
    return (
      <>
        {children}
        <button className="lock-btn" title="锁定保险库" onClick={() => lockVault().then(() => lockTabs())}>
          锁定
        </button>
      </>
    );
  }

  const doSetup = async () => {
    if (pw1.length < 8) {
      setError("主口令至少 8 个字符");
      return;
    }
    if (pw1 !== pw2) {
      setError("两次输入不一致");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await setupMasterPassword(pw1);
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  const doUnlock = async () => {
    setBusy(true);
    setError(null);
    try {
      await unlockVault(unlockPw);
    } catch (e) {
      setError("口令错误");
    } finally {
      setBusy(false);
    }
  };

  const isSetup = !vault.configured;

  return (
    <main className="container">
      <h1>XLink</h1>
      <p className="subtitle">
        {isSetup ? "首次使用：设置主口令" : "保险库已锁定"}
      </p>
      <div className="vault-form">
        {isSetup ? (
          <>
            <input
              type="password"
              placeholder="主口令（≥8 字符）"
              value={pw1}
              onChange={(e) => setPw1(e.target.value)}
            />
            <input
              type="password"
              placeholder="确认主口令"
              value={pw2}
              onChange={(e) => setPw2(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && doSetup()}
            />
            <button onClick={doSetup} disabled={busy}>
              {busy ? "生成中…" : "设置并解锁"}
            </button>
          </>
        ) : (
          <>
            <input
              type="password"
              placeholder="主口令"
              value={unlockPw}
              onChange={(e) => setUnlockPw(e.target.value)}
              onKeyDown={(e) => e.key === "Enter" && doUnlock()}
            />
            <button onClick={doUnlock} disabled={busy}>
              {busy ? "解锁中…" : "解锁"}
            </button>
          </>
        )}
      </div>
      {error && <p className="error">{error}</p>}
      <p className="hint">
        主口令用于加密保存的密码与私钥口令（scrypt + AES-256-GCM），
        无法找回，请妥善保管。
      </p>
    </main>
  );
}
