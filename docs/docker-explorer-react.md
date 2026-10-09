# Docker Explorer en React

## Integracion
- Una sola app React y el mismo compilado de Condui.
- Wails abre /#/docker-explorer?session=<id> en una ventana independiente.
- main.jsx selecciona DockerExplorer sin montar App ni sus efectos.
- Componentes separados para stacks, imagenes y confirmacion.
- Llamadas al servicio Go mediante bindings Wails por nombre.
- Se retiran la pagina plana y sus endpoints HTTP. Logs y explorador de bases de datos conservan su servidor.
- Estilos de Layout.css compartidos; reglas propias limitadas al explorador.
- Traducciones ES/EN mediante i18next.

## Validacion manual
1. Compilar manualmente y abrir desde Stacks e Imagenes.
2. Comprobar que la ventana principal conserva terminales y conexion SSH.
3. Verificar acciones Compose, estados y puertos publicados.
4. Probar filtros, seleccion parcial, seleccionar todo y deseleccionar todo.
5. Cancelar y confirmar eliminaciones; comprobar imagenes usadas por contenedores detenidos.
6. Probar desconexion SSH, cambios de idioma, recarga y listas vacias.

No se ejecutaron pruebas dinamicas ni builds.
