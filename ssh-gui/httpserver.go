package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	neturl "net/url"
	"regexp"
	"strings"

	"github.com/wailsapp/wails/v3/pkg/application"

	"ssh-gui/backend/dbexplorer"
	"ssh-gui/backend/weblog"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func (a *App) startDockerLogServer() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", a.handleLogViewer)
	mux.HandleFunc("/stream", a.handleLogStream)
	mux.HandleFunc("/db", a.handleDbExplorer)
	mux.HandleFunc("/db-api/connect", a.handleDbApiConnect)
	mux.HandleFunc("/db-api/disconnect", a.handleDbApiDisconnect)
	mux.HandleFunc("/db-api/databases", a.handleDbApiDatabases)
	mux.HandleFunc("/db-api/schemas", a.handleDbApiSchemas)
	mux.HandleFunc("/db-api/tables", a.handleDbApiTables)
	mux.HandleFunc("/db-api/columns", a.handleDbApiColumns)
	mux.HandleFunc("/db-api/query", a.handleDbApiQuery)
	mux.HandleFunc("/db-api/credentials", a.handleDbApiCredentials)
	for port := 9091; port <= 9110; port++ {
		srv := &http.Server{Addr: fmt.Sprintf("127.0.0.1:%d", port), Handler: mux}
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			continue
		}
		a.logServerPort = port
		srv.Serve(ln)
		return
	}
}

func (a *App) handleLogViewer(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	session := r.URL.Query().Get("session")
	container := r.URL.Query().Get("container")
	html := weblog.RenderLogViewerHTML(name, session, container)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(html))
}

func (a *App) handleLogStream(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	containerID := r.URL.Query().Get("container")

	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		http.Error(w, "session not found", http.StatusNotFound)
		return
	}

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer cmdSession.Close()

	stdout, err := cmdSession.StdoutPipe()
	if err != nil {
		cmdSession.Close()
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if err := cmdSession.Start(fmt.Sprintf("docker logs -f --tail=500 %s 2>&1", containerID)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:9091")

	flusher, canFlush := w.(http.Flusher)
	ctx := r.Context()

	lineCleaner := strings.NewReplacer("\r", "")
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line := scanner.Text()
		// Remove CR characters
		line = lineCleaner.Replace(line)
		// Remove ANSI escape sequences
		line = ansiRegex.ReplaceAllString(line, "")

		fmt.Fprintf(w, "data: %s\n\n", strings.ReplaceAll(line, "\n", " "))
		if canFlush {
			flusher.Flush()
		}
	}
	fmt.Fprintf(w, "data: __END__\n\n")
	if canFlush {
		flusher.Flush()
	}
}

// OpenDockerLogWindow abre los logs del contenedor en una ventana del navegador del sistema
func (a *App) OpenDockerLogWindow(sessionID string, containerID string, containerName string) error {
	if a.logServerPort == 0 {
		return fmt.Errorf("servidor de logs no iniciado")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/?session=%s&container=%s&name=%s",
		a.logServerPort,
		neturl.QueryEscape(sessionID),
		neturl.QueryEscape(containerID),
		neturl.QueryEscape(containerName),
	)
	window := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
		URL:   url,
		Title: containerName,
	})
	window.Show()

	return nil
}

// OpenDbExplorerWindow abre el explorador de BD en una ventana del navegador del sistema
func (a *App) OpenDbExplorerWindow(sessionID, dbType string, port int) error {
	if a.logServerPort == 0 {
		return fmt.Errorf("servidor no iniciado")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/db?session=%s&type=%s&port=%d",
		a.logServerPort,
		neturl.QueryEscape(sessionID),
		neturl.QueryEscape(dbType),
		port,
	)
	window := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
		URL:       url,
		Title:     fmt.Sprintf("DB Explorer - %s:%d", dbType, port),
		Width:     1400,
		Height:    900,
		MinWidth:  1000,
		MinHeight: 700,
	})
	window.Show()

	return nil
}

// OpenSqliteExplorerWindow opens a remote SQLite file in the database explorer.
func (a *App) OpenSqliteExplorerWindow(sessionID, remotePath string) error {
	if a.logServerPort == 0 {
		return fmt.Errorf("servidor no iniciado")
	}
	url := fmt.Sprintf("http://127.0.0.1:%d/db?session=%s&type=sqlite&path=%s",
		a.logServerPort, neturl.QueryEscape(sessionID), neturl.QueryEscape(remotePath))
	window := application.Get().Window.NewWithOptions(application.WebviewWindowOptions{
		URL: url, Title: fmt.Sprintf("DB Explorer - %s", remotePath),
		Width: 1400, Height: 900, MinWidth: 1000, MinHeight: 700,
	})
	window.Show()
	return nil
}

func (a *App) handleDbExplorer(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	dbType := r.URL.Query().Get("type")
	portStr := r.URL.Query().Get("port")
	remotePath := r.URL.Query().Get("path")
	if portStr == "" {
		portStr = "0"
	}
	apiBase := fmt.Sprintf("http://127.0.0.1:%d", a.logServerPort)

	html := dbexplorer.RenderHTML(sessionID, dbType, portStr, remotePath, apiBase)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}
