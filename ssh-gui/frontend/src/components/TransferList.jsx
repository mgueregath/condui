import { useState } from "react";
import { CancelTransfer } from "../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";
import { LuDownload, LuUpload, LuX, LuLoader } from "react-icons/lu";
import "./TransferList.css";

export default function TransferList({ transfers }) {
  const { t } = useTranslation();
  const [cancelling, setCancelling] = useState({});
  const [error, setError] = useState("");

  const cancel = async (id) => {
    setCancelling((prev) => ({ ...prev, [id]: true }));
    setError("");
    try {
      await CancelTransfer(id);
    } catch (err) {
      setCancelling((prev) => ({ ...prev, [id]: false }));
      setError(err?.message || String(err));
    }
  };

  const items = Object.values(transfers);
  return (
    <div className="transfer-list">
      {error && <p className="transfer-alert" role="alert">{error}</p>}
      {items.length === 0 ? (
        <p className="transfer-empty">{t("panel.noTransfers")}</p>
      ) : (
        <div className="transfer-table-scroll">
          <table className="transfer-table">
            <colgroup>
              <col className="transfer-direction-column" />
              <col />
              <col className="transfer-size-column" />
              <col className="transfer-progress-column" />
              <col className="transfer-speed-column" />
              <col className="transfer-status-column" />
              <col className="transfer-actions-column" />
            </colgroup>
            <thead>
              <tr>
                <th scope="col"><span className="transfer-sr-only">{t("panel.transferDirection")}</span></th>
                <th scope="col">{t("panel.transferFile")}</th>
                <th scope="col" className="transfer-number">{t("panel.transferSize")}</th>
                <th scope="col">{t("panel.transferProgress")}</th>
                <th scope="col" className="transfer-number">{t("panel.transferAverageSpeed")}</th>
                <th scope="col">{t("panel.status")}</th>
                <th scope="col" className="transfer-actions">{t("panel.actions")}</th>
              </tr>
            </thead>
            <tbody>
              {items.map((transfer) => {
                const cancelLabel = cancelling[transfer.id]
                  ? t("panel.transferCancelling")
                  : t("common.cancel");
                return (
                  <tr key={transfer.id}>
                    <td className="transfer-direction" title={t("panel.transfer" + (transfer.direction === "upload" ? "Upload" : "Download"))}>
                      {transfer.direction === "upload" ? <LuUpload aria-hidden="true" /> : <LuDownload aria-hidden="true" />}
                      <span className="transfer-sr-only">{t("panel.transfer" + (transfer.direction === "upload" ? "Upload" : "Download"))}</span>
                    </td>
                    <td className="transfer-name" title={transfer.name}>{transfer.name}</td>
                    <td className="transfer-number">{((transfer.total || 0) / (1024 * 1024)).toFixed(2)} MiB</td>
                    <td>
                      <div className="transfer-progress-cell">
                        <div
                          className="transfer-progress"
                          role="progressbar"
                          aria-label={transfer.name}
                          aria-valuemin={0}
                          aria-valuemax={100}
                          aria-valuenow={transfer.progress || 0}
                        >
                          <div
                            className="transfer-progress-fill"
                            data-status={transfer.status}
                            style={{ width: (transfer.progress || 0) + "%" }}
                          />
                        </div>
                        <span className="transfer-percent">{transfer.progress || 0}%</span>
                      </div>
                    </td>
                    <td className="transfer-number">{((transfer.bytesPerSecond || 0) / (1024 * 1024)).toFixed(2)} MiB/s</td>
                    <td>
                      <span className="transfer-status" data-status={transfer.status}>
                        {t("panel.transferStatus." + transfer.status)}
                      </span>
                    </td>
                    <td className="transfer-actions">
                      {transfer.status === "active" && (
                        <button
                          type="button"
                          className="transfer-action"
                          title={cancelLabel}
                          aria-label={cancelLabel}
                          disabled={Boolean(cancelling[transfer.id])}
                          onClick={() => cancel(transfer.id)}
                        >
                          {cancelling[transfer.id] ? <LuLoader aria-hidden="true" /> : <LuX aria-hidden="true" />}
                        </button>
                      )}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
