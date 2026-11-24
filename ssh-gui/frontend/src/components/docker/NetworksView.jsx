import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { FaTrash } from "react-icons/fa";

export default function NetworksView({ networks, query, busy, onDelete }) {
 const { t } = useTranslation("dockerExplorer");
 const [unused, setUnused] = useState(false);
 const [selected, setSelected] = useState(new Set());
 const rows = networks.map(network => ({
  ...network,
  subnets: (network.IPAM?.Config || []).map(config => config.Subnet).filter(Boolean).join(", ")
 })).filter(network =>
  (!unused || !network.InUse) &&
  [network.Name, network.ID, network.Driver, network.Scope, network.subnets].join(" ").toLowerCase().includes(query)
 ).sort((a, b) => a.Name.localeCompare(b.Name));
 const selectable = rows.filter(network => !network.InUse && !network.Protected);
 const selectedRows = selectable.filter(network => selected.has(network.ID));
 const allSelected = selectable.length > 0 && selectedRows.length === selectable.length;
 const selectAll = useRef(null);

 useEffect(() => { setSelected(new Set()); }, [networks]);
 useEffect(() => {
  if (selectAll.current) selectAll.current.indeterminate = selectedRows.length > 0 && !allSelected;
 }, [selectedRows.length, allSelected]);

 function toggle(id) {
  setSelected(previous => {
   const next = new Set(previous);
   if (next.has(id)) next.delete(id);
   else next.add(id);
   return next;
  });
 }

 function toggleAll() {
  setSelected(previous => {
   const next = new Set(previous);
   for (const { ID } of selectable) {
    if (allSelected) next.delete(ID);
    else next.add(ID);
   }
   return next;
  });
 }

 return <>
  <div className="dx-toolbar">
   <label><input type="checkbox" checked={unused} onChange={event => setUnused(event.target.checked)} />{t("network.unused")}</label>
   <button type="button" className="btn-secondary dx-delete" disabled={busy || !selectedRows.length}
    onClick={() => onDelete(selectedRows)}>{t("deleteSelected")} ({selectedRows.length})</button>
  </div>
  {rows.length ? <div className="dx-table-wrap">
   <table className="dx-table">
    <thead><tr>
     <th><input ref={selectAll} type="checkbox" checked={allSelected} disabled={busy || !selectable.length}
      aria-label={t(allSelected ? "deselectAll" : "selectAll")} onChange={toggleAll} /></th>
     {["network.name", "driver", "network.scope", "network.subnets", "usage", "actions"].map(key => <th key={key}>{t(key)}</th>)}
    </tr></thead>
    <tbody>{rows.map(network => {
     const blocked = network.InUse || network.Protected;
     return <tr key={network.ID}>
      <td><input type="checkbox" aria-label={t("network.select", { name: network.Name })} disabled={busy || blocked}
       checked={!blocked && selected.has(network.ID)} onChange={() => toggle(network.ID)} /></td>
      <td title={network.ID}>{network.Name}</td>
      <td>{network.Driver}</td>
      <td>{network.Scope}</td>
      <td>{network.subnets || "-"}</td>
      <td>{t(network.Protected ? "network.protected" : network.InUse ? "network.inUse" : "unused")}</td>
      <td><button type="button" className="btn-secondary dx-delete" title={t("delete")}
       aria-label={t("delete") + " " + network.Name} disabled={busy || blocked} onClick={() => onDelete([network])}>
       <FaTrash aria-hidden="true" />
      </button></td>
     </tr>;
    })}</tbody>
   </table>
  </div> : <p>{t("network.empty")}</p>}
 </>;
}
