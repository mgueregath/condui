import { useTranslation } from "react-i18next";
import { FaTerminal, FaFolderOpen, FaDocker, FaDesktop, FaDatabase, FaLock } from "react-icons/fa";
import { BiTransfer } from "react-icons/bi";
import { GiWarpPipe } from "react-icons/gi";
import { LuNetwork } from "react-icons/lu";
import { PiCpuFill } from "react-icons/pi";

// Phone-only bottom navigation: one full-screen view per entry.
// ids "terminal" | "resources" | "files" | panel tab ids.
const ITEMS = [
  { id: "terminal", labelKey: "mobile.console", icon: <FaTerminal /> },
  { id: "resources", labelKey: "mobile.resources", icon: <PiCpuFill /> },
  { id: "files", labelKey: "mobile.files", icon: <FaFolderOpen /> },
  { id: "transfers", labelKey: "panel.transfers", icon: <BiTransfer /> },
  { id: "tunnels", labelKey: "panel.tunnels", icon: <GiWarpPipe /> },
  { id: "ports", labelKey: "panel.ports", icon: <LuNetwork /> },
  { id: "docker", labelKey: "panel.docker", icon: <FaDocker /> },
  { id: "virtualbox", labelKey: "panel.virtualbox", icon: <FaDesktop /> },
  { id: "databases", labelKey: "panel.databases", icon: <FaDatabase />, pro: true },
];

export default function MobileNav({ view, onChange, isPro, onUpgrade, hideFiles }) {
  const { t } = useTranslation();
  return (
    <nav className="mobile-nav" aria-label="Navigation">
      {ITEMS.filter((item) => !(hideFiles && item.id === "files")).map((item) => {
        const locked = item.pro && !isPro;
        return (
          <button
            key={item.id}
            type="button"
            className={`mobile-nav-item${view === item.id ? " active" : ""}${locked ? " locked" : ""}`}
            onClick={() => (locked ? onUpgrade?.() : onChange(item.id))}
            ref={(el) => {
              if (el && view === item.id) el.scrollIntoView({ inline: "center", block: "nearest" });
            }}
          >
            <span className="mobile-nav-icon">{item.icon}</span>
            <span className="mobile-nav-label">
              {t(item.labelKey)}
              {locked && <FaLock className="mobile-nav-lock" />}
            </span>
          </button>
        );
      })}
    </nav>
  );
}
