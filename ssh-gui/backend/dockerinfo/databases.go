package dockerinfo

import (
	"fmt"
	"strconv"
	"strings"

	"golang.org/x/crypto/ssh"

	"ssh-gui/backend/models"
)

// --- Detección de bases de datos ---

var knownDBPorts = map[int]string{
	1433:  "SQL Server",
	1521:  "Oracle",
	3306:  "MySQL / MariaDB",
	5432:  "PostgreSQL",
	5433:  "PostgreSQL",
	5984:  "CouchDB",
	6379:  "Redis",
	6380:  "Redis",
	7474:  "Neo4j",
	8086:  "InfluxDB",
	8088:  "InfluxDB",
	9042:  "Cassandra",
	9200:  "Elasticsearch",
	9300:  "Elasticsearch",
	19042: "Cassandra",
	27017: "MongoDB",
	27018: "MongoDB",
	27019: "MongoDB",
}

var knownDBImages = []struct{ sub, name string }{
	{"postgres", "PostgreSQL"},
	{"timescale", "TimescaleDB"},
	{"mysql", "MySQL"},
	{"mariadb", "MariaDB"},
	{"mongo", "MongoDB"},
	{"redis", "Redis"},
	{"elasticsearch", "Elasticsearch"},
	{"opensearch", "OpenSearch"},
	{"cassandra", "Cassandra"},
	{"scylladb", "ScyllaDB"},
	{"couchdb", "CouchDB"},
	{"influxdb", "InfluxDB"},
	{"mssql", "SQL Server"},
	{"sqlserver", "SQL Server"},
	{"oracle", "Oracle"},
	{"neo4j", "Neo4j"},
	{"clickhouse", "ClickHouse"},
	{"cockroach", "CockroachDB"},
	{"rethinkdb", "RethinkDB"},
	{"arangodb", "ArangoDB"},
}

// GetDatabases discovers database services running on the remote host, both
// directly listening on known ports and exposed via Docker containers.
func GetDatabases(client *ssh.Client) ([]models.DatabaseInfo, error) {
	cmdSession, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	// SYS <port> <addr>  — puertos escuchando
	// DOC <name>|<image>|<ports>  — contenedores docker
	cmd := `ss -tlnp 2>/dev/null | awk 'NR>1{n=split($4,a,":");if(a[n]+0>0)print "SYS",a[n]+0,$4}'` +
		`; docker ps --format 'DOC {{.Names}}|{{.Image}}|{{.Ports}}' 2>/dev/null || true`

	output, _ := cmdSession.CombinedOutput(cmd)

	// Primera pasada: recoger puertos Docker mapeados para deduplicar
	dockerPorts := map[int]bool{}
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, "DOC ") {
			continue
		}
		parts := strings.SplitN(strings.TrimPrefix(line, "DOC "), "|", 3)
		if len(parts) < 3 {
			continue
		}
		for _, pm := range strings.Split(parts[2], ", ") {
			if idx := strings.Index(pm, "->"); idx > 0 {
				hostPart := pm[:idx]
				if ci := strings.LastIndex(hostPart, ":"); ci >= 0 {
					if p, err := strconv.Atoi(hostPart[ci+1:]); err == nil {
						dockerPorts[p] = true
					}
				}
			}
		}
	}

	var dbs []models.DatabaseInfo
	seen := map[string]bool{}

	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(line, "SYS ") {
			fields := strings.Fields(line)
			if len(fields) < 2 {
				continue
			}
			port, _ := strconv.Atoi(fields[1])
			name, ok := knownDBPorts[port]
			if !ok || dockerPorts[port] {
				continue // desconocido o ya cubierto por Docker
			}
			addr := ""
			if len(fields) > 2 {
				addr = fields[2]
			}
			key := fmt.Sprintf("sys:%d", port)
			if seen[key] {
				continue
			}
			seen[key] = true
			dbs = append(dbs, models.DatabaseInfo{
				Name:    name,
				Port:    port,
				Address: addr,
				Source:  "system",
			})

		} else if strings.HasPrefix(line, "DOC ") {
			parts := strings.SplitN(strings.TrimPrefix(line, "DOC "), "|", 3)
			if len(parts) < 2 {
				continue
			}
			containerName := parts[0]
			image := parts[1]
			portsStr := ""
			if len(parts) > 2 {
				portsStr = parts[2]
			}

			imageLower := strings.ToLower(image)
			dbName := ""
			for _, kd := range knownDBImages {
				if strings.Contains(imageLower, kd.sub) {
					dbName = kd.name
					break
				}
			}
			if dbName == "" {
				continue
			}

			// Puerto contenedor (interno, destino del ->)
			port := 0
			addr := ""
			for _, pm := range strings.Split(portsStr, ", ") {
				if idx := strings.Index(pm, "->"); idx > 0 {
					inner := strings.Split(pm[idx+2:], "/")[0]
					if p, err := strconv.Atoi(inner); err == nil && port == 0 {
						port = p
					}
					// dirección host
					hostPart := pm[:idx]
					if !strings.HasPrefix(hostPart, ":::") {
						addr = hostPart
					}
				}
			}

			key := fmt.Sprintf("docker:%s", containerName)
			if seen[key] {
				continue
			}
			seen[key] = true
			dbs = append(dbs, models.DatabaseInfo{
				Name:      dbName,
				Port:      port,
				Address:   addr,
				Source:    "docker",
				Container: containerName,
				Image:     image,
			})
		}
	}

	return dbs, nil
}
