import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";
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

function ComposeFile({ sessionID, path }) {
 const { t } = useTranslation("dockerExplorer");
 const [result, setResult] = useState(null);

 useEffect(() => {
  let active = true;
  ReadRemoteFile(sessionID, path).then(
   content => { if (active) setResult({ content }); },
   () => { if (active) setResult({ failed: true }); },
  );
  return () => { active = false; };
 }, [sessionID, path]);

 return <div className="dx-compose-file">
  <code>{path}</code>
  {!result || result.failed || !result.content ? (
   <span role="status">{t(!result ? "compose.loading" : result.failed ? "compose.failed" : "compose.empty")}</span>
  ) : <pre><code>{result.content}</code></pre>}
 </div>;
}

export default function StackComposeFiles({ sessionID, containers }) {
 const { t } = useTranslation("dockerExplorer");
 const paths = composePaths(containers);
 return <div className="dx-compose-files">
  {paths.length ? paths.map(path => <ComposeFile key={JSON.stringify([sessionID, path])} sessionID={sessionID} path={path} />)
   : <span role="status">{t("compose.missing")}</span>}
 </div>;
}
