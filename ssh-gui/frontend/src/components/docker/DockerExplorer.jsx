import { useCallback, useEffect, useRef, useState } from "react";
import { useTranslation } from "react-i18next";
import { FaDocker, FaSyncAlt } from "react-icons/fa";
import { getSnapshot, deleteImage, controlStack } from "./explorerApi";
import { imageTags } from "./explorerData";
import StacksView from "./StacksView";
import ImagesView from "./ImagesView";
import ConfirmAction from "./ConfirmAction";
import "./translations";
import "./DockerExplorer.css";

const errorText = error => error?.message || String(error);

export default function DockerExplorer({ sessionID }) {
 const { t } = useTranslation("dockerExplorer");
 const [snapshot, setSnapshot] = useState({Containers: [], Images: []});
 const [view, setView] = useState("stacks");
 const [search, setSearch] = useState("");
 const [busy, setBusy] = useState(false);
 const [status, setStatus] = useState({text: "", error: false});
 const [confirmation, setConfirmation] = useState(null);
 const locked = useRef(false);
 const refresh = useCallback(async () => {
  if (locked.current) return;
  locked.current = true; setBusy(true); setStatus({text: t("loading"), error: false});
  try {
   const data = await getSnapshot(sessionID);
   setSnapshot({Containers: data.Containers || [], Images: data.Images || []});
   setStatus({text: t("updated"), error: false});
  } catch (error) { setStatus({text: errorText(error), error: true}); }
  finally { locked.current = false; setBusy(false); }
 }, [sessionID, t]);
 useEffect(() => { refresh(); }, [refresh]);
 async function mutate(operation, success) {
  if (locked.current) return;
  locked.current = true; setBusy(true); setConfirmation(null);
  setStatus({text: t("working"), error: false});
  const errors = [];
  try { await operation(); } catch (error) { errors.push(errorText(error)); }
  try {
   const data = await getSnapshot(sessionID);
   setSnapshot({Containers: data.Containers || [], Images: data.Images || []});
  } catch (error) { errors.push(t("refreshFailed") + ": " + errorText(error)); }
  setStatus({text: errors.length ? errors.join(" ") : success, error: !!errors.length});
  locked.current = false; setBusy(false);
 }
 function stackAction(project, action) {
  if (busy) return;
  const run = () => mutate(() => controlStack(sessionID, project, action), t("actionDone"));
  if (action === "start") { run(); return; }
  setConfirmation({title: t(action), description: t("stackConfirm", {name: project}), run});
 }
 function requestDelete(images) {
  if (busy) return;
  setConfirmation({
   title: t("deleteTitle"), description: t("deleteDescription"),
   items: images.map(image => (imageTags(image) || t("untagged")) + " | " + image.Id),
   run: () => mutate(async () => {
    const errors = [];
    for (const image of images) {
     try { await deleteImage(sessionID, image.Id); }
     catch (error) { errors.push((imageTags(image) || image.Id) + ": " + errorText(error)); }
    }
    if (errors.length) throw new Error(errors.join(" "));
   }, t("deleted"))
  });
 }
 const query = search.trim().toLowerCase();
 return <main className="docker-explorer">
  <header className="dx-header"><h1><FaDocker aria-hidden="true" /> Docker</h1>
   <button type="button" className="btn-primary" disabled={busy} onClick={refresh}><FaSyncAlt aria-hidden="true" /> {t("refresh")}</button>
  </header>
  <nav className="dx-tabs" aria-label={t("views")}>
   {["stacks", "images"].map(tab => <button type="button" key={tab} aria-pressed={view === tab} onClick={() => setView(tab)}>{t(tab)}</button>)}
  </nav>
  <p className={"dx-status" + (status.error ? " error" : "")} role="status" aria-live="polite">{status.text}</p>
  <input className="modern-input dx-search" type="search" value={search} onChange={event => setSearch(event.target.value)} placeholder={t("search")} aria-label={t("search")} />
  <section hidden={view !== "stacks"} aria-label={t("stacks")}>
   <StacksView containers={snapshot.Containers} query={query} busy={busy} onAction={stackAction} onRefresh={refresh} />
  </section>
  <section hidden={view !== "images"} aria-label={t("images")}>
   <ImagesView images={snapshot.Images} containers={snapshot.Containers} query={query} busy={busy} onDelete={requestDelete} />
  </section>
  <ConfirmAction request={confirmation} onClose={() => setConfirmation(null)} onConfirm={() => confirmation?.run()} />
 </main>;
}
