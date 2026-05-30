# ModernTerm - Roadmap de Implementación

## Objetivo

Construir una aplicación multiplataforma (Windows, Linux y macOS) para administración remota mediante SSH, SFTP y túneles SSH, inspirada en MobaXterm, Tabby y Termius.

---

# Estado Actual

## Completado

* Aplicación Wails funcional.
* Frontend React funcional.
* xterm.js integrado.
* Comunicación React → Wails → Go funcional.
* Comunicación Go → Wails → React funcional.
* Cliente SSH funcional.
* Shell interactiva remota funcional.
* Envío de comandos funcional.
* Recepción de salida funcional.

---

# Incremento 1 - Session Manager

## Objetivo

Permitir múltiples conexiones SSH simultáneas.

## Backend

Crear:

```text
backend/sessions
```

Implementar:

```go
type SSHSession struct
type SessionManager struct
```

Funciones:

* CreateSession()
* CloseSession()
* GetSession()
* ListSessions()
* SendInput()
* ResizeTerminal()

## Frontend

Crear modelo:

```ts
interface Session
```

Resultado esperado:

* Más de una conexión activa simultáneamente.

---

# Incremento 2 - Arquitectura de Tabs

## Objetivo

Permitir múltiples terminales abiertas.

## Funcionalidades

* Abrir pestaña.
* Cerrar pestaña.
* Cambiar pestaña.
* Mantener estado independiente.

## Resultado esperado

```text
s1 SSH
s2 SSH
s3 SSH
+
```

---

# Incremento 3 - Resize Real del Terminal

## Objetivo

Sincronizar tamaño del terminal remoto.

## Backend

Implementar:

```go
session.WindowChange()
```

## Frontend

Capturar:

```ts
term.rows
term.cols
```

## Resultado esperado

Funcionamiento correcto de:

* top
* htop
* vim
* nano
* less

---

# Incremento 4 - Layout Profesional

## Objetivo

Reemplazar interfaz de prueba.

## Layout

```text
Sidebar
Tabs
Panel principal
Panel inferior
```

## Librerías

* MUI
* react-resizable-panels

## Resultado esperado

Interfaz similar a VSCode.

---

# Incremento 5 - Gestión de Conexiones

## Objetivo

Guardar conexiones reutilizables.

## Funcionalidades

* Crear conexión.
* Editar conexión.
* Eliminar conexión.
* Duplicar conexión.

## Campos

* Nombre
* Host
* Puerto
* Usuario
* Método autenticación

---

# Incremento 6 - Persistencia SQLite

## Objetivo

Guardar configuración localmente.

## Tablas

### connections

```sql
id
name
host
port
username
auth_type
```

### folders

```sql
id
name
path
```

### tunnels

```sql
id
connection_id
...
```

---

# Incremento 7 - Almacenamiento Seguro de Credenciales

## Objetivo

No almacenar contraseñas en SQLite.

## Plataformas

### macOS

Keychain

### Windows

Credential Manager

### Linux

Secret Service

---

# Incremento 8 - Multiplexación SSH

## Objetivo

Múltiples terminales usando una sola conexión SSH.

## Resultado esperado

```text
Producción
├─ Shell 1
├─ Shell 2
└─ Shell 3
```

---

# Incremento 9 - Explorador SFTP

## Objetivo

Navegación remota.

## Funciones

* Listar carpetas.
* Listar archivos.
* Cambiar directorio.

## Resultado esperado

```text
/
├─ home
├─ var
├─ root
└─ opt
```

---

# Incremento 10 - Transferencia de Archivos

## Funciones

* Upload
* Download
* Rename
* Delete
* Mkdir

## Resultado esperado

Experiencia similar a MobaXterm.

---

# Incremento 11 - Drag & Drop

## Objetivo

Subir archivos arrastrando.

## Librerías

```text
react-dropzone
```

## Resultado esperado

Arrastrar archivo al explorador remoto.

---

# Incremento 12 - Cola de Transferencias

## Objetivo

Visualizar progreso.

## Panel

```text
Transferencias
```

## Información

* Archivo
* Progreso
* Velocidad
* Estado

---

# Incremento 13 - Túneles SSH

## Objetivo

Port Forwarding.

## Soportar

### Local

```bash
-L
```

### Remote

```bash
-R
```

### Dynamic

```bash
-D
```

## Resultado esperado

```text
localhost:3306 -> mysql:3306
localhost:9092 -> kafka:9092
```

---

# Incremento 14 - Gestión Visual de Túneles

## Funciones

* Crear
* Editar
* Activar
* Desactivar
* Eliminar

## Vista

```text
Túneles
```

---

# Incremento 15 - Favoritos

## Funciones

* Marcar conexión favorita.
* Agrupar conexiones.

## Resultado esperado

```text
Favoritos
Producción
QA
Laboratorio
```

---

# Incremento 16 - Historial de Comandos

## Funciones

* Historial por servidor.
* Búsqueda.

## Resultado esperado

Experiencia similar a terminal local.

---

# Incremento 17 - Buscador Global

## Objetivo

Buscar rápidamente.

## Alcance

* Conexiones
* Archivos
* Túneles

---

# Incremento 18 - Docker Explorer

## Objetivo

Administración visual Docker.

## Funciones

* Containers
* Images
* Networks
* Volumes

## Resultado esperado

```text
Docker
├─ Containers
├─ Images
├─ Volumes
```

---

# Incremento 19 - Editor Remoto

## Objetivo

Editar archivos directamente.

## Librería

Monaco Editor

## Flujo

```text
Download
Edit
Upload
```

---

# Incremento 20 - Búsqueda en Archivos

## Objetivo

Buscar texto remoto.

## Backend

grep/find

## Frontend

Panel de resultados.

---

# Incremento 21 - Monitor de Recursos

## Información

* CPU
* RAM
* Disco
* Load Average

---

# Incremento 22 - Gestor de Logs

## Funciones

* Tail
* Follow
* Filtro

## Comandos

```bash
tail -f
journalctl -f
```

---

# Incremento 23 - Temas

## Modos

* Dark
* Light

## Personalización

* Fuentes
* Tamaño terminal

---

# Incremento 24 - Perfiles Compartibles

## Objetivo

Exportar e importar configuraciones.

## Formato

```json
connections.json
```

---

# Incremento 25 - Release 1.0

## Requisitos

* SSH estable.
* SFTP estable.
* Túneles estables.
* Editor remoto.
* Docker Explorer.
* Persistencia.
* Credenciales seguras.

## Plataformas

* macOS Intel
* macOS ARM
* Windows x64
* Linux x64

## Resultado Final

Cliente profesional de administración remota comparable a MobaXterm, Tabby o Termius Desktop.
