import { useSyncExternalStore } from "react";
import * as VaultService from "../../bindings/xlink/backend/vault/service.js";

/**
 * 保险库状态（最小等效 store，useSyncExternalStore 实现）。
 * 主口令仅在 Unlock 调用瞬间流经前端，之后全部在 Go 侧内存中。
 */

export interface VaultStatus {
  loaded: boolean; // 首次 Status 查询是否已完成（未完成前不渲染设置/解锁表单）
  configured: boolean;
  unlocked: boolean;
}

let status: VaultStatus = { loaded: false, configured: false, unlocked: false };
const listeners = new Set<() => void>();

function setStatus(next: VaultStatus) {
  status = next;
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useVault(): VaultStatus {
  return useSyncExternalStore(subscribe, () => status);
}

export async function refreshVaultStatus(): Promise<void> {
  const s = await VaultService.Status();
  setStatus({ loaded: true, configured: s.configured, unlocked: s.unlocked });
}

export async function setupMasterPassword(password: string): Promise<void> {
  await VaultService.SetupMasterPassword(password);
  await refreshVaultStatus();
}

export async function unlockVault(password: string): Promise<void> {
  await VaultService.Unlock(password);
  await refreshVaultStatus();
}

export async function lockVault(): Promise<void> {
  await VaultService.Lock();
  await refreshVaultStatus();
}
