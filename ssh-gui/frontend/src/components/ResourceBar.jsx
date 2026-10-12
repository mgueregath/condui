import { useState, useEffect } from "react";
import { GetSystemStats } from "../../bindings/ssh-gui/app";
import { useTranslation } from "react-i18next";
import { FaArrowDown, FaArrowUp } from "react-icons/fa";
import ResourceTooltip from "./ResourceTooltip";
import { CPUDetails, MemoryDetails, DiskDetails } from "./ResourceDetails";
import { fmtGB, fmtBps, fmtUptime } from "./resourceFormat";

function barColor(pct) {
  if (pct > 85) return "var(--red)";
  if (pct > 65) return "var(--yellow)";
  return "var(--accent)";
}

function StatItem({ label, pct, text, details }) {
  const clamp = Math.min(100, Math.max(0, pct));
  return (
    <ResourceTooltip content={details}>
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
    </ResourceTooltip>
  );
}

export default function ResourceBar({ sessionId, disconnected }) {
  const { t } = useTranslation();
  const [stats, setStats] = useState(null);

  useEffect(() => {
    setStats(null);
    if (!sessionId || disconnected) return;
    let active = true;
    let timer;
    const fetchStats = async () => {
      try {
        const result = await GetSystemStats(sessionId);
        if (active) setStats(result);
      } catch {
        // Keep the last valid sample while the remote session recovers.
      } finally {
        if (active) timer = setTimeout(fetchStats, 5000);
      }
    };
    fetchStats();
    return () => { active = false; clearTimeout(timer); };
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
            details={<CPUDetails stats={stats} />}
            label="CPU"
            pct={stats.cpuPercent}
            text={`${Math.round(stats.cpuPercent)}%`}
          />
          <div className="res-sep" />
          <StatItem
            details={<MemoryDetails stats={stats} />}
            label="RAM"
            pct={memPct}
            text={`${fmtGB(stats.memFreeGB)} ${t("resources.free")}`}
          />
          <div className="res-sep" />
          <StatItem
            details={<DiskDetails stats={stats} />}
            label={t("resources.disk")}
            pct={diskPct}
            text={`${fmtGB(stats.diskFreeGB)} ${t("resources.free")}`}
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
            <span className="res-io-rx"><FaArrowDown /> {fmtBps(stats.netRxBps)}</span>
            <span className="res-io-tx"><FaArrowUp /> {fmtBps(stats.netTxBps)}</span>
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
