import { useEffect, useRef } from "react";
import { useTranslation } from "react-i18next";

export default function ConfirmAction({ request, onClose, onConfirm }) {
 const { t } = useTranslation("dockerExplorer");
 const dialog = useRef(null);
 useEffect(() => {
  if (request && !dialog.current.open) dialog.current.showModal();
  if (!request && dialog.current.open) dialog.current.close();
 }, [request]);
 return <dialog ref={dialog} className="dx-confirm" aria-labelledby="dx-confirm-title" onCancel={onClose}>
  {request && <>
   <h2 id="dx-confirm-title">{request.title}</h2>
   <p>{request.description}</p>
   {request.items?.length > 0 && <ul>{request.items.map(item => <li key={item}>{item}</li>)}</ul>}
   <div className="dx-confirm-actions">
    <button type="button" className="btn-secondary" autoFocus onClick={onClose}>{t("cancel")}</button>
    <button type="button" className="dx-danger" onClick={onConfirm}>{t("confirm")}</button>
   </div>
  </>}
 </dialog>;
}
