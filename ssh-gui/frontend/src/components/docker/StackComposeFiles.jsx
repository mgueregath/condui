import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
import { Call } from "@wailsio/runtime";
import { ReadRemoteFile } from "../../../bindings/ssh-gui/app";

function composePaths(containers) {
 const paths = new Set();
 for (const { Config } of containers) {
  const labels = Config.Labels || {};
  const files = labels["com.docker.compose.project.config_files"] || "";
  const directory = labels["com.docker.compose.project.working_dir"];
  for (const entry of files.split(",")) {
   const path = entry.trim();
   if (!path || path === "-") continue;
   if (path.startsWith("/")) paths.add(path);
   else if (directory?.startsWith("/")) paths.add(directory.replace(/\/$/, "") + "/" + path);
  }
 }
 return [...paths];
}

function ComposeFile({ sessionID, path, portainerServer, unavailable }) {
 const { t } = useTranslation("dockerExplorer");
 const [result, setResult] = useState(null);

 useEffect(() => {
  let active = true;
  async function read() {
   try {
    const content = await ReadRemoteFile(sessionID, path);
    if (active) setResult({ content });
   } catch {
    if (!active) return;
    if (portainerServer && /^\/data\/compose\/\d+\//.test(path)) {
     try {
      const file = await Call.ByName("main.App.ReadDockerExplorerPortainerComposeFile", sessionID, path);
      if (active) setResult({ content: file.Content, location: file.Location });
      return;
     } catch {
      // Show the file as unavailable if Portainer cannot supply it.
     }
    }
    if (active) setResult({ failed: true });
   }
  }
  read();
  return () => { active = false; };
 }, [sessionID, path, portainerServer]);

 return <div className="dx-compose-file">
  <code>{result?.location || path}</code>
  {!result || result.failed || !result.content ? (
   <span role="status">{t(!result ? "compose.loading" : result.failed ? (unavailable || "compose.failed") : "compose.empty")}</span>
  ) : <pre><code>{result.content}</code></pre>}
 </div>;
}

export default function StackComposeFiles({ sessionID, containers, portainerServer, portainerAgent }) {
 const { t } = useTranslation("dockerExplorer");
 const paths = composePaths(containers);
 const unavailable = portainerAgent && (portainerServer ? "compose.local" : "compose.remote");
 return <div className="dx-compose-files">
  <p className="dx-note">{t("compose.readOnly")}</p>
  {paths.length ? paths.map(path => <ComposeFile key={JSON.stringify([sessionID, path])} sessionID={sessionID} path={path} portainerServer={portainerServer} unavailable={unavailable} />)
   : <span role="status">{t(unavailable || "compose.missing")}</span>}
 </div>;
}
