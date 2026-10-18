import { useTranslation } from "react-i18next";
import "./ResponseMessage.css";

export default function ResponseMessage({ severity = "info", message, details, progress }) {
  const { t } = useTranslation();
  if (!message) return null;
  return (
    <div className="response-message" data-severity={severity} role={severity === "error" ? "alert" : "status"}>
      <p>{message}</p>
      {progress !== undefined && <progress aria-label={t("common.response.progress")} max={100} value={progress ?? undefined} />}
      {details && <details><summary>{t("common.response.details")}</summary><pre>{details}</pre></details>}
    </div>
  );
}
