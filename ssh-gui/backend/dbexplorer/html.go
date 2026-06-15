package dbexplorer

import (
	_ "embed"
	"fmt"
	"strings"
)

//go:embed assets/db-explorer.html
var dbExplorerHTML string

// RenderHTML renders the DB Explorer page with connection placeholders substituted.
func RenderHTML(sessionID, dbType, portStr, apiBase string) string {
	return strings.NewReplacer(
		`PLACEHOLDER_SESSION`, fmt.Sprintf("%q", sessionID),
		`PLACEHOLDER_DBTYPE`, fmt.Sprintf("%q", dbType),
		`PLACEHOLDER_PORT`, portStr,
		`PLACEHOLDER_API`, fmt.Sprintf("%q", apiBase),
	).Replace(dbExplorerHTML)
}