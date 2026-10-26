import { useSyncExternalStore } from "react";
import * as AppSettingsService from "../../bindings/xlink/backend/appsettings/service.js";

/** 应用设置（终端行为 / 关闭到托盘）。持久化在 Go 侧 settings 表。 */

export interface TerminalSettings {
  fontSize: number;
  scrollback: number;
  cursorBlink: boolean;
}

export interface AppSettings {
  terminal: TerminalSettings;
  closeToTray: boolean;
}

export const defaultSettings: AppSettings = {
  terminal: { fontSize: 13, scrollback: 5000, cursorBlink: true },
  closeToTray: true,
};

let settings: AppSettings = defaultSettings;
let loaded = false;
const listeners = new Set<() => void>();

// 缓存快照对象：useSyncExternalStore 的 getSnapshot 必须返回稳定引用
let snapshot: { settings: AppSettings; loaded: boolean } = { settings, loaded };

function emit() {
  snapshot = { settings, loaded };
  listeners.forEach((l) => l());
}

function subscribe(l: () => void) {
  listeners.add(l);
  return () => listeners.delete(l);
}

export function useSettings(): { settings: AppSettings; loaded: boolean } {
  return useSyncExternalStore(subscribe, () => snapshot);
}

/** getSettings：模块级读取（供 createTerminal 等非组件调用）。 */
export function getSettings(): AppSettings {
  return settings;
}

export async function loadSettings(): Promise<void> {
  try {
    settings = (await AppSettingsService.GetAppSettings()) as AppSettings;
  } catch {
    settings = defaultSettings;
  }
  loaded = true;
  emit();
}

export async function saveSettings(next: AppSettings): Promise<void> {
  await AppSettingsService.SaveAppSettings(next);
  settings = next;
  loaded = true;
  emit();
}
