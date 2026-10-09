# Volumenes Docker

## Alcance
- Pestaña React en la ventana Docker existente.
- Nombre, driver, scope, fecha de creacion, punto de montaje y referencias.
- Busqueda por nombre, driver y ruta; filtro sin uso.
- Seleccion individual, multiple y seleccionar/deseleccionar visibles.
- Eliminacion confirmada con advertencia de perdida de datos, sin forzar.
- Contenedores activos y detenidos protegen sus volumenes.
- Solo volumenes Docker; los bind mounts no se gestionan como volumenes.

## Validacion manual
1. Compilar manualmente y abrir Volumenes.
2. Comprobar listado vacio, volumenes locales y fechas ausentes.
3. Verificar referencias de contenedores activos y detenidos.
4. Filtrar, seleccionar todo, deseleccionar y cambiar de pestaña.
5. Cancelar y confirmar eliminacion sobre un volumen de prueba sin datos necesarios.
6. Comprobar errores por referencias concurrentes o desconexion.

No se ejecutaron pruebas dinamicas ni builds.
