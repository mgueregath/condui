// manage — herramienta CLI para administrar usuarios de condui-server.
//
// Uso:
//
//	go run ./cmd/manage [flags] <comando> [args...]
//
// Flags globales:
//
//	-db   ruta al archivo SQLite (default: $DB_PATH o ./data/condui.db)
//
// Comandos:
//
//	list                              — lista todos los usuarios
//	create  <email> <password>        — crea una cuenta nueva
//	set-tier <email> <tier> [expiry]  — cambia el tier; expiry en formato YYYY-MM-DD (opcional)
//	reset-password <email> <password> — cambia la contraseña
//
// Ejemplos:
//
//	go run ./cmd/manage list
//	go run ./cmd/manage create admin@ejemplo.com MiPass123
//	go run ./cmd/manage set-tier user@ejemplo.com pro 2026-12-31
//	go run ./cmd/manage set-tier user@ejemplo.com free
//	go run ./cmd/manage reset-password user@ejemplo.com NuevaPass456
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	dbPath := flag.String("db", envOr("DB_PATH", "./data/condui.db"), "ruta a la base de datos SQLite")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		usage()
		os.Exit(1)
	}

	db, err := sql.Open("sqlite", *dbPath+"?_foreign_keys=on")
	if err != nil {
		fatalf("no se pudo abrir la base de datos: %v", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		fatalf("no se puede conectar a %s: %v\n  ¿Existe el archivo? ¿El servidor corrió al menos una vez para crear las tablas?", *dbPath, err)
	}

	cmd := args[0]
	rest := args[1:]

	switch cmd {
	case "list":
		cmdList(db)
	case "create":
		if len(rest) < 2 {
			fatalf("uso: create <email> <password>")
		}
		cmdCreate(db, rest[0], rest[1])
	case "set-tier":
		if len(rest) < 2 {
			fatalf("uso: set-tier <email> <tier> [YYYY-MM-DD]")
		}
		var expiry *time.Time
		if len(rest) >= 3 {
			t, err := time.ParseInLocation("2006-01-02", rest[2], time.Local)
			if err != nil {
				fatalf("formato de fecha inválido (usa YYYY-MM-DD): %v", err)
			}
			// expira al final del día indicado
			eod := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.Local)
			expiry = &eod
		}
		cmdSetTier(db, rest[0], rest[1], expiry)
	case "reset-password":
		if len(rest) < 2 {
			fatalf("uso: reset-password <email> <password>")
		}
		cmdResetPassword(db, rest[0], rest[1])
	default:
		fatalf("comando desconocido: %q\nEjecuta sin argumentos para ver la ayuda.", cmd)
	}
}

// ─── comandos ────────────────────────────────────────────────────────────────

func cmdList(db *sql.DB) {
	rows, err := db.Query(
		`SELECT id, email, tier, tier_expires_at, created_at FROM users ORDER BY created_at`,
	)
	if err != nil {
		fatalf("error al listar usuarios: %v", err)
	}
	defer rows.Close()

	fmt.Printf("%-36s  %-30s  %-5s  %-20s  %s\n",
		"ID", "EMAIL", "TIER", "EXPIRA", "CREADO")
	fmt.Println(strings.Repeat("─", 110))

	count := 0
	for rows.Next() {
		var id, email, tier, createdAt string
		var expiresAt sql.NullString
		if err := rows.Scan(&id, &email, &tier, &expiresAt, &createdAt); err != nil {
			fatalf("error al leer fila: %v", err)
		}

		expStr := "—"
		expired := ""
		if expiresAt.Valid && expiresAt.String != "" {
			t, _ := time.Parse(time.RFC3339, expiresAt.String)
			expStr = t.Format("2006-01-02")
			if time.Now().After(t) {
				expired = " (VENCIDO→free)"
			}
		}

		fmt.Printf("%-36s  %-30s  %-5s  %-20s  %s%s\n",
			id, email, tier, expStr, createdAt[:10], expired)
		count++
	}
	if count == 0 {
		fmt.Println("  (sin usuarios)")
	}
}

func cmdCreate(db *sql.DB, email, password string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		fatalf("el email no puede estar vacío")
	}
	if len(password) < 8 {
		fatalf("la contraseña debe tener al menos 8 caracteres")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fatalf("error al hashear contraseña: %v", err)
	}

	id := uuid.NewString()
	_, err = db.Exec(
		`INSERT INTO users(id, email, password_hash) VALUES(?, ?, ?)`,
		id, email, string(hash),
	)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			fatalf("el email %q ya está registrado", email)
		}
		fatalf("error al crear usuario: %v", err)
	}

	fmt.Printf("✓ Usuario creado\n  id:    %s\n  email: %s\n  tier:  free\n", id, email)
}

func cmdSetTier(db *sql.DB, email, tier string, expiresAt *time.Time) {
	email = strings.ToLower(strings.TrimSpace(email))
	tier = strings.ToLower(strings.TrimSpace(tier))

	if tier != "free" && tier != "pro" {
		fatalf("tier inválido: %q (valores válidos: free, pro)", tier)
	}

	var userID string
	err := db.QueryRow(`SELECT id FROM users WHERE email = ?`, email).Scan(&userID)
	if err != nil {
		fatalf("usuario no encontrado: %q", email)
	}

	if expiresAt == nil {
		_, err = db.Exec(
			`UPDATE users SET tier = ?, tier_expires_at = NULL WHERE id = ?`, tier, userID)
	} else {
		_, err = db.Exec(
			`UPDATE users SET tier = ?, tier_expires_at = ? WHERE id = ?`,
			tier, expiresAt.UTC().Format(time.RFC3339), userID)
	}
	if err != nil {
		fatalf("error al actualizar tier: %v", err)
	}

	expMsg := "sin caducidad"
	if expiresAt != nil {
		expMsg = "expira el " + expiresAt.Format("2006-01-02")
	}
	fmt.Printf("✓ Tier actualizado\n  email: %s\n  tier:  %s\n  exp:   %s\n", email, tier, expMsg)
}

func cmdResetPassword(db *sql.DB, email, password string) {
	email = strings.ToLower(strings.TrimSpace(email))
	if len(password) < 8 {
		fatalf("la contraseña debe tener al menos 8 caracteres")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fatalf("error al hashear contraseña: %v", err)
	}

	res, err := db.Exec(
		`UPDATE users SET password_hash = ? WHERE email = ?`, string(hash), email)
	if err != nil {
		fatalf("error al actualizar contraseña: %v", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		fatalf("usuario no encontrado: %q", email)
	}
	fmt.Printf("✓ Contraseña actualizada para %s\n", email)
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func usage() {
	fmt.Print(`manage — administración de usuarios condui-server

Uso:
  go run ./cmd/manage [flags] <comando> [args...]

Flags:
  -db string   ruta al SQLite (default: $DB_PATH o ./data/condui.db)

Comandos:
  list                              lista todos los usuarios
  create  <email> <password>        crea una cuenta nueva (tier: free)
  set-tier <email> <tier> [expiry]  cambia el tier; expiry = YYYY-MM-DD (opcional)
  reset-password <email> <password> cambia la contraseña

Ejemplos:
  go run ./cmd/manage list
  go run ./cmd/manage create admin@midominio.com ContraseñaSegura1
  go run ./cmd/manage set-tier user@midominio.com pro 2026-12-31
  go run ./cmd/manage set-tier user@midominio.com free
  go run ./cmd/manage reset-password user@midominio.com NuevaPass456
`)
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", args...)
	os.Exit(1)
}
