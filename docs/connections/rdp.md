# Acceso RDP en la conexión existente

## Alcance

Agregar RDP junto a SSH y VNC en la modal lateral. Host/IP, nombre, carpeta y color siguen siendo comunes. RDP tiene habilitación, puerto inicial 3389, usuario y contraseña propios; permite reutilizar las credenciales SSH por campo. El usuario puede incluir el dominio con el formato DOMINIO\usuario.

La prueba valida una sesión RDP autenticada. La apertura crea una pestaña con escritorio, teclado y ratón; reconexión y cierre siguen el flujo compartido. El transporte y las credenciales permanecen en Go. El frontend solo presenta imagen y envía entrada mediante los mecanismos de Wails existentes.

## Implementación aplicada

Dependencia autorizada e incorporada: gopher-rdp v1.0.8 con adaptación local. Ver [implementación y validación manual](rdp-implementacion.md) y [evaluación previa](rdp-evaluacion.md).

## Compatibilidad

Persistir la configuración RDP en el mismo registro y usar el cifrado y redacción de contraseñas existentes. Mantener los registros SSH/VNC actuales. Omitir RDP de sincronización y compartición hasta soportar esos contratos.

## Aceptación manual

Comprobar guardado y reapertura, credenciales propias y reutilizadas, dominio, contraseña incorrecta, host inaccesible, primer frame, actualizaciones de pantalla, teclado, ratón, desconexión, reconexión y cierre. Revisar certificados, español e inglés.

No ejecutar builds ni pruebas dinámicas automáticamente.
