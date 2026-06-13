import { useState, useEffect, useRef } from "react";
import { GetSystemStats } from "../../bindings/ssh-gui/app";;

function fmtGB(gb) {
  if (gb >= 1000) return `${(gb / 1024).toFixed(1)}T`;
  if (gb >= 1) return `${gb.toFixed(1)}G`;
  return `${(gb * 1024).toFixed(0)}M`;
}

function fmtBps(bps) {
  if (bps >= 1e9) return `${(bps / 1e9).toFixed(1)}G`;
  if (bps >= 1e6) return `${(bps / 1e6).toFixed(1)}M`;
  if (bps >= 1e3) return `${(bps / 1e3).toFixed(0)}K`;
  return `${Math.round(bps)}B`;
}

function fmtUptime(secs) {
  const s = Math.floor(secs);
  const mo = Math.floor(s / 2592000);
  const d = Math.floor((s % 2592000) / 86400);
  const h = Math.floor((s % 86400) / 3600);
  const m = Math.floor((s % 3600) / 60);
  if (mo > 0) return `${mo}mo${d}d`;
  if (d > 0) return `${d}d${h}h`;
  if (h > 0) return `${h}h${m}m`;
  return `${m}m`;
}

function barColor(pct) {
  if (pct > 85) return "var(--red)";
  if (pct > 65) return "var(--yellow)";
  return "var(--accent)";
}

function StatItem({ label, pct, text }) {
  const clamp = Math.min(100, Math.max(0, pct));
  return (
    <div className="res-item">
      <span className="res-label">{label}</span>
      <div className="res-bar">
        <div
          className="res-bar-fill"
          style={{ width: `${clamp}%`, background: barColor(clamp) }}
        />
      </div>
      <span className="res-value">{text}</span>
    </div>
  );
}

export default function ResourceBar({ sessionId, disconnected }) {
  const [stats, setStats] = useState(null);
  const timer = useRef(null);

  useEffect(() => {
    setStats(null);
    if (!sessionId || disconnected) return;

    const fetchStats = async () => {
      try {
        const s = await GetSystemStats(sessionId);
        setStats(s);
      } catch {
        /* ignore — session may still be starting */
      }
    };

    fetchStats();
    timer.current = setInterval(fetchStats, 5000);
    return () => clearInterval(timer.current);
  }, [sessionId, disconnected]);

  // Siempre renderizar la barra cuando hay sesión activa (aunque no haya stats aún)
  // para que FitAddon calcule bien la altura del terminal desde el primer render.
  if (!sessionId || disconnected) return null;

  const memPct =
    stats && stats.memTotalGB > 0
      ? (stats.memUsedGB / stats.memTotalGB) * 100
      : 0;
  const diskPct =
    stats && stats.diskTotalGB > 0
      ? (stats.diskUsedGB / stats.diskTotalGB) * 100
      : 0;

  return (
    <div className="resource-bar">
      {stats ? (
        <>
          <StatItem
            label="CPU"
            pct={stats.cpuPercent}
            text={`${Math.round(stats.cpuPercent)}%`}
          />
          <div className="res-sep" />
          <StatItem
            label="RAM"
            pct={memPct}
            text={`${fmtGB(stats.memFreeGB)} free`}
          />
          <div className="res-sep" />
          <StatItem
            label="Disk"
            pct={diskPct}
            text={`${fmtGB(stats.diskFreeGB)} free`}
          />
          <div className="res-sep" />
          <div className="res-item">
            <span className="res-label">UP</span>
            <span className="res-value res-uptime">
              {fmtUptime(stats.uptimeSecs)}
            </span>
          </div>
          <div className="res-sep" />
          <div className="res-item">
            <span className="res-label">NET</span>
            <span className="res-io-rx">↓ {fmtBps(stats.netRxBps)}</span>
            <span className="res-io-tx">↑ {fmtBps(stats.netTxBps)}</span>
          </div>
          <div className="res-sep" />
          <div className="res-item">
            <span className="res-label">DISK</span>
            <span className="res-io-rx">R {fmtBps(stats.diskReadBps)}</span>
            <span className="res-io-tx">W {fmtBps(stats.diskWriteBps)}</span>
          </div>
        </>
      ) : (
        <span className="res-loading">—</span>
      )}
    </div>
  );
}
