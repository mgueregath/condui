# Flujo estandar de conexiones

El flujo existente de SSH es el estandar para todos los tipos de conexion, incluido VNC y cualquier protocolo futuro.

## Comportamiento comun

- Abrir usa el mismo listado, indicador de conexion y seleccion de sesion existente o nueva.
- Cada sesion ocupa una pestana con nombre, color y estado independiente del protocolo.
- Una desconexion conserva la pestana y ofrece Reconectar desde la pestana y la vista desconectada.
- Reconectar usa el mismo manejador, indicador y bloqueo de botones. Reemplaza la sesion en su posicion actual, limpia el estado desconectado y activa la pestana.
- Un fallo conserva la pestana desconectada, muestra el dialogo comun con el protocolo correspondiente y habilita un nuevo intento.
- Cerrar usa el mismo manejador y elimina la pestana despues de liberar la sesion.

## Diferencias por protocolo

La creacion de sesion se resuelve en connectSession. Cada protocolo aporta su transporte y los datos necesarios para su visor. SSH conserva los recursos de terminal y archivos; VNC crea su visor con la URL y credencial efimeras.

Los visores notifican la desconexion al mismo estado de pestanas. Los servicios de transporte liberan sus recursos al desconectarse y al cerrar. No crear mapas, estados, dialogos ni flujos de reconexion exclusivos de un protocolo. Las mejoras al ciclo de vida deben aplicarse al flujo compartido.

## Validacion manual

Comprobar apertura, desconexion, reconexion correcta, error y cierre tanto con SSH como con VNC. No ejecutar builds ni pruebas dinamicas automaticamente.
