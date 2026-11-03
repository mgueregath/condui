import { useSyncExternalStore } from "react";
import { isAndroidApp } from "./platform";

// Android has no extra native windows (docker logs, stacks, DB explorer...), so
// they are simulated inside the app: each one is a "window" listed as a tab in the
// top bar and shown over the content area. Desktop keeps its native windows.
export const usesInAppWindows = isAndroidApp;

// spec: { kind: "docker-explorer" | "url", title, sessionID, url? }
let state = { windows: [], activeId: null }; // activeId null = the main app
let seq = 0;
const listeners = new Set();

const emit = (next) => {
  state = next;
  listeners.forEach((l) => l());
};
const subscribe = (l) => {
  listeners.add(l);
  return () => listeners.delete(l);
};
export const useInAppWindows = () => useSyncExternalStore(subscribe, () => state);

function remove(ids) {
  const windows = state.windows.filter((w) => !ids.includes(w.id));
  if (windows.length === state.windows.length) return;
  const activeId = ids.includes(state.activeId) ? (windows.at(-1)?.id ?? null) : state.activeId;
  emit({ windows, activeId });
}

export function openInAppWindow(spec) {
  const key = spec.key || `${spec.kind}:${spec.sessionID || ""}:${spec.url || ""}`;
  const existing = state.windows.find((w) => w.key === key);
  if (existing) return emit({ ...state, activeId: existing.id });
  seq += 1;
  emit({ windows: [...state.windows, { ...spec, id: seq, key }], activeId: seq });
}
export const activateInAppWindow = (id) => emit({ ...state, activeId: id });
export const closeInAppWindow = (id) => remove([id]);
export const closeSessionWindows = (sessionID) =>
  remove(state.windows.filter((w) => w.sessionID === sessionID).map((w) => w.id));

// System Back: MainActivity asks the page first. Returns true when a window was closed
// (the one in view, or the latest one if the main app is showing), false to let the
// app handle Back as usual. No history entries involved: Chrome skips entries a page
// adds without a user gesture, which made a second Back leave the app.
window.__conduiBack = () => {
  if (!state.windows.length) return false;
  remove([state.activeId ?? state.windows.at(-1).id]);
  return true;
};
