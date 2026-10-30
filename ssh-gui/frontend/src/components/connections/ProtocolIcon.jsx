import { LuMonitor, LuTerminal } from "react-icons/lu";

export default function ProtocolIcon({ protocol = "ssh" }) {
  const Icon = protocol === "vnc" ? LuMonitor : LuTerminal;
  return <Icon aria-label={protocol.toUpperCase()} role="img" />;
}
