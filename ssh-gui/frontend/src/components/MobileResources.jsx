import { useState, useEffect } from "react";
import { useTranslation } from "react-i18next";
import { FaArrowDown, FaArrowUp } from "react-icons/fa";
import { GetSystemStats } from "../../bindings/ssh-gui/app";
import { CPUDetails, MemoryDetails, DiskDetails } from "./ResourceDetails";
import { fmtGB, fmtBps, fmtUptime } from "./resourceFormat";

function barColor(pct) {
  if (pct > 85) return "var(--red)";
  if (pct > 65) return "var(--yellow)";
  return "var(--accent)";
}

function Card({ title, value, pct, children }) {
  const clamp = Math.min(100, Math.max(0, pct ?? 0));
  return (
    <section className="mobile-res-card">
      <header>
        <span className="mobile-res-title">{title}</span>
        <span className="mobile-res-value">{value}</span>
      </header>
      {pct != null && (
        <div className="mobile-res-bar">
          <div style={{ width: `${clamp}%`, background: barColor(clamp) }} />
        </div>
      )}
      {children && <div className="mobile-res-details">{children}</div>}
    </section>
  );
}

// Phone view of the stats that the desktop shows in the small bar under the
// terminal. Polls only while mounted, i.e. while this view is open.
export default function MobileResources({ sessionId, disconnected }) {
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
        // keep the last sample while the session recovers
      } finally {
        if (active) timer = setTimeout(fetchStats, 3000);
      }
    };
    fetchStats();
    return () => { active = false; clearTimeout(timer); };
  }, [sessionId, disconnected]);

  if (!stats) {
    return <div className="mobile-resources"><span className="res-loading">—</span></div>;
  }

  const memPct = stats.memTotalGB > 0 ? (stats.memUsedGB / stats.memTotalGB) * 100 : 0;
  const diskPct = stats.diskTotalGB > 0 ? (stats.diskUsedGB / stats.diskTotalGB) * 100 : 0;

  return (
    <div className="mobile-resources">
      <Card title="CPU" value={`${Math.round(stats.cpuPercent)}%`} pct={stats.cpuPercent}>
        <CPUDetails stats={stats} />
      </Card>
      <Card
        title="RAM"
        value={`${fmtGB(stats.memFreeGB)} ${t("resources.free")}`}
        pct={memPct}
      >
        <MemoryDetails stats={stats} />
      </Card>
      <Card
        title={t("resources.disk")}
        value={`${fmtGB(stats.diskFreeGB)} ${t("resources.free")}`}
        pct={diskPct}
      >
        <DiskDetails stats={stats} />
      </Card>
      <Card title={t("mobile.network")} value="">
        <div className="resource-detail-row">
          <span><FaArrowDown /> RX</span><span>{fmtBps(stats.netRxBps)}</span>
        </div>
        <div className="resource-detail-row">
          <span><FaArrowUp /> TX</span><span>{fmtBps(stats.netTxBps)}</span>
        </div>
      </Card>
      <Card title={t("mobile.diskIo")} value="">
        <div className="resource-detail-row"><span>R</span><span>{fmtBps(stats.diskReadBps)}</span></div>
        <div className="resource-detail-row"><span>W</span><span>{fmtBps(stats.diskWriteBps)}</span></div>
      </Card>
      <Card title={t("mobile.uptime")} value={fmtUptime(stats.uptimeSecs)} />
    </div>
  );
}
