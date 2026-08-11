import RFB from "@novnc/novnc";
import "./style.css";

const form = document.querySelector("#connection-form");
const screen = document.querySelector("#screen");
const status = document.querySelector("#status");
const disconnectButton = document.querySelector("#disconnect");

let rfb = null;

function logError(context, error) {
  console.error(`[VNC] ${context}`, error);
}

function setStatus(message, state = "idle") {
  status.textContent = message;
  status.dataset.state = state;
}

function disconnect() {
  rfb?.disconnect();
  rfb = null;
  disconnectButton.disabled = true;
}

form.addEventListener("submit", (event) => {
  event.preventDefault();
  disconnect();

  const data = new FormData(form);
  const url = data.get("url").trim();
  const username = data.get("username").trim();
  const password = data.get("password");
  const credentials = { username, password };

  setStatus(`Conectando a ${url}...`);

  try {
    rfb = new RFB(screen, url, {
      credentials,
    });
    rfb.scaleViewport = true;
    rfb.resizeSession = true;

    rfb.addEventListener("connect", () => {
      setStatus("Conexion VNC establecida", "success");
      disconnectButton.disabled = false;
      rfb.focus();
    });

    rfb.addEventListener("credentialsrequired", (credentialsEvent) => {
      console.info("[VNC] Credenciales solicitadas:", credentialsEvent.detail?.types || []);
      rfb.sendCredentials(credentials);
    });

    rfb.addEventListener("securityfailure", (securityEvent) => {
      logError("Fallo de seguridad", securityEvent);
      setStatus(`Fallo de seguridad: ${securityEvent.detail?.reason || "sin detalle"}`, "error");
    });

    rfb.addEventListener("disconnect", (disconnectEvent) => {
      if (!disconnectEvent.detail?.clean) {
        logError("Desconexion inesperada", disconnectEvent);
      }
      const message = disconnectEvent.detail?.clean
        ? "Conexion VNC cerrada"
        : "La conexion VNC se cerro inesperadamente";
      setStatus(message, disconnectEvent.detail?.clean ? "idle" : "error");
      rfb = null;
      disconnectButton.disabled = true;
    });
  } catch (error) {
    logError("No se pudo crear el cliente noVNC", error);
    setStatus(`No se pudo iniciar la conexion: ${error.message}`, "error");
    rfb = null;
  }
});

disconnectButton.addEventListener("click", disconnect);

window.addEventListener("error", (event) => {
  logError("Error global", event.error || event);
});

window.addEventListener("unhandledrejection", (event) => {
  logError("Promesa rechazada sin manejar", event.reason);
});
