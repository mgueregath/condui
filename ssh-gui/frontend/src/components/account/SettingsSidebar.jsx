import { FaCloud, FaDownload, FaPalette, FaShieldAlt, FaUser } from "react-icons/fa";

const ITEMS = [
  { id: "account", icon: FaUser },
  { id: "sync", icon: FaCloud },
  { id: "appearance", icon: FaPalette },
  { id: "security", icon: FaShieldAlt },
  { id: "updates", icon: FaDownload },
];

export default function SettingsSidebar({ activeSection, onSelect, t }) {
  return (
    <aside className="settings-sidebar">
      <h2>{t("settings.title")}</h2>
      <nav aria-label={t("settings.title")}>
        {ITEMS.map(({ id, icon: Icon }) => (
          <button
            key={id}
            type="button"
            className={`settings-nav-item${activeSection === id ? " active" : ""}`}
            onClick={() => onSelect(id)}
          >
            <Icon aria-hidden="true" />
            <span>{t(`settings.sections.${id}`)}</span>
          </button>
        ))}
      </nav>
    </aside>
  );
}
