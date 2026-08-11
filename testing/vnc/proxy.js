import net from "node:net";
import { WebSocket, WebSocketServer } from "ws";

const targetHost = procees.env.VNC_TARGET_HOST;
const targetPort = Number(process.env.VNC_TARGET_PORT || 5900);
const listenPort = Number(process.env.VNC_PROXY_PORT || 6080);

if (!targetHost) {
  console.error("Falta definir VNC_TARGET_HOST con la IP o hostname del Mac.");
  process.exit(1);
}

const server = new WebSocketServer({ port: listenPort });

server.on("connection", (webSocket) => {
  console.log(`Conectando con ${targetHost}:${targetPort}`);
  const tcpSocket = net.createConnection({ host: targetHost, port: targetPort });
  let webSocketBytes = 0;
  let tcpBytes = 0;

  tcpSocket.on("connect", () => {
    console.log("Proxy conectado al servidor VNC del Mac.");
  });

  webSocket.on("message", (data) => {
    webSocketBytes += data.length;
    if (webSocketBytes === data.length) {
      console.log(`Primer envio WebSocket -> VNC: ${data.length} bytes`);
    }
    if (!tcpSocket.destroyed) tcpSocket.write(data);
  });

  tcpSocket.on("data", (data) => {
    tcpBytes += data.length;
    if (tcpBytes === data.length) {
      console.log(`Primer envio VNC -> WebSocket: ${data.length} bytes`);
    }
    if (webSocket.readyState === WebSocket.OPEN) webSocket.send(data);
  });

  tcpSocket.on("error", (error) => {
    console.error("Error TCP VNC:", error);
    webSocket.close(1011, "No fue posible conectar con el servidor VNC");
  });

  webSocket.on("close", (code, reason) => {
    console.log("WebSocket cerrado:", {
      code,
      reason: reason.toString(),
      webSocketBytes,
      tcpBytes,
    });
    tcpSocket.destroy();
  });
  webSocket.on("error", (error) => {
    console.error("Error WebSocket:", error);
    tcpSocket.destroy();
  });
  tcpSocket.on("close", (hadError) => {
    console.log("Conexion TCP VNC cerrada:", { hadError });
    webSocket.close();
  });
});

server.on("listening", () => {
  console.log(`Proxy disponible en ws://127.0.0.1:${listenPort}`);
});

server.on("error", (error) => {
  console.error("Error del servidor proxy:", error);
});

process.on("uncaughtException", (error) => {
  console.error("Excepcion no controlada en el proxy:", error);
});

process.on("unhandledRejection", (reason) => {
  console.error("Promesa rechazada sin manejar en el proxy:", reason);
});
