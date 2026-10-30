# PoC VNC: implementacion y validacion pendiente

## Recorrido

Crear una conexion VNC, guardar, probar y abrir una pestana de escritorio. El visor noVNC permite teclado y raton. Las conexiones antiguas siguen usando SSH.

El backend autentica RFB antes de entregar el flujo al visor. La contrasena permanece en Go y reutiliza el cifrado del vault. El puente escucha en loopback, comprueba el origen exacto y requiere un token efimero enviado como primer mensaje, sin secretos en la URL. Cada sesion admite un solo visor y caduca si no se adjunta en 30 segundos.

Ver [autenticacion macOS](macos.md) para el uso de usuario y contrasena mediante Apple Remote Desktop.

## Limites

- RFB 3.3, 3.7 y 3.8 con autenticacion VNC por contrasena. Los saludos 3.6 de UltraVNC se negocian como 3.3; 3.889 de Apple, 4.0 de Intel AMT y 4.1/5.0 de RealVNC se negocian como 3.8, siguiendo noVNC. El metodo sin autenticacion solo se admite con contrasena vacia.
- La autenticacion VNC clasica utiliza los primeros ocho bytes de la contrasena.
- Servidores que exigen VeNCrypt, TLS u otros metodos devuelven un error de autenticacion no soportada.
- Usar una red de confianza o VPN: el tramo al servidor VNC no esta cifrado.
- Sin sincronizacion, comparticion, jump hosts ni herramientas SSH para VNC.
- Reconexion manual desde la pestana o la vista desconectada. Sin portapapeles compartido, transferencia de archivos ni reconexion automatica.
- El origen del WebView debe identificarse como Wails o localhost; origenes opacos se rechazan.

## Validacion pendiente

No se ejecutaron builds ni pruebas dinamicas. Seguir [la aceptacion manual](poc.md#aceptacion-manual), incluyendo primer frame, actualizaciones de pantalla, teclado, raton, cambio entre pestanas, cancelacion y cierre. Comprobar el WebView de cada plataforma antes de anunciar compatibilidad.

La dependencia autorizada es @novnc/novnc. Go reutiliza github.com/coder/websocket, ya presente. Las llamadas VNC estan en un adaptador separado de los bindings generados.

## Reconexion manual

Al desconectarse VNC, Reconectar abre una sesion nueva en la misma posicion de la pestana y libera el puente anterior. El boton se desactiva durante el intento. El flujo compartido se define en [el estandar de conexiones](../connection-lifecycle.md).

Validar manualmente una desconexion, reconexion correcta, error de reconexion y cierre durante el intento.
