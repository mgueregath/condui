# Estandar de mensajes y respuestas

Este documento define el criterio comun de Condui para mensajes de interfaz y respuestas de operaciones. Aplica a formularios, modales, paneles y ventanas auxiliares. Es una especificacion; su existencia no implica que todos los flujos ya la implementen.

## Severidad

| Estado | Uso | Color semantico |
| --- | --- | --- |
| Error | La operacion fallo o no pudo completarse. | `--red` |
| Exito | La operacion termino correctamente. | `--green` |
| Advertencia | Existe un riesgo o un resultado parcial que requiere atencion. | `--yellow` |
| Informacion | Contexto que no representa un fallo ni un resultado exitoso. | `--accent` |
| En curso | La operacion sigue trabajando. | `--accent` |

La severidad depende del resultado, nunca del componente que lo muestra. Un error de Sync, SSH o TestConnection debe tener la misma apariencia. Un resultado parcial se presenta como advertencia e indica lo completado y lo pendiente. El texto debe expresar el estado sin depender solo del color.

## Presentacion visual

- Usar una caja comun con fondo solido `--bg-elevated`, borde de 1 px del color semantico, radio de 6 px y padding de 8 px 12 px.
- Mostrar el mensaje principal con `--text-primary`. Usar el color semantico en el indicador o titulo breve de estado. Si no existe titulo, aplicar el color semantico al mensaje.
- Mantener la misma caja para todas las severidades. Evitar fondos transparentes, colores hexadecimales locales y gradientes.
- El mensaje breve no necesita titulo adicional. No repetir el titulo de la modal dentro de la caja.
- El fondo de terminal se reserva para detalles tecnicos desplegables, con `--terminal-bg`, texto `--terminal-text` (contraste claro sobre el fondo oscuro) y formato monoespaciado. No mostrar la respuesta cruda del servidor como mensaje principal.
- El token de terminal existente es `--terminal-bg`; no introducir variantes como `--bg-terminal`.
- Los mensajes extensos deben ajustar lineas. Los detalles tecnicos pueden tener scroll dentro de su bloque.

## Ubicacion

- Formularios: mostrar el resultado dentro de la misma modal, antes de los botones. Las validaciones de campo aparecen junto al campo correspondiente.
- Operaciones de panel: mostrar el resultado junto a la accion o en el area de estado del panel.
- Abrir una modal de alerta solo si no existe un contexto visible adecuado o si el mensaje requiere reconocimiento explicito. Evitar apilar una alerta sobre un formulario que puede mostrar el error.
- Las confirmaciones destructivas usan modal con descripcion concreta y botones Cancelar y la accion explicita, por ejemplo Eliminar. Son previas a la operacion, no mensajes de resultado.
- No usar `window.alert`, `window.confirm` ni `window.prompt`. No agregar toasts como mecanismo alternativo en esta implementacion.

## Contenido y respuestas

- Mensaje principal breve, traducido y orientado al usuario: que ocurrio y, cuando sea posible, como continuar.
- Conservar el detalle tecnico en un campo separado para consulta. No exponer credenciales, tokens ni datos sensibles.
- Normalizar los errores de string, Error u objetos de respuesta mediante una utilidad comun. Evitar `[object Object]` y usar un mensaje traducido de respaldo si no hay detalle util.
- Modelo minimo de interfaz: `severity` y `message`; `details` y `progress` son opcionales. No requiere cambiar todos los contratos del backend.
- Traducciones anidadas, con claves cortas y claras. Los mensajes de la operacion pertenecen a su dominio; las etiquetas compartidas pertenecen a `common`.
- Registrar en consola puede complementar el aviso, pero nunca sustituir un error que afecta una accion del usuario.

## Comportamiento

- Al iniciar una operacion, limpiar el resultado anterior y mostrar un estado en curso. Bloquear envios duplicados.
- Al fallar, mantener el formulario y sus datos, mostrar el error y habilitar el reintento.
- Si el exito cierra la modal y actualiza visiblemente la lista, ese cambio basta como confirmacion. Evitar una alerta adicional de exito.
- Los errores y advertencias no desaparecen por temporizador. Se limpian al reintentar, cambiar los datos relacionados o cerrar el contexto.
- Una prueba de conexion debe evaluar los datos visibles del formulario. Cambiar esos datos invalida su resultado anterior.
- Durante operaciones que no admiten cancelacion, bloquear el cierre y explicar que siguen en curso. No ofrecer Cancelar si solo oculta la interfaz.
- Un porcentaje representa trabajo medido. Indicar su unidad cuando sea relevante: elementos, bytes, etc. Durante un conteo previo, mostrar indicador indeterminado y texto explicativo. No simular porcentajes ni tiempos restantes.
- Mostrar 100% solo tras confirmar el exito. Si falla, retirar el estado en curso y mostrar el error, incluyendo el resultado parcial cuando se conozca.

## Accesibilidad

- Errores nuevos: `role="alert"`. Informacion, exito y progreso: `role="status"` con anuncio moderado.
- Progreso determinado con semantica de barra y valor accesible. Progreso indeterminado sin porcentaje ficticio. Evitar anunciar cada incremento a lectores de pantalla.
- Modales con nombre accesible, foco inicial, foco contenido y restauracion del foco al cerrar. Escape respeta las restricciones de cierre de la operacion.

## Aplicacion a los casos actuales

| Caso | Criterio esperado |
| --- | --- |
| Error SSH | Caja de error comun; respuesta tecnica en detalles desplegables. |
| Error de Sync | Error rojo, nunca `vault-success`. |
| TestConnection | Caja de error comun dentro del cuerpo, antes de los botones; exito y carga conservan su severidad. |
| Guardar conexion o carpeta | Estado en curso y error visible dentro del formulario. |
| Error al abrir Docker | Mensaje de error comun en el contexto de origen, sin alerta nativa. |
| Borrado remoto | Indicador durante conteo, porcentaje por elementos durante borrado y error visible si falla. |
