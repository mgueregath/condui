const KEY = "condui-theme";
const modes = ["light", "dark", "system"];
const system = window.matchMedia("(prefers-color-scheme: dark)");
let preference = "system";

try {
  const saved = localStorage.getItem(KEY);
  if (modes.includes(saved)) preference = saved;
} catch (_) {}

function applyTheme() {
  const resolved = preference === "system" ? (system.matches ? "dark" : "light") : preference;
  document.documentElement.dataset.theme = resolved;
  document.documentElement.style.colorScheme = resolved;
}

export function getTheme() {
  return preference;
}

export function setTheme(value) {
  if (!modes.includes(value)) return;
  preference = value;
  try { localStorage.setItem(KEY, value); } catch (_) {}
  applyTheme();
  window.dispatchEvent(new Event("themechange"));
}

system.addEventListener("change", applyTheme);
window.addEventListener("storage", (event) => {
  if (event.key !== KEY && event.key !== null) return;
  preference = modes.includes(event.newValue) ? event.newValue : "system";
  applyTheme();
  window.dispatchEvent(new Event("themechange"));
});
applyTheme();
