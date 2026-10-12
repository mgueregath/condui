import { useTranslation } from "react-i18next";
import { fmtGB } from "./resourceFormat";

function Row({ label, children }) {
  return <div className="resource-detail-row"><span>{label}</span><span>{children}</span></div>;
}

export function CPUDetails({ stats }) {
  const { t } = useTranslation();
  return <>
    <strong>CPU</strong>
    <Row label={t("resources.total")}>{stats.cpuPercent.toFixed(1)}%</Row>
    {stats.cpuCores?.length ? stats.cpuCores.map((core) => (
      <Row key={core.id} label={t("resources.core", { id: core.id })}>{core.percent.toFixed(1)}%</Row>
    )) : <p className="resource-detail-note">{t("resources.coresUnavailable")}</p>}
  </>;
}

export function MemoryDetails({ stats }) {
  const { t } = useTranslation();
  return <>
    <strong>RAM</strong>
    <Row label={t("resources.total")}>{fmtGB(stats.memTotalGB)}</Row>
    <Row label={t("resources.used")}>{fmtGB(stats.memUsedGB)}</Row>
    <Row label={t("resources.reserved")}>{stats.memReservedGB == null ? t("resources.notReported") : fmtGB(stats.memReservedGB)}</Row>
    <Row label={t("resources.buffers")}>{fmtGB(stats.memBuffersGB ?? 0)}</Row>
    <Row label={t("resources.available")}>{fmtGB(stats.memAvailableGB ?? stats.memFreeGB)}</Row>
    <p className="resource-detail-note">{t("resources.memoryOverlap")}</p>
  </>;
}

export function DiskDetails({ stats }) {
  const { t } = useTranslation();
  const updated = stats.partitionsUpdatedAt;
  return <>
    <strong>{t("resources.partitions")}</strong>
    {stats.partitions?.length ? stats.partitions.map((partition) => (
      <div className="resource-partition" key={`${partition.device}:${partition.mountPoint}`}>
        <div className="resource-partition-path">{partition.mountPoint}</div>
        <div className="resource-detail-note resource-partition-path">{partition.device}</div>
        <Row label={t("resources.used")}>{fmtGB(partition.usedGB)} / {fmtGB(partition.totalGB)} ({partition.usedPercent.toFixed(0)}%)</Row>
        <Row label={t("resources.available")}>{fmtGB(partition.availableGB)}</Row>
      </div>
    )) : <p className="resource-detail-note">{t("resources.partitionsUnavailable")}</p>}
    <p className="resource-detail-note">{t("resources.partitionRefresh")}</p>
    {updated > 0 && <p className="resource-detail-note">{t("resources.updated", { time: new Date(updated * 1000).toLocaleTimeString() })}</p>}
  </>;
}
