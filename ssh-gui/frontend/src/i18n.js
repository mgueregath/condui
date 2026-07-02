import i18n from "i18next";
import { initReactI18next } from "react-i18next";

import en from "./locales/en.json";
import es from "./locales/es.json";

const savedLanguage = localStorage.getItem("condui.language");
const browserLanguage = navigator.language?.split("-")[0];
const language = savedLanguage || (browserLanguage === "en" ? "en" : "es");
console.log("Language set to:", language, "Saved language:", savedLanguage, "Browser language:", browserLanguage);

i18n.use(initReactI18next).init({
  resources: {
    en: { translation: en },
    es: { translation: es },
  },
  lng: language,
  fallbackLng: "en",
  interpolation: {
    escapeValue: false,
  },
});

i18n.on("languageChanged", (nextLanguage) => {
  localStorage.setItem("condui.language", nextLanguage);
  document.documentElement.lang = nextLanguage;
});

document.documentElement.lang = language;

export default i18n;
