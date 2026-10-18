export function responseError(error, message, secrets = []) {
  const candidate = typeof error === "string" ? error : error?.message || error?.error?.message || error?.error;
  let details = typeof candidate === "string" ? candidate : "";
  for (const secret of secrets) {
    if (secret) details = details.split(secret).join("[redacted]");
  }
  details = details.replace(/((?:password|passphrase|token|authorization)\s*[:=]\s*)[^\s,;]+/gi, "$1[redacted]");
  return { severity: "error", message, details: details || undefined };
}
