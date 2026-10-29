import React, { lazy, Suspense } from "react";
import { createRoot } from "react-dom/client";
import "./platform";
import "./style.css";
import "./components/Layout.css";
import "./mobile.css";
import "./i18n";
import "bootstrap/dist/css/bootstrap.min.css";

const App = lazy(() => import("./App"));
const DockerExplorer = lazy(() => import("./components/docker/DockerExplorer"));
const [route, query = ""] = window.location.hash.slice(1).split("?");
const sessionID = new URLSearchParams(query).get("session") || "";

createRoot(document.getElementById("root")).render(
 <React.StrictMode>
  <Suspense fallback={<div role="status">Condui...</div>}>
   {route === "/docker-explorer" ? <DockerExplorer sessionID={sessionID} /> : <App />}
  </Suspense>
 </React.StrictMode>
);
