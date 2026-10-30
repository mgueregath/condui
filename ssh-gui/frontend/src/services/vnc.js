import { Call } from "@wailsio/runtime";

async function invoke(method, ...args) {
  try {
    return await Call.ByName(`main.App.${method}`, ...args);
  } catch (error) {
    let message = typeof error === "string" ? error : error?.message;
    // Wails may wrap a Go error as a JSON string inside RuntimeError.message.
    if (typeof message === "string") {
      try {
        const detail = JSON.parse(message);
        if (typeof detail?.message === "string") message = detail.message;
      } catch { /* Already a plain error message. */ }
    }
    if (message) throw new Error(message);
    throw error;
  }
}

// Keep hand-written bindings separate from Wails generated files.
export const connectVNC = (id) => invoke("ConnectVNC", id, window.location.origin);
export const closeVNC = (id) => invoke("CloseVNC", id);
export const testVNC = (connection) => invoke("TestVNCConfig", connection);
