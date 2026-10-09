import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { FaTrash } from "react-icons/fa";
import { containerName, volumeReferences } from "./explorerData";
import "./VolumesView.css";

export default function VolumesView({ volumes, containers, query, busy, onDelete }) {
 const { t, i18n } = useTranslation("dockerExplorer");
 const [unused, setUnused] = useState(false);
 const [selected, setSelected] = useState(new Set());
 const selectAll = useRef(null);
 const rows = volumes.map(volume => ({volume, refs: volumeReferences(containers, volume)})).filter(({volume, refs}) =>
  [volume.Name, volume.Driver, volume.Mountpoint].join(" ").toLowerCase().includes(query) && (!unused || !refs.length)
 ).sort((a, b) => a.volume.Name.localeCompare(b.volume.Name));
 const selectable = rows.filter(row => !row.refs.length).map(row => row.volume.Name);
 const count = selectable.filter(name => selected.has(name)).length;
 const allSelected = selectable.length > 0 && count === selectable.length;
 const selectedVolumes = volumes.filter(volume => selected.has(volume.Name) && !volumeReferences(containers, volume).length);
 useEffect(() => { setSelected(new Set()); }, [volumes, containers]);
 useEffect(() => {
  if (selectAll.current) selectAll.current.indeterminate = count > 0 && count < selectable.length;
 }, [count, selectable.length]);
 function toggle(name) {
  setSelected(previous => {
   const next = new Set(previous);
   if (next.has(name)) next.delete(name); else next.add(name);
   return next;
  });
 }
 function toggleAll() {
  setSelected(previous => {
   const next = new Set(previous);
   selectable.forEach(name => { if (allSelected) next.delete(name); else next.add(name); });
   return next;
  });
 }
 return <>
  <div className="dx-toolbar">
   <label><input type="checkbox" checked={unused} onChange={event => setUnused(event.target.checked)} />{t("unusedOnly")}</label>
   <button type="button" className="btn-secondary dx-delete" disabled={busy || !selectedVolumes.length} onClick={() => onDelete(selectedVolumes)}>{t("deleteSelected")} ({selectedVolumes.length})</button>
  </div>
  <p className="dx-note">{t("volumeNote")}</p>
  <div className="dx-table-wrap"><table className="dx-table dx-volumes-table">
   <colgroup><col className="dx-volume-col-select" /><col className="dx-volume-col-name" /><col className="dx-volume-col-driver" /><col className="dx-volume-col-created" /><col className="dx-volume-col-path" /><col /><col className="dx-volume-col-actions" /></colgroup>
   <thead><tr>
    <th><label><input ref={selectAll} type="checkbox" checked={allSelected} disabled={busy || !selectable.length} onChange={toggleAll} aria-label={t(allSelected ? "deselectAll" : "selectAll")} title={t("selectVisibleVolumes")} /></label></th>
    {["volume", "driver", "created", "mountpoint", "usage", "actions"].map(key => <th key={key}>{t(key)}</th>)}
   </tr></thead>
   <tbody>{rows.map(({volume, refs}) => {
    const created = volume.CreatedAt ? new Date(volume.CreatedAt) : null;
    const hasDate = created && !Number.isNaN(created.getTime());
    return <tr key={volume.Name}>
     <td><input type="checkbox" aria-label={t("selectVolume", {name: volume.Name})} checked={selected.has(volume.Name)} disabled={busy || !!refs.length} onChange={() => toggle(volume.Name)} /></td>
     <td><span className="dx-volume-text" title={volume.Name}>{volume.Name}</span></td>
     <td><span className="dx-volume-text" title={volume.Driver}>{volume.Driver}</span><small>{volume.Scope}</small></td>
     <td title={hasDate ? created.toLocaleString(i18n.language) : ""}>{hasDate ? <>{created.toLocaleDateString(i18n.language)}<small>{created.toLocaleTimeString(i18n.language)}</small></> : "-"}</td>
     <td><span className="dx-volume-text" title={volume.Mountpoint}>{volume.Mountpoint || "-"}</span></td>
     <td>{refs.length ? <ul className="dx-volume-usage">{refs.map(container => <li key={container.Id} title={containerName(container) + " (" + container.State.Status + ")"}>{containerName(container)} ({container.State.Status})</li>)}</ul> : t("unused")}</td>
     <td><button type="button" title={t("delete")} aria-label={t("deleteVolume", {name: volume.Name})} className="btn-secondary dx-delete" disabled={busy || !!refs.length} onClick={() => onDelete([volume])}><FaTrash aria-hidden="true" /></button></td>
    </tr>;
   })}</tbody>
  </table></div>
  {!rows.length && <p>{t("emptyVolumes")}</p>}
 </>;
}
