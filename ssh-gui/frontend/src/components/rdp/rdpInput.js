import { sendRDPInput } from "../../services/rdp";

const scans = {
  Escape: 0x01, Digit1: 0x02, Digit2: 0x03, Digit3: 0x04, Digit4: 0x05, Digit5: 0x06,
  Digit6: 0x07, Digit7: 0x08, Digit8: 0x09, Digit9: 0x0a, Digit0: 0x0b,
  Minus: 0x0c, Equal: 0x0d, Backspace: 0x0e, Tab: 0x0f,
  KeyQ: 0x10, KeyW: 0x11, KeyE: 0x12, KeyR: 0x13, KeyT: 0x14, KeyY: 0x15, KeyU: 0x16,
  KeyI: 0x17, KeyO: 0x18, KeyP: 0x19, BracketLeft: 0x1a, BracketRight: 0x1b,
  Enter: 0x1c, ControlLeft: 0x1d, KeyA: 0x1e, KeyS: 0x1f, KeyD: 0x20, KeyF: 0x21,
  KeyG: 0x22, KeyH: 0x23, KeyJ: 0x24, KeyK: 0x25, KeyL: 0x26, Semicolon: 0x27,
  Quote: 0x28, Backquote: 0x29, ShiftLeft: 0x2a, Backslash: 0x2b, KeyZ: 0x2c,
  KeyX: 0x2d, KeyC: 0x2e, KeyV: 0x2f, KeyB: 0x30, KeyN: 0x31, KeyM: 0x32,
  Comma: 0x33, Period: 0x34, Slash: 0x35, ShiftRight: 0x36, AltLeft: 0x38,
  Space: 0x39, CapsLock: 0x3a, F1: 0x3b, F2: 0x3c, F3: 0x3d, F4: 0x3e,
  F5: 0x3f, F6: 0x40, F7: 0x41, F8: 0x42, F9: 0x43, F10: 0x44, F11: 0x57, F12: 0x58,
  ControlRight: 0xe01d, AltRight: 0xe038, Home: 0xe047, ArrowUp: 0xe048,
  PageUp: 0xe049, ArrowLeft: 0xe04b, ArrowRight: 0xe04d, End: 0xe04f,
  ArrowDown: 0xe050, PageDown: 0xe051, Insert: 0xe052, Delete: 0xe053,
  MetaLeft: 0xe05b, MetaRight: 0xe05c, ContextMenu: 0xe05d,
};

export function bindRDPInput(canvas, sessionId, onFailure) {
  const pressed = new Map();
  const remove = [];
  let queue = [], animation, disposed = false, chain = Promise.resolve();
  const flush = () => {
    cancelAnimationFrame(animation); animation = undefined;
    if (!queue.length) return;
    const events = queue; queue = [];
    chain = chain.then(() => sendRDPInput(sessionId, events)).catch(error => { if (!disposed) onFailure(error); });
  };
  const push = event => {
    if (disposed) return;
    if (event.kind === "move" && queue.at(-1)?.kind === "move") queue[queue.length - 1] = event;
    else queue.push(event);
    if (queue.length >= 128) flush();
    else if (animation === undefined) animation = requestAnimationFrame(flush);
  };
  const release = () => { pressed.clear(); push({ kind: "release" }); flush(); };
  const listen = (target, name, callback, options) => {
    target.addEventListener(name, callback, options);
    remove.push(() => target.removeEventListener(name, callback, options));
  };
  const position = event => {
    const rect = canvas.getBoundingClientRect();
    const scale = Math.min(rect.width / canvas.width, rect.height / canvas.height);
    return {
      x: Math.max(0, Math.min(canvas.width - 1, Math.floor((event.clientX - rect.left - (rect.width - canvas.width * scale) / 2) / scale))),
      y: Math.max(0, Math.min(canvas.height - 1, Math.floor((event.clientY - rect.top - (rect.height - canvas.height * scale) / 2) / scale))),
    };
  };
  listen(canvas, "pointermove", event => push({ kind: "move", ...position(event) }));
  listen(canvas, "pointerdown", event => {
    if (event.button > 2) return;
    event.preventDefault(); canvas.focus(); canvas.setPointerCapture(event.pointerId);
    push({ kind: "button", button: event.button, down: true, ...position(event) });
  });
  listen(canvas, "pointerup", event => {
    if (event.button <= 2) push({ kind: "button", button: event.button, down: false, ...position(event) });
    if (canvas.hasPointerCapture(event.pointerId)) canvas.releasePointerCapture(event.pointerId);
  });
  listen(canvas, "pointercancel", release);
  listen(canvas, "contextmenu", event => event.preventDefault());
  listen(canvas, "wheel", event => {
    event.preventDefault();
    if (event.deltaY) push({ kind: "wheel", delta: event.deltaY > 0 ? -120 : 120, ...position(event) });
  }, { passive: false });
  listen(canvas, "keydown", event => {
    if (event.isComposing || event.key === "Dead") return;
    event.preventDefault();
    const key = event.code || event.key;
    let inputs = pressed.get(key);
    if (!inputs) {
      if (Array.from(event.key).length === 1 && !event.ctrlKey && !event.altKey && !event.metaKey) {
        inputs = event.key.split("").map(char => ({ kind: "unicode", code: char.charCodeAt(0) }));
      } else if (scans[event.code]) inputs = [{ kind: "key", code: scans[event.code] }];
      else return;
      pressed.set(key, inputs);
    }
    inputs.forEach(input => push({ ...input, down: true }));
  });
  listen(canvas, "keyup", event => {
    const key = event.code || event.key;
    const inputs = pressed.get(key);
    if (!inputs) return;
    event.preventDefault(); pressed.delete(key);
    inputs.forEach(input => push({ ...input, down: false }));
  });
  listen(canvas, "blur", release);
  listen(window, "blur", release);
  return {
    release,
    dispose() {
      release(); disposed = true; remove.forEach(callback => callback()); cancelAnimationFrame(animation);
    },
  };
}
