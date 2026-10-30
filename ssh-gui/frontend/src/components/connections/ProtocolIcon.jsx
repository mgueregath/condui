import { LuMonitor, LuTerminal, LuMonitorCog } from "react-icons/lu";

export default function ProtocolIcon({ protocol = "ssh" }) {
  const Icon = protocol === "rdp" ? LuMonitorCog : protocol === "vnc" ? LuMonitor : LuTerminal;
  return <Icon aria-label={protocol.toUpperCase()} role="img" />;
}
