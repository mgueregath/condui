# Autenticacion macOS

El formulario VNC incluye usuario opcional. Con usuario se solicita Apple Remote Desktop (security type 30); sin usuario se conserva VNC clasico por contrasena. No se cambia automaticamente de ARD a VNC clasico si el servidor rechaza ARD.

ARD reutiliza la contrasena cifrada del vault. El intercambio DH, MD5 y AES-128 ECB sigue el protocolo implementado por noVNC; no se agregan dependencias. Usuario y contrasena admiten hasta 63 bytes UTF-8 cada uno.

## Validacion manual pendiente

1. Editar la conexion VNC e ingresar usuario y contrasena de una cuenta autorizada en Compartir pantalla o Gestion remota de macOS.
2. Probar y abrir la conexion; verificar escritorio, teclado y raton.
3. Revisar usuario incorrecto, contrasena incorrecta y cuenta sin permisos.
4. Comprobar VNC clasico dejando el usuario vacio y usando la contrasena VNC del servidor.

No se ejecutan builds ni pruebas dinamicas automaticamente. ARD es un protocolo heredado; sigue siendo necesario usar una red de confianza o VPN.
