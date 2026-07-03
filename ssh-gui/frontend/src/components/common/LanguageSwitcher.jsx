import { FaGlobe } from "react-icons/fa";
import { useTranslation } from "react-i18next";

const LANGUAGES = [
  { code: "es", label: "Español" },
  { code: "en", label: "English" },
];

export default function LanguageSwitcher() {
  const { t, i18n } = useTranslation();
  const currentLanguage = i18n.resolvedLanguage || i18n.language || "es";

  return (
    <label className="language-switcher">
      <span className="language-switcher-icon" aria-hidden="true">
        <FaGlobe />
      </span>
      <span className="sr-only">{t("settings.language")}</span>
      <select
        aria-label={t("settings.language")}
        value={currentLanguage.split("-")[0]}
        onChange={(event) => i18n.changeLanguage(event.target.value)}
      >
        {LANGUAGES.map((language) => (
          <option key={language.code} value={language.code}>
            {language.label}
          </option>
        ))}
      </select>
    </label>
  );
}
