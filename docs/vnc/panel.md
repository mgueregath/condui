# Panel e iconografia

## Nueva conexion

Agregar al inicio del formulario el campo Tipo de conexion, con dos opciones seleccionables y texto visible:

| Opcion | Icono de react-icons/lu | Descripcion |
| --- | --- | --- |
| SSH | LuTerminal | Terminal remota |
| VNC | LuMonitor | Escritorio remoto |

Usar botones de seleccion con fondo solido, estado seleccionado y navegacion por teclado. Sin gradientes ni dependencias nuevas. El texto identifica el protocolo sin depender del icono o del color.

## Campos

| Campo | SSH | VNC |
| --- | --- | --- |
| Nombre, host, carpeta y color | Visibles | Visibles |
| Puerto inicial | 22 | 5900 |
| Usuario | Visible | Oculto |
| Autenticacion | Contrasena o clave privada | Contrasena |
| Clave privada y passphrase | Segun autenticacion | Ocultas |
| Jump host | Comportamiento actual | Oculto |

Mostrar host y puerto separados; no interpretar sufijos de display VNC como :1 en la PoC. Validar nombre, host y puerto entero entre 1 y 65535. La autenticacion debe permitir enviar una contrasena vacia y mostrar el error devuelto por el servidor si no la acepta.

Al cambiar de tipo en una conexion nueva, conservar nombre, host, carpeta y color. Cambiar el puerto al predeterminado del destino solo si aun coincide con el predeterminado del origen. Limpiar credenciales y opciones especificas del protocolo anterior, y reiniciar el resultado de Probar conexion.

En Editar conexion, mostrar el tipo como dato de solo lectura para evitar convertir una conexion con sesiones o referencias existentes. Para otro protocolo se crea una conexion nueva.

## Listado y sesiones

- Mostrar LuTerminal o LuMonitor junto al nombre en el listado y la pestana de sesion, con etiqueta accesible SSH o VNC.
- Mantener el indicador de estado independiente del icono de protocolo.
- SSH conserva usuario@host; VNC muestra host:puerto.
- Probar conexion usa el protocolo seleccionado y verifica negociacion y autenticacion, no solo apertura del puerto.
- Abrir VNC dirige a la vista de escritorio remoto. No ofrecer terminal, SFTP, Docker, bases de datos ni acciones de comandos SSH para esa sesion.
- Excluir VNC de los candidatos a jump host y del menu Conectar via de la PoC.

## Traducciones

Reutilizar claves comunes y agregar claves anidadas cortas bajo connection: type.label, type.ssh, type.vnc, vnc.description y vnc.error. Mantener los idiomas existentes sincronizados. No insertar textos de interfaz directamente en JSX.
