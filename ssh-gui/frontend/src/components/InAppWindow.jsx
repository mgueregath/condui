import { lazy, Suspense, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { FaDocker, FaDatabase, FaFileAlt, FaTimes, FaHome } from "react-icons/fa";
import { useInAppWindows, activateInAppWindow, closeInAppWindow } from "../inAppWindow";

const DockerExplorer = lazy(() => import("./docker/DockerExplorer"));

const ICONS = { "docker-explorer": FaDocker, db: FaDatabase, log: FaFileAlt };

// Top-bar tabs: the main app plus every open in-app window.
// sessionNames: sessionID -> connection name, shown under each window title.
export function InAppWindowTabs({ sessionNames = {} }) {
  const { t } = useTranslation();
  const { windows, activeId } = useInAppWindows();
  // Keep the active tab in view when the strip scrolls (narrow screens).
  useEffect(() => {
    document.querySelector(".inapp-tab.active")?.scrollIntoView({ inline: "nearest", block: "nearest" });
  }, [activeId, windows.length]);
  if (!windows.length) return null;
  return (
    <div className="inapp-tabs" role="tablist">
      <button
        type="button"
        role="tab"
        aria-selected={activeId === null}
        className={`inapp-tab${activeId === null ? " active" : ""}`}
        onClick={() => activateInAppWindow(null)}
      >
        <FaHome />
        <span className="inapp-tab-label">{t("windows.main")}</span>
      </button>
      {windows.map((w) => {
        const Icon = ICONS[w.icon] || ICONS[w.kind] || FaFileAlt;
        return (
          <div key={w.id} className={`inapp-tab${activeId === w.id ? " active" : ""}`}>
            <button type="button" role="tab" aria-selected={activeId === w.id} onClick={() => activateInAppWindow(w.id)}>
              <Icon />
              <span className="inapp-tab-text">
                <span className="inapp-tab-label">{w.title}</span>
                {sessionNames[w.sessionID] && <small className="inapp-tab-sub">{sessionNames[w.sessionID]}</small>}
              </span>
            </button>
            <button type="button" className="inapp-tab-close" aria-label={t("windows.close")} onClick={() => closeInAppWindow(w.id)}>
              <FaTimes />
            </button>
          </div>
        );
      })}
    </div>
  );
}

// Content of the windows. All stay mounted (only the active one is visible) so live
// log streams keep running while you look at another window.
export default function InAppWindowHost() {
  const { windows, activeId } = useInAppWindows();
  if (!windows.length) return null;
  return (
    <>
      {windows.map((w) => (
        <div key={w.id} className="inapp-window" hidden={activeId !== w.id}>
          {w.kind === "docker-explorer" && (
            <Suspense fallback={null}>
              <div className="inapp-window-scroll">
                <DockerExplorer sessionID={w.sessionID} />
              </div>
            </Suspense>
          )}
          {w.kind === "url" && <iframe title={w.title} src={w.url} />}
        </div>
      ))}
    </>
  );
}
