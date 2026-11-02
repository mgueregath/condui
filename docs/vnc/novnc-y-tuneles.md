# noVNC por WebSocket y túneles en la conexión

La modal presenta SSH, VNC (TCP), noVNC y RDP. noVNC aparece inmediatamente debajo de VNC y puede habilitarse independientemente del acceso VNC directo.

## noVNC

Se puede configurar la URL HTTP/HTTPS del visor o una URL WebSocket exacta. Por ejemplo, se acepta `http://100.64.0.4:8006/` y se intenta conectar a `ws://100.64.0.4:8006/websockify` y a `ws://100.64.0.4:8006/`. HTTPS se convierte en WSS.

Para URLs HTTP, el parámetro `path` especifica el endpoint relativo en el mismo servidor y tiene prioridad exclusiva. Si no existe, se prueban rutas `websockify` relativas al directorio de la página, bajo la ruta introducida y en la raíz, y finalmente la ruta original; se eliminan duplicados y se conservan los parámetros de consulta. Una URL `ws://` o `wss://` se utiliza directamente. El presupuesto de apertura es de 10 segundos, con un máximo de 3 segundos por intento. Se detienen los intentos al aceptar WebSocket, al cancelar o ante HTTP 401/403.

La resolución es una aproximación para endpoints habituales: no descarga ni ejecuta el JavaScript del visor, ni interpreta sus opciones host/port/encrypt. Si el servidor usa una ruta diferente o configuración personalizada, indicar `?path=...` en la URL HTTP o el endpoint WebSocket exacto.
Esta implementación admite RFB binario sobre WebSocket para servidores sin credenciales. No incorpora inicio de sesión HTTP, cookies del navegador, autenticación de proxies ni subprotocolo base64. Las conexiones `wss://` comprueban el certificado TLS. Se puede probar, abrir, cancelar, cerrar y reconectar noVNC como los otros escritorios.

Go abre el WebSocket remoto y negocia RFB. El visor reutiliza el puente local autenticado con token efímero; la URL remota no se entrega al visor. Las URLs con usuario, contraseña o fragmento se rechazan. La configuración se guarda como `vnc.websocket` en el JSON existente, con su propio `enabled` y `url`, sin migración SQL. La habilitación de VNC directo sigue dependiendo de `vnc.enabled`.

## Túneles

Túneles aparece como una pestaña en el menú lateral de la modal, junto a SSH, VNC, noVNC y RDP. Su panel permite consultar, crear, editar y eliminar túneles sin abrir una sesión SSH. Muestra el puerto local, el destino y el estado real del listener. La navegación con flechas, Inicio y Fin incluye esta pestaña; cambiar de pestaña conserva los campos del túnel en edición.

- En conexiones existentes, las operaciones se aplican inmediatamente, incluso si después se cancela la edición de los demás campos. La interfaz lo indica explícitamente y confirma la eliminación.
- En conexiones nuevas, los túneles son un borrador y se guardan junto con la conexión. Cancelar descarta ese borrador.
- Guardar los demás campos de una conexión existente envía `tunnels: null`, preservando la lista vigente y evitando restaurar una copia antigua.
- La modal y el tab Túneles del panel inferior comparten `useTunnels` y la misma persistencia. El backend emite `tunnels-changed` al crear, editar, eliminar, activar/desactivar y cerrar la sesión SSH. Ambas vistas vuelven a leer la lista y su estado.
- Editar o eliminar un túnel activo cierra su listener anterior. Para activarlo de nuevo, abrir una sesión SSH y usar el panel inferior.
- Los puertos deben ser enteros entre 1 y 65535 y el host remoto no puede estar vacío. El editor de la modal también evita puertos locales duplicados dentro de la conexión.

## Verificación

Se realizó análisis sintáctico estático de los archivos JS/JSX/JSON modificados y compilación completa de la aplicación Go, además de los paquetes de VNC y modelos. El enlazador emitió advertencias sobre objetos compilados para macOS 26.0 con destino 11.0; la compilación terminó correctamente. No se ejecutaron `npm run` ni pruebas dinámicas.

Pendiente de validación manual: abrir un servidor noVNC real por `ws` y `wss`, recibir el escritorio y enviar teclado/ratón; probar rutas incorrectas, TLS inválido, cancelación y reconexión; crear/editar/eliminar desde ambas vistas; comprobar el estado al cerrar SSH; guardar y cancelar una conexión nueva con túneles.

La reubicación de Túneles se verificó con análisis sintáctico estático de JSX y CSS, sin ejecutar comandos `npm run` ni pruebas dinámicas.

La aceptación de URLs HTTP/HTTPS se verificó mediante compilación Go y análisis sintáctico estático del frontend. Sin pruebas dinámicas ni `npm run`; la resolución contra el servidor real queda pendiente.

## Estilo de los selectores

Todos los SELECT de la modal de conexión utilizan el mismo componente `Select` y la clase `modern-input`. La altura de 40 px del selector Carpeta se aplica también a los selectores de autenticación, jump host, origen de credenciales y opciones RDP. La regla queda limitada a `.connection-editor`. Verificación mediante análisis estático de CSS, sin pruebas dinámicas.

## Reutilizar datos SSH

Los campos Usuario y Contraseña de VNC y RDP muestran un checkbox «USAR DATO SSH» en la misma fila de su etiqueta, con la disposición del control de habilitación del protocolo. Sustituyen los selectores de origen. El usuario solo permite reutilización con SSH habilitado; la contraseña también exige autenticación SSH por contraseña. Marcar el checkbox conserva el dato propio y deshabilita su edición mientras se hereda el dato SSH. El control permanece disponible para desmarcarlo. `Input` admite un accesorio `labelAction` junto a su etiqueta, sin anidar etiquetas ni deshabilitar el checkbox junto con el campo heredado.

Verificación: análisis sintáctico estático de JSX, CSS y traducciones. Sin `npm run` ni pruebas dinámicas.
