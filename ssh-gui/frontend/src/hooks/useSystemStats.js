import { useEffect, useState } from "react";
import { GetSystemStats } from "../../bindings/ssh-gui/app";

export default function useSystemStats(sessionId) {
  const [samples, setSamples] = useState(() => new Map());

  useEffect(() => {
    if (!sessionId) return;
    let active = true;
    let timer;
    const fetchStats = async () => {
      try {
        const sample = await GetSystemStats(sessionId);
        if (active && sample) {
          setSamples((previous) => new Map(previous).set(sessionId, sample));
        }
      } catch {
        // Keep the last valid sample while the remote session recovers.
      } finally {
        if (active) timer = setTimeout(fetchStats, 5000);
      }
    };
    fetchStats();
    return () => { active = false; clearTimeout(timer); };
  }, [sessionId]);

  return samples.get(sessionId);
}
