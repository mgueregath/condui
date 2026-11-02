import { useCallback, useEffect, useRef, useState } from "react";
import { Events } from "@wailsio/runtime";
import { GetTunnels } from "../../bindings/ssh-gui/app";

// Both connection settings and the bottom panel read the same persisted list.
export default function useTunnels(connectionId) {
  const [tunnels, setTunnels] = useState([]);
  const [error, setError] = useState(null);
  const request = useRef(0);
  const reload = useCallback(async () => {
    const current = ++request.current;
    if (!connectionId) { setTunnels([]); setError(null); return; }
    try {
      const result = await GetTunnels(connectionId);
      if (current === request.current) { setTunnels(result || []); setError(null); }
    } catch (error) {
      if (current === request.current) setError(error);
    }
  }, [connectionId]);
  useEffect(() => {
    setTunnels([]);
    const off = Events.On("tunnels-changed", reload);
    void reload();
    return () => { ++request.current; off?.(); };
  }, [reload]);
  return { tunnels, error, reload };
}
