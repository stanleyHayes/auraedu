import { Platform } from "react-native";
import * as ExpoSecureStore from "expo-secure-store";

// expo-secure-store is backed by the native iOS Keychain / Android Keystore and
// has no web backend — calling it on web throws
// "ExpoSecureStore.default.getValueWithKeyAsync is not a function". The mobile
// app targets native (teacher/parent/student); web is a preview/dev surface, so
// fall back to localStorage there. Callers import this module in place of
// expo-secure-store; the async API is identical.
interface WebStore {
  getItem(key: string): string | null;
  setItem(key: string, value: string): void;
  removeItem(key: string): void;
}

const webStore: WebStore | undefined =
  Platform.OS === "web" ? (globalThis as { localStorage?: WebStore }).localStorage : undefined;

export async function getItemAsync(key: string): Promise<string | null> {
  if (Platform.OS === "web") {
    try {
      return webStore?.getItem(key) ?? null;
    } catch {
      return null;
    }
  }
  return ExpoSecureStore.getItemAsync(key);
}

export async function setItemAsync(key: string, value: string): Promise<void> {
  if (Platform.OS === "web") {
    try {
      webStore?.setItem(key, value);
    } catch {
      /* storage blocked (private mode) — ignore */
    }
    return;
  }
  await ExpoSecureStore.setItemAsync(key, value);
}

export async function deleteItemAsync(key: string): Promise<void> {
  if (Platform.OS === "web") {
    try {
      webStore?.removeItem(key);
    } catch {
      /* ignore */
    }
    return;
  }
  await ExpoSecureStore.deleteItemAsync(key);
}
