import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { getTheme, setTheme } from "../../services/theme";

export default function ThemeSwitcher() {
  const { t } = useTranslation();
  const [theme, updateTheme] = useState(getTheme);

  useEffect(() => {
    const refresh = () => updateTheme(getTheme());
    window.addEventListener("themechange", refresh);
    return () => window.removeEventListener("themechange", refresh);
  }, []);

  return (
    <select className="theme-switcher" aria-label={t("settings.theme.title")} value={theme} onChange={(event) => setTheme(event.target.value)}>
      {["light", "dark", "system"].map((mode) => <option key={mode} value={mode}>{t(`settings.theme.${mode}`)}</option>)}
    </select>
  );
}
