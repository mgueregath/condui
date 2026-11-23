# Cambio de conexión activa

La barra de recursos muestra la última muestra válida de la sesión seleccionada.
La consulta se reanuda al volver a la sesión y se repite cinco segundos después
de cada respuesta. La vista móvil comparte esa muestra y la misma consulta.

Las comprobaciones de Docker y VirtualBox se guardan por sesión SSH, tanto si
encuentran el programa remoto como si confirman su ausencia. Una reconexión con
un nuevo identificador vuelve a realizar las comprobaciones.

El resaltado de las pestañas y las barras de recursos se actualizan sin
transiciones. La pestaña de conexión mantiene el mismo tamaño de texto al
cambiar de estado.
