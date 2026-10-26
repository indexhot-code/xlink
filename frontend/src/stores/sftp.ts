import { useSyncExternalStore } from "react";

/** SFTP 面板状态：每个标签可切换 终端/SFTP 视图；记录当前路径。 */

export interface SftpPaneState {
  localPath: string;
  remotePath: string;
}

let openFor: string | null = null; // tab key
let panes: Record<string, SftpPaneState> = {};
const listeners = new Set<() => void>();

// 缓存快照对象：useSyncExternalStore 的 getSnapshot 必须返回稳定引用
let snapshot: { openFor: string | null; panes: Record<string, SftpPaneState> } = {
  openFor,
  panes,
};

function emit() {
  snapshot = { openFor, panes };
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useSftp() {
  return useSyncExternalStore(subscribe, () => snapshot);
}

export function toggleSftp(tabKey: string) {
  openFor = openFor === tabKey ? null : tabKey;
  emit();
}

export function closeSftp() {
  openFor = null;
  emit();
}

export function setLocalPath(tabKey: string, p: string) {
  panes = { ...panes, [tabKey]: { remotePath: panes[tabKey]?.remotePath || "/", localPath: p } };
  emit();
}

export function setRemotePath(tabKey: string, p: string) {
  panes = { ...panes, [tabKey]: { localPath: panes[tabKey]?.localPath || "C:\\", remotePath: p } };
  emit();
}

export function getPane(tabKey: string): SftpPaneState {
  return panes[tabKey] || { localPath: "C:\\", remotePath: "/" };
}
