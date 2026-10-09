import { FaPlay, FaStop, FaRedoAlt, FaSyncAlt } from "react-icons/fa";
import { useTranslation } from "react-i18next";
import { containerName, groupContainers } from "./explorerData";
import "./StacksView.css";

function Action({ title, icon: Icon, disabled, onClick, action }) {
 return <button type="button" className="dx-icon" data-action={action} title={title} aria-label={title} disabled={disabled} onClick={onClick}><Icon aria-hidden="true" /></button>;
}

function Ports({ container }) {
 const ports = Object.entries(container.NetworkSettings.Ports || {}).flatMap(([port, bindings]) =>
  (bindings || []).map(binding => {
   const ip = binding.HostIp || "";
   return (ip.includes(":") ? "[" + ip + "]" : ip) + ":" + binding.HostPort + " -> " + port;
  })
 );
 return ports.length ? <ul className="dx-ports">{ports.map((port, index) => <li key={port + index} title={port}>{port}</li>)}</ul> : "-";
}

export default function StacksView({ containers, query, busy, onAction, onRefresh }) {
 const { t } = useTranslation("dockerExplorer");
 const groups = groupContainers(containers).map(group => ({
  ...group,
  visible: group.containers.filter(container => [group.project, containerName(container), container.Config.Image].join(" ").toLowerCase().includes(query))
 })).filter(group => group.visible.length);
 return <>
  <p className="dx-note">{t("stackNote")}</p>
  {!groups.length && <p>{t("emptyContainers")}</p>}
  {groups.map(group => {
   const title = group.project || t("ungrouped");
   const active = group.containers.filter(container => container.State.Status === "running").length;
   const state = active === group.containers.length ? "running" : active ? "partial" : "stopped";
   return <article className="dx-stack" key={group.key}>
    <header className="dx-stack-heading">
     <div><h2>{title}</h2><div className="dx-summary">
      <span className={"dx-state " + state}>{t(state)}</span>
      <span>{group.type && group.type + " | "}{t("activeCount", {active, total: group.containers.length})}</span>
     </div></div>
     <div className="dx-stack-actions" role="group" aria-label={t("actionsFor", {name: title})}>
      {group.type === "Compose" && <>
       <Action action="start" title={t("start")} icon={FaPlay} disabled={busy || active === group.containers.length} onClick={() => onAction(group.project, "start")} />
       <Action action="stop" title={t("stop")} icon={FaStop} disabled={busy || !active} onClick={() => onAction(group.project, "stop")} />
       <Action action="restart" title={t("restart")} icon={FaRedoAlt} disabled={busy || !active} onClick={() => onAction(group.project, "restart")} />
      </>}
      <Action title={t("refresh")} icon={FaSyncAlt} disabled={busy} onClick={onRefresh} />
     </div>
    </header>
    <div className="dx-table-wrap"><table className="dx-table dx-stacks-table">
     <colgroup><col className="dx-stack-col-container" /><col className="dx-stack-col-service" /><col className="dx-stack-col-image" /><col className="dx-stack-col-state" /><col className="dx-stack-col-ports" /></colgroup>
     <thead><tr>{["container", "service", "image", "state", "ports"].map(key => <th key={key}>{t(key)}</th>)}</tr></thead>
     <tbody>{group.visible.map(container => {
      const labels = container.Config.Labels || {};
      const running = container.State.Status === "running";
      return <tr key={container.Id}>
       <td className={"dx-container-name" + (running ? " running" : "")}><span className="dx-stack-text" title={containerName(container)}>{containerName(container)}</span></td>
       <td><span className="dx-stack-text" title={labels["com.docker.compose.service"] || labels["com.docker.swarm.service.name"] || "-"}>{labels["com.docker.compose.service"] || labels["com.docker.swarm.service.name"] || "-"}</span></td>
       <td><span className="dx-stack-text" title={container.Config.Image}>{container.Config.Image}</span></td>
       <td><span className={"dx-state " + (running ? "running" : "stopped")}>{container.State.Status}</span></td>
       <td><Ports container={container} /></td>
      </tr>;
     })}</tbody>
    </table></div>
   </article>;
  })}
 </>;
}
