import { Call } from "@wailsio/runtime";

async function invoke(method, ...args) {
  try { return await Call.ByName(`main.App.${method}`, ...args); }
  catch (error) {
    let message = typeof error === "string" ? error : error?.message;
    if (typeof message === "string") {
      try { const parsed = JSON.parse(message); if (typeof parsed?.message === "string") message = parsed.message; } catch { /* Plain message. */ }
    }
    if (message) throw new Error(message);
    throw error;
  }
}
export const connectRDP = id => invoke("ConnectRDP", id);
export const closeRDP = id => invoke("CloseRDP", id);
export const testRDP = connection => invoke("TestRDPConfig", connection);
export const getRDPFrame = id => invoke("GetRDPFrame", id);
export const sendRDPInput = (id, events) => invoke("SendRDPInput", id, events);
