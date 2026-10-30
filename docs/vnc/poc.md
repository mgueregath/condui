# PoC tecnica VNC

## Situacion actual

ConnectionModal.jsx expone campos y una prueba orientados a SSH. El modelo backend Connection no contiene protocolo; ConnectionNode.jsx muestra usuario@host y ofrece acciones SSH. Agregar solo el selector visual no habilita VNC.

## Contrato minimo

- Agregar protocol con valores ssh y vnc al modelo, persistencia y contrato frontend. Ausencia del campo en registros anteriores equivale a ssh.
- Conservar authType para autenticacion; no usarlo como identificador del protocolo.
- Persistir protocol en lectura, creacion, edicion y exportacion/importacion local si existen esos recorridos. Rechazar valores desconocidos.
- Aplicar validacion por protocolo tambien en backend. VNC no requiere usuario ni admite clave privada o jumpHostId en la PoC.
- Reutilizar el cifrado y la redaccion de contrasenas existentes; no incluir secretos en errores, logs ni URLs.
- Derivar apertura y prueba segun protocol. Mantener separado el servicio VNC del servicio de terminal SSH.
- Excluir conexiones VNC de sincronizacion y comparticion hasta extender sus contratos; conservar el comportamiento SSH existente.

## Prueba de viabilidad

Resolver primero la visualizacion interactiva dentro del WebView de Wails. Evaluar un cliente VNC web y un puente local WebSocket a TCP en Go; la seleccion e incorporacion de dependencias queda pendiente de autorizacion explicita, sin instalar paquetes en esta propuesta.

El puente debe escuchar solo en loopback, validar origen y usar una credencial efimera por sesion. Cerrar cliente, socket y conexion TCP al cancelar o cerrar la pestana. Evitar transmitir la contrasena en la URL. Un puente local no cifra el tramo TCP hacia el servidor VNC; limitar la PoC a una red de confianza o VPN y comunicar esa condicion junto al formulario VNC.

Comprobar negociacion RFB, autenticacion por contrasena, primer frame, actualizacion de pantalla y entrada de teclado/raton. Si el servidor exige un metodo no soportado, devolver un error claro. No anunciar soporte VNC completo hasta completar este recorrido.

## Puntos de cambio

| Area | Responsabilidad |
| --- | --- |
| backend/models/connection.go y backend/storage | Protocolo persistido y compatibilidad con SSH existente |
| app_connections.go y servicio VNC separado | Validacion, prueba y ciclo de vida de sesiones |
| frontend/src/components/connections/ConnectionModal.jsx | Selector y campos por protocolo |
| frontend/src/components/connections/ConnectionNode.jsx | Icono, direccion y acciones disponibles |
| Frontend de sesiones | Vista VNC separada de la terminal y despacho por protocolo |
| Traducciones, bindings y contratos de sincronizacion | Propagacion del tipo y exclusion VNC donde aun no se admite |

Centralizar la eleccion LuTerminal/LuMonitor en un componente pequeno de icono de protocolo compartido. Mantener formulario, visor y transporte con responsabilidades separadas.

## Aceptacion manual

1. Abrir y editar una conexion antigua sin protocol: sigue siendo SSH y conserva su comportamiento.
2. Crear VNC desde Nueva conexion; comprobar campos, puerto y cambio de protocolo.
3. Guardar y volver a abrir la aplicacion; comprobar protocolo, icono y datos persistidos.
4. Probar y abrir un servidor VNC compatible; visualizar escritorio y enviar teclado y raton.
5. Revisar host inaccesible, puerto invalido, contrasena incorrecta y autenticacion no soportada.
6. Cancelar durante la conexion, cerrar una sesion activa y comprobar liberacion de recursos.
7. Verificar que VNC no aparece como jump host ni ofrece herramientas SSH o comparticion/sincronizacion.
8. Comprobar navegacion por teclado, etiquetas accesibles e idiomas disponibles.

No ejecutar pruebas dinamicas ni builds automaticamente. La validacion funcional y los builds se realizan manualmente.
