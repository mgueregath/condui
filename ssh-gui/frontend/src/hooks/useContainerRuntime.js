import { useEffect, useState } from "react";
import { Call } from "@wailsio/runtime";

export default function useContainerRuntime(sessionId) {
  const [result, setResult] = useState(null);
  useEffect(() => {
    let cancelled = false;
    setResult(null);
    if (sessionId) {
      Call.ByName("main.App.HasContainerRuntime", sessionId)
        .then((installed) => {
          if (!cancelled) setResult({ sessionId, installed });
        })
        .catch((error) => {
          if (!cancelled) console.error("Container runtime detection failed:", error);
        });
    }
    return () => { cancelled = true; };
  }, [sessionId]);
  return !!sessionId && result?.sessionId === sessionId && result.installed === true;
}
