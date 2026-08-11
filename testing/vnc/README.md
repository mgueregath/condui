# Prueba de conexion VNC

Pagina minima para verificar desde el navegador la conexion con un servidor VNC de macOS mediante `@novnc/novnc`.

Aunque un cliente como `vncviewer` puede conectarse directamente al puerto TCP 5900 del Mac, noVNC requiere un proxy WebSocket-to-TCP. Este directorio incluye un proxy local para realizar esa conversion.

## Uso

1. Instalar las dependencias con `npm install`.
2. En una terminal, configurar la IP del Mac e iniciar el proxy:

   ```powershell
   $env:VNC_TARGET_HOST="192.168.1.50"
   npm run proxy
   ```

3. En otra terminal, iniciar la pagina con `npm run dev`.
4. Abrir la URL local indicada por Vite.
5. Mantener `ws://127.0.0.1:6080`, ingresar el usuario y la contrasena usados por el cliente VNC que funciona, y pulsar `Conectar`.

El puerto VNC objetivo predeterminado es `5900`. Puede cambiarse antes de iniciar el proxy:

```powershell
$env:VNC_TARGET_PORT="5901"
```

El test valida el transporte y la autenticacion compatibles con noVNC. Algunas configuraciones de macOS pueden permitir clientes VNC nativos pero rechazar noVNC si no esta habilitada la opcion para que usuarios VNC controlen la pantalla mediante contrasena.
