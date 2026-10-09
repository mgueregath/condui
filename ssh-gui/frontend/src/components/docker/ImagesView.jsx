import { useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { containerName, imageTags, imageReferences } from "./explorerData";
import { FaTrash } from "react-icons/fa";
import "./ImagesView.css";

export default function ImagesView({ images, containers, query, busy, onDelete }) {
 const { t, i18n } = useTranslation("dockerExplorer");
 const [unused, setUnused] = useState(false);
 const [untagged, setUntagged] = useState(false);
 const [selected, setSelected] = useState(new Set());
 const selectAll = useRef(null);
 const rows = images.map(image => ({image, refs: imageReferences(containers, image)})).filter(({image, refs}) =>
  [imageTags(image), image.Id].join(" ").toLowerCase().includes(query) &&
  (!unused || !refs.length) && (!untagged || !image.RepoTags?.length)
 ).sort((a, b) => a.image.Created.localeCompare(b.image.Created));
 const selectable = rows.filter(row => !row.refs.length).map(row => row.image.Id);
 const count = selectable.filter(id => selected.has(id)).length;
 const allSelected = selectable.length > 0 && count === selectable.length;
 const selectedImages = images.filter(image => selected.has(image.Id) && !imageReferences(containers, image).length);
 useEffect(() => { setSelected(new Set()); }, [images, containers]);
 useEffect(() => {
  if (selectAll.current) selectAll.current.indeterminate = count > 0 && count < selectable.length;
 }, [count, selectable.length]);
 function toggle(id) {
  setSelected(previous => {
   const next = new Set(previous);
   if (next.has(id)) next.delete(id); else next.add(id);
   return next;
  });
 }
 function toggleAll() {
  setSelected(previous => {
   const next = new Set(previous);
   selectable.forEach(id => { if (allSelected) next.delete(id); else next.add(id); });
   return next;
  });
 }
 return <>
  <div className="dx-toolbar">
   <label><input type="checkbox" checked={unused} onChange={event => setUnused(event.target.checked)} />{t("unusedOnly")}</label>
   <label><input type="checkbox" checked={untagged} onChange={event => setUntagged(event.target.checked)} />{t("untaggedOnly")}</label>
   <button type="button" className="btn-secondary dx-delete" disabled={busy || !selectedImages.length} onClick={() => onDelete(selectedImages)}>{t("deleteSelected")} ({selectedImages.length})</button>
  </div>
  <p className="dx-note">{t("imageNote")}</p>
  <div className="dx-table-wrap"><table className="dx-table dx-images-table">
   <colgroup><col className="dx-col-select" /><col className="dx-col-image" /><col className="dx-col-created" /><col className="dx-col-size" /><col /><col className="dx-col-actions" /></colgroup>
   <thead><tr>
    <th><label><input ref={selectAll} type="checkbox" checked={allSelected} disabled={busy || !selectable.length} onChange={toggleAll} aria-label={t(allSelected ? "deselectAll" : "selectAll")} title={t(allSelected ? "deselectAll" : "selectAll") + " - " + t("selectVisible")} /></label></th>
    {["image", "created", "size", "usage", "actions"].map(key => <th key={key}>{t(key)}</th>)}
   </tr></thead>
   <tbody>{rows.map(({image, refs}) => <tr key={image.Id}>
    <td><input type="checkbox" aria-label={t("selectImage", {name: imageTags(image) || image.Id})} checked={selected.has(image.Id)} disabled={busy || !!refs.length} onChange={() => toggle(image.Id)} /></td>
    <td><span className="dx-image-tag" title={imageTags(image) || t("untagged")}>{imageTags(image) || t("untagged")}</span><small title={image.Id}>{image.Id}</small></td>
    <td title={new Date(image.Created).toLocaleString(i18n.language)}>{new Date(image.Created).toLocaleDateString(i18n.language)}<small>{new Date(image.Created).toLocaleTimeString(i18n.language)}</small></td>
    <td>{(image.Size / 1024 / 1024).toFixed(1)} MiB</td>
    <td>{refs.length ? <ul className="dx-image-usage">{refs.map(container => <li key={container.Id} title={containerName(container) + " (" + container.State.Status + ")"}>{containerName(container)} ({container.State.Status})</li>)}</ul> : t("unused")}</td>
    <td><button type="button" title={t("delete")} className="btn-secondary dx-delete" disabled={busy || !!refs.length} onClick={() => onDelete([image])}><FaTrash /></button></td>
   </tr>)}</tbody>
  </table></div>
  {!rows.length && <p>{t("emptyImages")}</p>}
 </>;
}
