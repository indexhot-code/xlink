// 多标签状态机 + 重启恢复。
// 持久化仅存 hostId（架构要求）：快照变更防抖写回；
// 内存态 sessionId 绝不落盘，重启后按 hostId 重建会话。
import { useSyncExternalStore } from "react";
import * as TerminalService from "../../bindings/xlink/backend/terminal/service.js";
import * as LayoutService from "../../bindings/xlink/backend/layout/service.js";
import type { Host } from "../../bindings/xlink/backend/connection/models.js";
import type { PtySessionInfo } from "../lib/ws";
import { isUnlocked } from "./vault";

export type TabStatus = "connecting" | "connected" | "error" | "disconnected";

export interface Tab {
  key: string; // 内存态唯一键（不落盘）
  host: Host;
  status: TabStatus;
  session: PtySessionInfo | null;
  error: string | null;
}

let tabs: Tab[] = [];
let activeKey: string | null = null;
const listeners = new Set<() => void>();

// 缓存快照对象：useSyncExternalStore 的 getSnapshot 必须返回稳定引用
let snapshot: { tabs: readonly Tab[]; activeKey: string | null } = { tabs, activeKey };

function emit() {
  snapshot = { tabs, activeKey };
  listeners.forEach((l) => l());
  schedulePersist();
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useTabs(): { tabs: readonly Tab[]; activeKey: string | null } {
  return useSyncExternalStore(subscribe, () => snapshot);
}

let persistTimer: ReturnType<typeof setTimeout> | undefined;
function schedulePersist() {
  clearTimeout(persistTimer);
  persistTimer = setTimeout(() => {
    const snap = tabs.map((t, i) => ({ position: i + 1, hostId: Number(t.host.id) }));
    LayoutService.SaveSnapshot(snap as never).catch(console.error);
  }, 300);
}

let keyCounter = 0;
const nextKey = () => "tab-" + ++keyCounter + "-" + Date.now();

/** 打开主机：已开则仅激活；否则新标签并连接。 */
export async function openHost(host: Host): Promise<void> {
  const existing = tabs.find((t) => Number(t.host.id) === Number(host.id));
  if (existing) {
    activeKey = existing.key;
    emit();
    return;
  }
  const tab: Tab = { key: nextKey(), host, status: "connecting", session: null, error: null };
  tabs = [...tabs, tab];
  activeKey = tab.key;
  emit();
  await connectTab(tab.key);
}

/** 连接/重试某个标签。 */
export async function connectTab(key: string): Promise<void> {
  const tab = tabs.find((t) => t.key === key);
  if (!tab) return;
  if (!isUnlocked()) {
    // 保险库锁定期间不允许建会话（多为终端掉线自动重连触发）：
    // 标记为因锁定断开，解锁后自动恢复。
    if (tab.status !== "disconnected" && !lockedKeys.includes(key)) lockedKeys.push(key);
    update(key, { status: "disconnected", session: null, error: null });
    return;
  }
  update(key, { status: "connecting", error: null });
  try {
    const session = await TerminalService.CreateSession({
      host: tab.host.host,
      port: tab.host.port,
      user: tab.host.user,
      auth: tab.host.auth,
      hostId: Number(tab.host.id) || 0,
      credentialId: Number(tab.host.credentialId) || 0,
      keyPath: tab.host.keyPath || "",
    } as never);
    update(key, { status: "connected", session });
  } catch (e) {
    update(key, { status: "error", error: String(e) });
  }
}

let lockedKeys: string[] = [];

/** 锁定保险库：记住此前连接着的标签并置为因锁定断开。 */
export function lockTabs(): void {
  lockedKeys = tabs
    .filter((t) => t.status === "connected" || t.status === "connecting")
    .map((t) => t.key);
  tabs = tabs.map((t) =>
    lockedKeys.includes(t.key) ? { ...t, status: "disconnected" as const, session: null, error: null } : t,
  );
  emit();
}

/** 解锁后自动恢复锁定前连接着的标签。 */
export async function reconnectAfterUnlock(): Promise<void> {
  if (!lockedKeys.length) return;
  const keys = [...lockedKeys];
  lockedKeys = [];
  for (const k of keys) {
    if (tabs.some((t) => t.key === k)) await connectTab(k);
  }
}

export function closeTab(key: string): void {
  const tab = tabs.find((t) => t.key === key);
  if (tab?.session) {
    TerminalService.CloseSession(tab.session.sessionId).catch(console.error);
  }
  const idx = tabs.findIndex((t) => t.key === key);
  tabs = tabs.filter((t) => t.key !== key);
  if (activeKey === key) {
    activeKey = tabs[Math.min(idx, tabs.length - 1)]?.key || null;
  }
  emit();
}

/** 克隆标签：同一主机开新标签并建立新 SSH 会话。 */
export async function cloneTab(key: string): Promise<void> {
  const tab = tabs.find((t) => t.key === key);
  if (!tab) return;
  const nt: Tab = { key: nextKey(), host: tab.host, status: "connecting", session: null, error: null };
  tabs = [...tabs, nt];
  activeKey = nt.key;
  emit();
  await connectTab(nt.key);
}

/** 克隆主机连接：已有标签则克隆之，否则新建连接。 */
export async function cloneHost(host: Host): Promise<void> {
  const existing = tabs.find((t) => Number(t.host.id) === Number(host.id));
  if (existing) return cloneTab(existing.key);
  return openHost(host);
}

export function activateTab(key: string): void {
  activeKey = key;
  emit();
}

function update(key: string, patch: Partial<Tab>) {
  tabs = tabs.map((t) => (t.key === key ? { ...t, ...patch } : t));
  emit();
}

/** 重启恢复：读快照重建标签。可自动重连（keyfile/agent 或带凭据且保险库已解锁）
 * 直接连接；其余保持手动重连态（口令主机不自动弹凭据框，符合 v1 重连规则）。 */
export async function restoreSnapshot(hosts: Host[], vaultUnlocked: boolean): Promise<void> {
  if (tabs.length > 0) return; // 标签仍在（如锁定→解锁）：不重复重建
  let snap: Array<{ position: number; hostId: number }>;
  try {
    snap = (await LayoutService.GetSnapshot()) as never[];
  } catch {
    return;
  }
  if (!snap?.length) return;

  const autoKeys: string[] = [];
  for (const s of snap) {
    const host = hosts.find((h) => Number(h.id) === s.hostId);
    if (!host) continue; // 快照引用了不存在的主机：容忍并跳过
    const tab: Tab = { key: nextKey(), host, status: "error", session: null, error: null };
    tabs = [...tabs, tab];
    if (!activeKey) activeKey = tab.key;
    const canAuto =
      vaultUnlocked &&
      (host.auth === 1 || host.auth === 2 || (host.auth === 0 && Number(host.credentialId) > 0));
    if (canAuto) autoKeys.push(tab.key);
  }
  emit();
  for (const k of autoKeys) {
    connectTab(k).catch(console.error);
  }
}
