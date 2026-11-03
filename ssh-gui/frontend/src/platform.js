import { useSyncExternalStore } from "react";

// Marks <html> with platform classes so mobile styles can be scoped without
// touching the desktop look:
//   is-mac      desktop macOS (window buttons sit over the top bar)
//   is-android  any Android device (phone or tablet)
//   is-phone    Android with a small viewport; tablets keep the desktop layout
const root = document.documentElement;
const isAndroid = /Android/i.test(navigator.userAgent || "");
// Desktop macOS only (iOS user agents also say "like Mac OS X").
const isMac = /Macintosh|Mac OS X/i.test(navigator.userAgent || "") && !isAndroid && !/iPhone|iPad|iPod/i.test(navigator.userAgent || "");
const phoneQuery = window.matchMedia("(max-width: 600px), (max-height: 480px)");

function apply() {
  root.classList.toggle("is-mac", isMac);
  root.classList.toggle("is-android", isAndroid);
  root.classList.toggle("is-phone", isAndroid && phoneQuery.matches);
}

apply();
phoneQuery.addEventListener("change", apply);

export const isAndroidApp = isAndroid;
export const isPhone = () => root.classList.contains("is-phone");

// React hook for components that render differently on phones. Always false on
// desktop and tablets, so their DOM is unchanged.
const subscribe = (cb) => {
  phoneQuery.addEventListener("change", cb);
  return () => phoneQuery.removeEventListener("change", cb);
};
export const useIsPhone = () =>
  useSyncExternalStore(subscribe, () => isAndroid && phoneQuery.matches);
