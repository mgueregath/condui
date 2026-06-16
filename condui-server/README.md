# condui-server

Backend de sincronización para [Condui](https://condui.app). Provee autenticación, almacenamiento cifrado de conexiones y compartición entre usuarios.

- **E2E encryption** — el servidor nunca ve credenciales en texto plano
- **SQLite + WAL** — sin dependencias externas de base de datos
- **Tiers free / pro** configurables sin redesplegar
- **TLS automático** vía Caddy + Let's Encrypt

---

## Requisitos

| Herramienta | Versión mínima |
|---|---|
| Go | 1.23 |
| Docker + Docker Compose | 24+ (solo para el despliegue) |
| Dominio con DNS apuntando al servidor | — |

---

## Configuración

Copia el archivo de ejemplo y edita los valores:

```bash
cp .env.example .env
```

| Variable | Default | Descripción |
|---|---|---|
| `JWT_SECRET` | *(requerido)* | Clave secreta para firmar JWT. Genera una segura con `openssl rand -hex 32` |
| `PORT` | `8080` | Puerto HTTP interno |
| `DB_PATH` | `./data/condui.db` | Ruta al archivo SQLite |
| `JWT_EXPIRY_MINUTES` | `15` | Duración del access token |
| `REFRESH_EXPIRY_DAYS` | `30` | Duración del refresh token |
| `ALLOWED_ORIGINS` | `*` | Orígenes CORS permitidos (en prod: `https://tu-dominio.com`) |

> **JWT_SECRET es obligatorio en producción.** El valor por defecto del ejemplo no es seguro.

---

## Despliegue con Docker (recomendado)

### 1. Configura el dominio en `Caddyfile`

```
# Caddyfile
tu-dominio.com {
    reverse_proxy server:8080
    encode gzip
    header {
        Strict-Transport-Security "max-age=31536000; includeSubDomains"
        X-Content-Type-Options nosniff
        X-Frame-Options DENY
        -Server
    }
}
```

### 2. Crea el `.env`

```bash
JWT_SECRET=$(openssl rand -hex 32)
echo "JWT_SECRET=$JWT_SECRET" > .env
echo "ALLOWED_ORIGINS=https://tu-dominio.com" >> .env
```

### 3. Levanta los servicios

```bash
docker compose up -d
```

Caddy obtiene el certificado TLS automáticamente en el primer arranque.

### Comandos útiles

```bash
# Ver logs en tiempo real
docker compose logs -f server

# Reiniciar el servidor
docker compose restart server

# Actualizar a la última imagen
docker compose pull && docker compose up -d

# Detener todo
docker compose down

# Detener y borrar datos (¡irreversible!)
docker compose down -v
```

---

## Ejecución local (desarrollo)

```bash
# Instalar dependencias
go mod download

# Ejecutar con variables de entorno del .env
export $(cat .env | xargs)
go run .
```

El servidor queda disponible en `http://localhost:8080`.

---

## API

Base URL: `https://tu-dominio.com/api/v1`

### Autenticación

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/auth/register` | Crear cuenta |
| `POST` | `/auth/login` | Iniciar sesión → access + refresh token |
| `POST` | `/auth/refresh` | Renovar access token |
| `POST` | `/auth/logout` | Cerrar sesión (invalida refresh token) |
| `GET` | `/auth/me` | Datos del usuario autenticado |
| `PUT` | `/auth/identity` | Subir clave pública X25519 |

Las rutas autenticadas requieren el header:
```
Authorization: Bearer <access_token>
```

### Sincronización de blobs

| Método | Ruta | Descripción |
|---|---|---|
| `GET` | `/blobs` | Listar blobs del usuario |
| `GET` | `/blobs/:id` | Descargar blob cifrado |
| `POST` | `/blobs` | Crear blob |
| `PUT` | `/blobs/:id` | Actualizar blob |
| `DELETE` | `/blobs/:id` | Eliminar blob |

### Compartición (plan pro)

| Método | Ruta | Descripción |
|---|---|---|
| `POST` | `/shares` | Crear invitación de compartición |
| `GET` | `/shares/sent` | Ver invitaciones enviadas |
| `GET` | `/shares/received` | Ver invitaciones recibidas |
| `PUT` | `/shares/:id/accept` | Aceptar invitación |
| `DELETE` | `/shares/:id` | Revocar / rechazar |

---

## Tiers

Los límites se almacenan en la tabla `tier_limits` y se pueden cambiar sin redesplegar:

```bash
# Ajustar límite de blobs para el plan free (via sqlite3)
docker exec -it condui-server sqlite3 /data/condui.db \
  "UPDATE tier_limits SET max_count = 20 WHERE tier='free' AND resource='blobs';"
```

| Tier | Blobs | Dispositivos | Compartición |
|---|---|---|---|
| `free` | 10 | 2 | No |
| `pro` | Ilimitado | Ilimitado | Sí |

---

## Gestión de usuarios

Herramienta CLI incluida para administración directa de la base de datos.

### Ejecutar desde `condui-server/`

```bash
# Listar todos los usuarios
go run ./cmd/manage list

# Crear cuenta
go run ./cmd/manage create admin@tudominio.com ContraseñaSegura1

# Activar plan pro con fecha de caducidad
go run ./cmd/manage set-tier user@tudominio.com pro 2026-12-31

# Revertir a free (sin fecha = inmediato, sin caducidad futura)
go run ./cmd/manage set-tier user@tudominio.com free

# Cambiar contraseña
go run ./cmd/manage reset-password user@tudominio.com NuevaPass456

# Con base de datos en ruta personalizada (Docker, por ejemplo)
go run ./cmd/manage -db /data/condui.db list
```

### Ejecutar contra el servidor en producción (Docker)

```bash
# Copiar el binario al contenedor y ejecutar ahí
docker compose exec server sh -c "
  cd /app && go run ./cmd/manage list
"

# O directamente con sqlite3 si está disponible en el host
docker cp condui-server_server_1:/data/condui.db ./condui.db
go run ./cmd/manage -db ./condui.db list
```

> **Nota sobre caducidad:** cuando el `tier_expires_at` vence, el tier efectivo pasa a `free` automáticamente en el próximo login o renovación de token. No es necesario ningún cron job ni intervención manual.

### Flags del CLI

```
-db string   ruta al SQLite (default: $DB_PATH o ./data/condui.db)
```

---

## Backup

La base de datos es un único archivo SQLite. Para hacer backup:

```bash
# Backup con copia atómica (safe con WAL mode)
docker exec condui-server_server_1 sqlite3 /data/condui.db ".backup /data/condui.db.bak"

# Copiar al host
docker cp condui-server_server_1:/data/condui.db.bak ./backup-$(date +%Y%m%d).db
```

Para automatizarlo, añade una tarea cron en el servidor:

```bash
# /etc/cron.daily/condui-backup
#!/bin/bash
docker exec condui-server_server_1 \
  sqlite3 /data/condui.db ".backup /data/condui-$(date +%Y%m%d).db.bak"
```

---

## Variables de entorno — referencia completa

```bash
# .env
JWT_SECRET=<openssl rand -hex 32>
PORT=8080
DB_PATH=./data/condui.db
JWT_EXPIRY_MINUTES=15
REFRESH_EXPIRY_DAYS=30
ALLOWED_ORIGINS=https://sync.condui.app
```
