import { useEffect, useRef, useState } from "react";
import { Call } from "@wailsio/runtime";

export default function useRemoteAvailability(sessionId, method) {
  const [results, setResults] = useState(() => new Map());
  const pending = useRef(new Set());
  useEffect(() => {
    if (!sessionId || results.has(sessionId) || pending.current.has(sessionId)) return;
    pending.current.add(sessionId);
    Call.ByName(method, sessionId)
      .then((installed) => {
        setResults((previous) => new Map(previous).set(sessionId, installed === true));
      })
      .catch((error) => {
        console.error(`Remote availability check failed (${method}):`, error);
      })
      .finally(() => pending.current.delete(sessionId));
  }, [sessionId, method, results]);
  return results.get(sessionId) === true;
}
