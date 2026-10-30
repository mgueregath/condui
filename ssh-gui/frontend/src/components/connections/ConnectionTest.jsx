import { useTranslation } from "react-i18next";
import { LuPlugZap } from "react-icons/lu";
import ResponseMessage from "../common/ResponseMessage";

export default function ConnectionTest({ protocol, action, success, disabled, onTest }) {
  const { t } = useTranslation();
  return <div className="connection-test">
    <button type="button" className="btn-secondary test-conn-btn" onClick={onTest} disabled={disabled || action.busy}>
      {action.busy ? <span className="conn-loader" /> : <LuPlugZap aria-hidden="true" />}
      {t(action.busy ? "connection.testing" : "connection.access.test", { protocol: protocol.toUpperCase() })}
    </button>
    <ResponseMessage {...(action.response || (success ? { severity: "success", message: t("connection.access.connected", { protocol: protocol.toUpperCase() }) } : {}))} />
  </div>;
}
