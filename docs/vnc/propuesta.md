# Conexiones VNC

Propuesta de feature. Estado: por implementar.

Permitir guardar y abrir escritorios remotos VNC desde el mismo panel de conexiones que SSH. La PoC debe completar el recorrido crear, guardar, conectar, interactuar y desconectar.

## Alcance de la PoC

- Selector SSH/VNC en Nueva conexion; SSH seleccionado inicialmente.
- Conexiones VNC guardadas en las carpetas existentes, con nombre y color.
- Sesion VNC en una pestana de escritorio remoto con pantalla, teclado y raton.
- Estados conectando, conectado, error y desconectado; cierre con liberacion de recursos.
- Conexion directa a un servidor VNC accesible, con autenticacion por contrasena.

Quedan fuera de esta PoC: transferencia de archivos, portapapeles compartido, audio, grabacion, reconexion automatica, tuneles SSH para VNC y compartir/sincronizar conexiones VNC. Estos ultimos flujos deben excluir VNC explicitamente hasta soportar el protocolo de extremo a extremo.

## Documentos

- [Panel e iconografia](panel.md).
- [Contrato tecnico y validacion manual](poc.md).
