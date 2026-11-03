# Apertura de conexiones después del rebase

El rebase de `desktops` sobre `main` combinó el flujo multiprotocolo con el manejo de una única sesión en teléfonos de forma incompleta.

`handleOpenConnection` perdió el parámetro `protocol`, pero siguió utilizándolo al buscar pestañas. Esto provocaba un `ReferenceError` antes del bloque de conexión. Después del transporte también usaba `sessionId` y `connectResult`, reemplazados previamente por `session`.

Se restaura el parámetro con `defaultProtocol(c)`, se usa `session.id` y se incorporan todos los datos de sesión a la pestaña: tipo, ruta SSH y URL/token de VNC/noVNC. Se mantienen la selección explícita de protocolo y la política de una sesión por teléfono. Al sustituir la sesión en teléfonos, RDP y VNC/noVNC usan sus métodos de cierre correspondientes.

También se elimina la entrada duplicada de `@wailsio/runtime` en `package.json`, conservando `latest`, consistente con el lockfile actual.

La sincronización multiprotocolo permanece en el stash existente; no se aplica ni se altera. Se preservan las modificaciones previas de `go.mod` y `package-lock.json`.

Verificación estática del flujo de apertura, elección de protocolo, reconexión y cierre. `git diff --check`. No se ejecutan comandos `npm run` ni pruebas dinámicas. Queda pendiente comprobar manualmente la apertura SSH, VNC, noVNC y RDP, y el reemplazo de sesión en teléfono.
