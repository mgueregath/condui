# Condui adaptation

Upstream: github.com/bouncyball-git/gopher-rdp v1.0.8, MIT (see LICENSE).
Only root library, protocol and util Go sources are included. The upstream CLI,
GUI display code and tests are omitted; no build or tests were run here.

Changes: Options.TLSConfig; Client.connectCancel and Close cancellation;
ConnectContext with DialContext, connection deadline, TLS 1.2 verification,
NLA by default and no automatic credential/security downgrade. DisableTLS
explicitly selects Standard RDP Security, skipping TLS and CredSSP while
retaining cancellation and the standard security exchange. Upstream Connect
is retained for reference; Condui only calls ConnectContext.

Update by replacing these upstream sources with the chosen tagged version and
reapplying these isolated changes. The global Go module cache is untouched.

SendKeyboard also recognizes the E0 scancode prefix and sets KbdFlagsExtended,
so navigation keys and right-side modifiers reach the remote desktop correctly.

AllowUntrusted is an explicit opt-in to TLS without certificate verification.
It does not disable TLS or NLA and is never enabled as an automatic fallback.
