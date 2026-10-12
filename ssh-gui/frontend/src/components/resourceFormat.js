export function fmtGB(gb) {
  if (gb >= 1000) return `${(gb / 1024).toFixed(1)}T`;
  if (gb >= 1) return `${gb.toFixed(1)}G`;
  return `${(gb * 1024).toFixed(0)}M`;
}

export function fmtBps(bps) {
  if (bps >= 1e9) return `${(bps / 1e9).toFixed(1)}G`;
  if (bps >= 1e6) return `${(bps / 1e6).toFixed(1)}M`;
  if (bps >= 1e3) return `${(bps / 1e3).toFixed(0)}K`;
  return `${Math.round(bps)}B`;
}

export function fmtUptime(secs) {
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
