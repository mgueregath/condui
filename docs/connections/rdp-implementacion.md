# Implementación RDP

Se incorpora gopher-rdp v1.0.8. RDP comparte Host/IP con SSH/VNC y mantiene puerto, usuario, contraseña, dominio y huella SHA-256 propios. Permite reutilizar credenciales SSH por campo; no claves privadas. Guarda secretos cifrados y los oculta al leer.

## Adaptación local

El replace en ssh-gui/go.mod apunta a third_party/gopher-rdp. Se conserva el núcleo con licencia MIT sin el visor externo. ConnectContext exige NLA y TLS 1.2, verifica certificados del sistema o una huella SHA-256 proporcionada explícitamente, y permite cancelar marcación y negociación. Tiempo límite: 30 segundos. No permite degradación automática. Los scancodes extendidos se adaptan para navegación y modificadores derechos. Detalles en ssh-gui/third_party/gopher-rdp/CONDUi-PATCH.md.

## Visor y alcance

Canvas integrado mediante eventos Wails, PNG completos hasta 10 veces por segundo y resolución seleccionable (1920x1080 por defecto). Incluye teclado, ratón y cursor. La prueba autentica y completa negociación; cierre, cancelación y reconexión manual siguen el flujo existente. No se habilitan audio, portapapeles, unidades, multimonitor, H.264 ni reconexión automática. IME queda pendiente. RDP queda excluido de compartir y sincronizar, igual que VNC.

## Validación manual pendiente

1. Guardar, reiniciar y editar accesos combinados; comprobar secretos ocultos.
2. Probar credenciales propias y reutilizadas, dominio y cambios sin guardar.
3. Revisar rechazo de contraseña incorrecta, host inaccesible y servidor sin NLA.
4. Revisar CA confiable, autofirmado con huella correcta, huella distinta y nombre incorrecto sin huella.
5. Cancelar durante negociación; cerrar y reconectar sin afectar SSH/VNC.
6. Revisar imagen, cursor, clics, rueda, flechas, modificadores, tildes y ñ.
7. Revisar traducciones español/inglés y navegación por teclado.

Solo se realizan revisiones estáticas. Builds y pruebas dinámicas se dejan para ejecución manual.

## Modo sin TLS

En el contexto RDP, Seguridad permite seleccionar RDP estándar sin TLS. La opción disableTLS se guarda en rdp_config y se aplica tanto a probar como a conectar y reconectar. No usa TLS, certificados ni NLA; conserva el intercambio de seguridad estándar de la biblioteca. No se realiza degradación automática. TLS con NLA sigue como valor predeterminado para conexiones existentes y nuevas. La huella guardada se conserva, pero solo se utiliza al volver a TLS.

Validar manualmente ambos modos, persistencia del selector y rechazo cuando el servidor exige TLS/NLA.

## TLS con certificado no verificado

Con TLS activo, Certificado del servidor permite elegir No verificar certificado. Se persiste allowUntrusted en rdp_config. Mantiene TLS y NLA, omite comprobaciones de CA, nombre y huella, y no requiere SHA-256. No verifica la identidad del servidor. La opción está desactivada por defecto y no se activa ante errores automáticamente. Una huella previa se conserva para volver al modo verificado. Probar, abrir y reconectar usan la misma configuración.

Validación manual pendiente: certificado autofirmado en ambos modos, persistencia y reconexión; al volver a verificar debe rechazarse una huella incorrecta.

## Captura de actualizaciones

El cliente registra OnBeginPaint junto con OnEndPaint. En gopher-rdp v1.0.8, beginPaint solo activa el lote si existe el callback de inicio; sin este registro endPaint nunca notifica y el visor permanece esperando el primer frame aunque la negociación termine correctamente. La captura sigue realizándose al finalizar cada lote, en la goroutine de lectura, antes de la siguiente escritura al framebuffer.

Validación manual pendiente: abrir escritorio, comprobar primer frame y actualizaciones posteriores, cambiar de pestaña y reconectar.

## Resolución

El contexto RDP permite seleccionar 1280x720, 1920x1080 o 2560x1440. Se guarda resolution en rdp_config y se aplica al probar, abrir y reconectar. Configuraciones sin valor usan 1920x1080. Go valida los presets tanto al guardar como al conectar. Cambiar la selección no modifica sesiones ya abiertas. El canvas conserva la proporción al ajustarse al visor.

Validación manual pendiente: guardar cada preset, reabrir la modal y reconectar comprobando las dimensiones del escritorio.
