# Prototipo Docker Explorer

Ventana independiente, abierta desde Docker mediante Stacks e Imagenes. La implementacion actual usa React y bindings Wails; ver [Docker Explorer en React](docker-explorer-react.md).

## Alcance
- Agrupaciones Compose y contenedores Swarm locales identificados por etiquetas.
- Contenedores sin agrupacion, servicios, imagenes, estados y puertos.
- Imagenes con tags, ID, fecha de creacion, tamaño y referencias de contenedores.
- Filtros por texto, sin uso y sin tag; orden de imagenes por antiguedad.
- Eliminacion individual o multiple confirmada, sin forzar y con comprobacion de referencias.
- Swarm no consulta servicios ni replicas de otros hosts.

## Validacion manual
1. Abrir desde una conexion SSH con Docker disponible.
2. Verificar agrupaciones activas, parciales y detenidas.
3. Comprobar que las imagenes referenciadas por contenedores detenidos no pueden eliminarse.
4. Filtrar y seleccionar imagenes sin uso; cancelar y confirmar la eliminacion.
5. Comprobar errores de permisos, desconexion y referencias concurrentes.
6. Verificar listas vacias y ausencia de Docker.

No se ejecutaron builds ni pruebas dinamicas.
