# Desglose del monitor de recursos

Los indicadores CPU, RAM y disco muestran detalles al pasar el mouse o recibir
foco con Tab. El tooltip se renderiza fuera del terminal para evitar recortes,
permite desplazar listas largas y se cierra con Escape.

## Campos adicionales de SystemStats

| Campo | Contenido |
| --- | --- |
| cpuCores | ID de CPU logica y porcentaje, no necesariamente nucleos fisicos. |
| memReservedGB | Memoria wired en macOS; null en Linux, donde no se informa separadamente. |
| memBuffersGB | Linux: buffers, cache y memoria recuperable menos memoria compartida. macOS: paginas inactivas y especulativas. |
| memAvailableGB | MemAvailable en Linux con fallback; total menos usada estimada en macOS. |
| partitions | Dispositivo, punto de montaje, capacidad usada/total/disponible en GiB y porcentaje de df. |
| partitionsUpdatedAt | Unix timestamp de la ultima lectura valida de particiones. Cero si no hay lectura. |

Reservada forma parte de usada; buffer/cache y disponible pueden solaparse.
No se deben sumar esas categorias como si fueran segmentos independientes.
Linux no permite inferir memoria reservada de hardware desde MemTotal.
El resumen y el desglose de RAM Linux usan una unica lectura de /proc/meminfo.

El protocolo SSH conserva las nueve lineas de resumen y agrega una fila
`MEM reserved buffers available` en bytes y filas `CORE id percent`.
En Linux, todas las CPU se miden en el mismo intervalo usando /proc/stat.
En macOS, Python 3 y ctypes consultan host_processor_info de Mach; es una
muestra adicional de un segundo, por lo que puede diferir del total de top.
No instala dependencias ni compila helpers. Si Python falta o falla, la CPU total
sigue visible y el tooltip informa que el desglose no esta disponible.
La consulta opcional por CPU se limita a cinco segundos, el resumen a quince.

Las particiones se consultan con `df -Pk -l` como maximo una vez por minuto por
sesion SSH; se excluyen sistemas temporales conocidos y montajes de red.
La cache se libera con la sesion. Si falla una lectura, se conserva la anterior
con su fecha; la consulta tiene un limite de diez segundos.
Los limites cierran el canal del comando SSH, no la conexion del terminal.
El porcentaje es el informado por df, que contempla bloques reservados; puede
superar 100 en sistemas saturados. APFS puede compartir capacidad entre volumenes.

El frontend consulta nuevamente cinco segundos despues de terminar cada lectura;
no superpone consultas y descarta respuestas de sesiones que ya no estan activas.

## Validacion manual pendiente

- Linux: comparar CPU por CPU con un monitor local y RAM con /proc/meminfo.
- macOS Intel y Apple Silicon: verificar tamano de pagina, memoria wired y CPU por CPU con Python disponible.
- macOS sin Python: confirmar que el resumen sigue disponible.
- Particiones: confirmar puntos de montaje con espacios, porcentaje y refresco tras un minuto.
- UI: abrir tooltips con hover y Tab, recorrer listas largas y cerrar con Escape.

No se ejecutaron pruebas dinamicas ni builds durante esta implementacion.
