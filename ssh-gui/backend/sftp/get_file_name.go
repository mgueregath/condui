package sftp

import (
	"path"
)

// GetFileName toma una ruta absoluta (ej. /var/log/nginx/access.log) 
// y retorna únicamente el nombre del archivo (ej. access.log).
func GetFileName(remotePath string) string {
	return path.Base(remotePath)
}