import { useRef, useState } from "react";
import { responseError } from "../components/common/responseError";

export default function useResponseAction(onBusyChange) {
  const locked = useRef(false);
  const [busy, setBusy] = useState(false);
  const [response, setResponse] = useState(null);
  const run = async (action, pending, failure, secrets) => {
    if (locked.current) return;
    locked.current = true;
    setBusy(true);
    onBusyChange?.(true);
    setResponse({ severity: "pending", message: pending });
    try {
      await action();
      setResponse(null);
    } catch (error) {
      setResponse(responseError(error, failure, secrets));
    } finally {
      locked.current = false;
      setBusy(false);
      onBusyChange?.(false);
    }
  };
  return { busy, response, setResponse, run };
}
