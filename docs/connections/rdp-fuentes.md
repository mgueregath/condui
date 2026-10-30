# Fuentes de la evaluación RDP

## gopher-rdp

- [Repositorio y API](https://github.com/bouncyball-git/gopher-rdp): funcionalidades, entrada, visores y eventos.
- [go.mod](https://raw.githubusercontent.com/bouncyball-git/gopher-rdp/main/go.mod): Go 1.25.6 y dependencia Ebiten.
- [Options](https://raw.githubusercontent.com/bouncyball-git/gopher-rdp/main/options.go): configuración, redirecciones, teclado y reconexión.
- [Client](https://raw.githubusercontent.com/bouncyball-git/gopher-rdp/main/client.go): Connect, dial con 10 segundos, upgradeTLS, OnDisconnect, Done y Close.
- [Licencia MIT](https://raw.githubusercontent.com/bouncyball-git/gopher-rdp/main/LICENSE).

## nakagami/grdp

- [Repositorio](https://github.com/nakagami/grdp).
- [go.mod](https://raw.githubusercontent.com/nakagami/grdp/master/go.mod): Go 1.26.3, struc y x/crypto.
- [API RdpClient](https://raw.githubusercontent.com/nakagami/grdp/master/grdp.go): dialer, bitmaps, entrada, negociación SSL/HYBRID y espera de Login.
- [SocketLayer](https://raw.githubusercontent.com/nakagami/grdp/master/core/socket.go): TLS y verificación de certificados desactivada.
- [Ejemplo web grdpwasm](https://github.com/nakagami/grdpwasm): arquitectura de navegador, no dependencia propuesta para Condui.
- [Licencia GPL-3.0](https://raw.githubusercontent.com/nakagami/grdp/master/LICENSE).

## adfnekc/grdp

- [API client documentada](https://pkg.go.dev/github.com/adfnekc/grdp/client): LoginContext, framebuffer, entrada, callbacks, errores y Setting.
- [Repositorio](https://github.com/adfnekc/grdp).
- [go.mod de master](https://raw.githubusercontent.com/adfnekc/grdp/master/go.mod): requisitos publicados en esa rama; contrastar con la revisión a instalar.
- [SocketLayer](https://raw.githubusercontent.com/adfnekc/grdp/master/core/socket.go): capa de transporte con configuración TLS.

La comparación no incluye pruebas de rendimiento, compilación ni validación de compatibilidad con servidores reales. No se atribuye fiabilidad por número de estrellas o por afirmaciones del README.
