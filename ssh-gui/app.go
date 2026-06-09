package main

import (
	"fmt"
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"time"
	"io"
	"os"
	"net"
	"net/http"
	neturl "net/url"
	"strings"
	"sync"
	"encoding/base64"
    "path/filepath"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	mysql "github.com/go-sql-driver/mysql"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-gui/backend/sessions"
    "ssh-gui/backend/storage"
	"ssh-gui/backend/models"

	sftpservice "ssh-gui/backend/sftp"
    "ssh-gui/backend/transfers"
)

type App struct {
	ctx context.Context

	sessionManager *sessions.SessionManager

	database *storage.Database

	transferManager *transfers.Manager

	dockerLogMu      sync.Mutex
	dockerLogSessions map[string]*ssh.Session

	logServerPort int

	dbConnMu      sync.Mutex
	dbConns       map[string]*DbConn
}

// DbConn holds an active SSH-tunneled database connection.
type DbConn struct {
	ID        string
	SessionID string
	DbType    string // "postgresql" | "mysql"
	Port      int
	User      string
	Password  string
	sshClient *ssh.Client
	// PostgreSQL: one *pgxpool.Pool per database (safe for concurrent access)
	pgPools map[string]*pgxpool.Pool
	pgMu    sync.Mutex
	// MySQL: one *sql.DB per database name
	myDBs map[string]*sql.DB
	myMu  sync.Mutex
}

// mysqlSSHClients maps DbConn.ID -> *ssh.Client for the custom condui-ssh dialer.
var mysqlSSHClients sync.Map

func init() {
	mysql.RegisterDialContext("condui-ssh", func(ctx context.Context, addr string) (net.Conn, error) {
		// addr is "<connID>@<host:port>"
		at := strings.LastIndex(addr, "@")
		if at < 0 {
			return nil, fmt.Errorf("invalid condui-ssh addr: %s", addr)
		}
		connID, realAddr := addr[:at], addr[at+1:]
		v, ok := mysqlSSHClients.Load(connID)
		if !ok {
			return nil, fmt.Errorf("no SSH client registered for conn %s", connID)
		}
		return v.(*ssh.Client).Dial("tcp", realAddr)
	})
}

func NewApp() *App {

	dbPath, err :=
		storage.DatabasePath()

	if err != nil {
		panic(err)
}

	db, err :=
		storage.NewDatabase(
			dbPath,
		)

	if err != nil {
		panic(err)
	}

	if err := db.Migrate(); err != nil {
		panic(err)
	}

	return &App{
		sessionManager:
			sessions.NewSessionManager(),
		transferManager:
    		transfers.NewManager(),

		database: db,
	}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	go a.startDockerLogServer()
}

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
	streamURL := fmt.Sprintf("/stream?session=%s&container=%s",
		neturl.QueryEscape(session), neturl.QueryEscape(container))
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<title>Logs — %s</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
html,body{height:100%%;background:#08060f;color:#c9d1de;font-family:"JetBrains Mono","Cascadia Code","Fira Code",monospace;font-size:12px;line-height:1.6}
#header{display:flex;align-items:center;justify-content:space-between;padding:10px 16px;background:#0d0b18;border-bottom:1px solid #1e1b2e;position:sticky;top:0;z-index:10;gap:12px}
#title{font-weight:600;font-size:13px;color:#e2e8f0}
#badge{font-size:10px;font-weight:700;letter-spacing:.06em;animation:pulse 1.5s ease-in-out infinite}
.live{color:#4ade80}.stopped{color:#f87171;animation:none!important}
@keyframes pulse{0%%,100%%{opacity:1}50%%{opacity:.3}}
#count{font-size:10px;color:#4b5563}
#controls{display:flex;gap:6px}
button{padding:3px 10px;border-radius:4px;border:1px solid #1e1b2e;background:transparent;color:#94a3b8;font-size:11px;font-family:inherit;cursor:pointer}
button:hover{background:#1e1b2e;color:#e2e8f0}
#follow-btn{border-color:var(--accent,#6366f1);color:#818cf8;display:none}
#logs{padding:10px 16px;min-height:calc(100%% - 45px)}
.line{white-space:pre-wrap;word-break:break-all}
::-webkit-scrollbar{width:6px}::-webkit-scrollbar-track{background:#0d0b18}::-webkit-scrollbar-thumb{background:#1e1b2e;border-radius:3px}
</style>
</head>
<body>
<div id="header">
  <div style="display:flex;align-items:center;gap:10px;min-width:0">
    <span id="title">%s</span>
    <span id="badge" class="live">● LIVE</span>
    <span id="count">0 líneas</span>
  </div>
  <div id="controls">
    <button onclick="document.getElementById('logs').innerHTML='';lineCount=0;updateCount()">Limpiar</button>
    <button id="follow-btn" onclick="enableFollow()">↓ Seguir</button>
  </div>
</div>
<div id="logs"></div>
<script>
var following=true,lineCount=0;
function updateCount(){document.getElementById('count').textContent=lineCount+' líneas'}
function enableFollow(){following=true;document.getElementById('follow-btn').style.display='none';window.scrollTo(0,document.body.scrollHeight)}
window.addEventListener('scroll',function(){
  var atBottom=window.innerHeight+window.scrollY>=document.body.offsetHeight-60;
  following=atBottom;
  document.getElementById('follow-btn').style.display=atBottom?'none':'block';
});
var es=new EventSource('%s');
es.onmessage=function(e){
  if(e.data==='__END__'){
    es.close();
    document.getElementById('badge').textContent='■ STOPPED';
    document.getElementById('badge').className='stopped';
    return;
  }
  var d=document.getElementById('logs');
  var line=document.createElement('div');
  line.className='line';
  line.textContent=e.data;
  d.appendChild(line);
  lineCount++;
  updateCount();
  if(following)window.scrollTo(0,document.body.scrollHeight);
};
es.onerror=function(){
  document.getElementById('badge').textContent='■ STOPPED';
  document.getElementById('badge').className='stopped';
};
</script>
</body>
</html>`, name, name, streamURL)
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

	ansiRe := strings.NewReplacer("\r", "")
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return
		default:
		}
		line := ansiRe.Replace(scanner.Text())
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
	runtime.BrowserOpenURL(a.ctx, url)
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
	runtime.BrowserOpenURL(a.ctx, url)
	return nil
}

// ─── Credentials storage ─────────────────────────────────────────────────────

type DbCred struct {
	User     string `json:"user"`
	Password string `json:"password"`
}

func dbCredPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".condui", "db-creds.json")
}

func loadAllDbCreds() map[string]DbCred {
	data, err := os.ReadFile(dbCredPath())
	if err != nil {
		return map[string]DbCred{}
	}
	var m map[string]DbCred
	json.Unmarshal(data, &m)
	if m == nil {
		m = map[string]DbCred{}
	}
	return m
}

func saveAllDbCreds(m map[string]DbCred) {
	p := dbCredPath()
	os.MkdirAll(filepath.Dir(p), 0700)
	data, _ := json.Marshal(m)
	os.WriteFile(p, data, 0600)
}

// ─── DB API helpers ──────────────────────────────────────────────────────────

func jsonOK(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func jsonErr(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// ─── DB API handlers ─────────────────────────────────────────────────────────

func (a *App) handleDbExplorer(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	dbType := r.URL.Query().Get("type")
	portStr := r.URL.Query().Get("port")
	apiBase := fmt.Sprintf("http://127.0.0.1:%d", a.logServerPort)

	html := strings.NewReplacer(
		`PLACEHOLDER_SESSION`, fmt.Sprintf("%q", sessionID),
		`PLACEHOLDER_DBTYPE`, fmt.Sprintf("%q", dbType),
		`PLACEHOLDER_PORT`, portStr,
		`PLACEHOLDER_API`, fmt.Sprintf("%q", apiBase),
	).Replace(dbExplorerHTML)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, html)
}

func (a *App) handleDbApiConnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Session  string `json:"session"`
		DbType   string `json:"dbType"`
		Port     int    `json:"port"`
		User     string `json:"user"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	connID, err := a.DbConnect(req.Session, req.DbType, req.Port, req.User, req.Password)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, map[string]string{"connId": connID})
}

func (a *App) handleDbApiDisconnect(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnID string `json:"connId"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	a.DbDisconnect(req.ConnID)
	jsonOK(w, map[string]bool{"ok": true})
}

func (a *App) handleDbApiDatabases(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	dbs, err := a.DbListDatabases(connID)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, dbs)
}

func (a *App) handleDbApiSchemas(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schemas, err := a.DbListSchemas(connID, db)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, schemas)
}

func (a *App) handleDbApiTables(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	tables, err := a.DbListTables(connID, db, schema)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, tables)
}

func (a *App) handleDbApiColumns(w http.ResponseWriter, r *http.Request) {
	connID := r.URL.Query().Get("connId")
	db := r.URL.Query().Get("db")
	schema := r.URL.Query().Get("schema")
	table := r.URL.Query().Get("table")
	cols, err := a.DbGetColumns(connID, db, schema, table)
	if err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	jsonOK(w, cols)
}

func (a *App) handleDbApiQuery(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnID string `json:"connId"`
		DB     string `json:"db"`
		SQL    string `json:"sql"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonErr(w, err.Error(), 400)
		return
	}
	result, _ := a.DbQuery(req.ConnID, req.DB, req.SQL)
	jsonOK(w, result)
}

func (a *App) handleDbApiCredentials(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		dbType := r.URL.Query().Get("type")
		port := r.URL.Query().Get("port")
		key := dbType + ":" + port
		creds := loadAllDbCreds()
		if c, ok := creds[key]; ok {
			jsonOK(w, c)
		} else {
			jsonOK(w, DbCred{})
		}
	case http.MethodPost:
		var req struct {
			Type     string `json:"type"`
			Port     int    `json:"port"`
			User     string `json:"user"`
			Password string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		key := fmt.Sprintf("%s:%d", req.Type, req.Port)
		creds := loadAllDbCreds()
		creds[key] = DbCred{User: req.User, Password: req.Password}
		saveAllDbCreds(creds)
		jsonOK(w, map[string]bool{"ok": true})
	case http.MethodDelete:
		var req struct {
			Type string `json:"type"`
			Port int    `json:"port"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		key := fmt.Sprintf("%s:%d", req.Type, req.Port)
		creds := loadAllDbCreds()
		delete(creds, key)
		saveAllDbCreds(creds)
		jsonOK(w, map[string]bool{"ok": true})
	default:
		jsonErr(w, "method not allowed", 405)
	}
}

func (a *App) ConnectSSH(
	connectionID string,
) (string, error) {

	connection, err :=
		a.database.GetConnectionByID(
			connectionID,
		)

	if err != nil {
		return "", err
	}

	config := &ssh.ClientConfig{
	User: connection.Username,
	Auth: []ssh.AuthMethod{
		ssh.Password(
			*connection.Password,
		),
	},
	HostKeyCallback:
		ssh.InsecureIgnoreHostKey(),
}


	client, err := ssh.Dial(
	"tcp",
	fmt.Sprintf(
		"%s:%d",
		connection.Host,
		connection.Port,
	),
	config,
)

	if err != nil {
		return "", err
	}

	sftpClient, err := sftp.NewClient(client)

if err != nil {
    return "", err
}

	session, err := client.NewSession()

	if err != nil {
		return "", err
	}

	modes := ssh.TerminalModes{
		ssh.ECHO: 1,
	}

	err = session.RequestPty(
		"xterm",
		40,
		120,
		modes,
	)

	if err != nil {
		return "", err
	}

	stdin, err := session.StdinPipe()

	if err != nil {
		return "", err
	}

	stdout, err := session.StdoutPipe()

	if err != nil {
		return "", err
	}

	stderr, err := session.StderrPipe()

	if err != nil {
		return "", err
	}

	sessionID := uuid.NewString()

	a.sessionManager.Add(
		&sessions.SSHSession{
			ID: sessionID,

			Client: client,
			Session: session,

        	SFTP: sftpClient,

			Stdin: stdin,
			Stdout: stdout,
			Stderr: stderr,

			Connected: true,

			Rows: 40,
			Cols: 120,
		},
	)

	if err := session.Shell(); err != nil {
		return "", err
	}

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stdout.Read(buffer)

			if err != nil {
				runtime.EventsEmit(
					a.ctx,
					"session-disconnected",
					map[string]any{
						"sessionId": sessionID,
					},
				)
				return
			}

			runtime.EventsEmit(
				a.ctx,
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	go func() {

		buffer := make([]byte, 4096)

		for {

			n, err := stderr.Read(buffer)

			if err != nil {
				return
			}

			runtime.EventsEmit(
				a.ctx,
				"terminal-output",
				map[string]any{
					"sessionId": sessionID,
					"data":      string(buffer[:n]),
				},
			)
		}
	}()

	return sessionID, nil
}

func (a *App) SendInput(
	sessionID string,
	data string,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	_, _ = session.Stdin.Write(
		[]byte(data),
	)
}

func (a *App) ListSessions() []string {

	list :=
		a.sessionManager.List()

	result := []string{}

	for _, s := range list {
		result = append(result, s.ID)
	}

	return result
}

func (a *App) CloseSession(
	sessionID string,
) error {

	return a.sessionManager.Remove(
		sessionID,
	)
}

func (a *App) ResizeTerminal(
	sessionID string,
	rows int,
	cols int,
) {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return
	}

	if session.Session == nil {
		return
	}

	_ = session.Session.WindowChange(
		rows,
		cols,
	)

	session.Rows = rows
	session.Cols = cols
}

func (a *App) CreateFolder(
	name string,
) error {

	_, err :=
		a.database.CreateFolder(
			name,
		)

	return err
}

func (a *App) GetFolders() (
	[]models.Folder,
	error,
) {

	return a.database.GetFolders()

}

func (a *App) GetConnections() (
	[]models.Connection,
	error,
) {

	return a.database.GetConnections()

}

func (a *App) CreateConnection(
	connection models.Connection,
) error {

	return a.database.CreateConnection(
		&connection,
	)
}

func (a *App) UpdateConnection(
	connection models.Connection,
) error {

	return a.database.UpdateConnection(
		&connection,
	)
}

func (a *App) DeleteConnection(
	id string,
) error {

	return a.database.DeleteConnection(
		id,
	)
}

func (a *App) UpdateFolder(
	id string,
	name string,
) error {

	return a.database.UpdateFolder(
		id,
		name,
	)
}

func (a *App) DeleteFolder(
	id string,
) error {

	return a.database.DeleteFolder(
		id,
	)
}

func (a *App) GetConnectionByID(
	id string,
) (*models.Connection, error) {

	return a.database.GetConnectionByID(
		id,
	)

}

func (a *App) ListDirectory(
    sessionID string,
    path string,
) (
    []sftpservice.FileItem,
    error,
) {

    session, ok :=
        a.sessionManager.Get(
            sessionID,
        )

    if !ok {

        return nil,
            fmt.Errorf(
                "session not found",
            )

    }

    if session.SFTP == nil {

        return nil,
            fmt.Errorf(
                "sftp client nil",
            )

    }

    return sftpservice.ListDirectory(
        session.SFTP,
        path,
    )
}

func (a *App) UploadFile(sessionID string, remoteDirectory string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok { return fmt.Errorf("session not found") }

	localPath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{Title: "Subir archivo"})
	if err != nil || localPath == "" { return fmt.Errorf("cancelado") }

	fileName := sftpservice.GetFileName(localPath)
	var remotePath string
	if remoteDirectory == "/" { remotePath = fmt.Sprintf("/%s", fileName) } else { remotePath = fmt.Sprintf("%s/%s", remoteDirectory, fileName) }

	// Abrir archivo local para obtener su peso total
	localFile, err := os.Open(localPath)
	if err != nil { return err }
	defer localFile.Close()

	stat, err := localFile.Stat()
	if err != nil { return err }

	// Crear archivo en el servidor remoto por SFTP
	remoteFile, err := session.SFTP.Create(remotePath)
	if err != nil { return err }
	defer remoteFile.Close()

	transferID := uuid.NewString()
	a.emitLog("SFTP", "Iniciando subida de: "+fileName, "")

	// Envolver el destino en nuestro ProgressWriter
	progress := &models.ProgressWriter{
		Total:       stat.Size(),
		ID:          transferID,
		FileName:    fileName,
		Direction:   "upload",
		AppCtx:      a.ctx,
	}

	// MultiWriter escribe en el archivo remoto y a la vez computa el progreso
	mw := io.MultiWriter(remoteFile, progress)
	_, err = io.Copy(mw, localFile)

	if err != nil {
		runtime.EventsEmit(a.ctx, "transfer-status", map[string]any{"id": transferID, "status": "error"})
		a.emitLog("SFTP", "Error al subir "+fileName, "error")
		return err
	}

	runtime.EventsEmit(a.ctx, "transfer-status", map[string]any{"id": transferID, "name": fileName, "progress": 100, "status": "done"})
	a.emitLog("SFTP", "Subida completada: "+fileName, "success")
	return nil
}

func (a *App) DownloadFile(
	sessionID string,
	remotePath string,
	localPath string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	// 1. Abrir diálogo nativo para guardar archivo
	fileName := sftpservice.GetFileName(remotePath) // O usa path.Base(remotePath)
	
	chosenLocalPath, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Descargar archivo remoto",
		DefaultFilename: fileName,
	})
	
	if err != nil || chosenLocalPath == "" {
		return fmt.Errorf("descarga cancelada por el usuario")
	}

	return sftpservice.DownloadFile(
		session.SFTP,
		remotePath,
		chosenLocalPath,
	)
}

func (a *App) DeleteRemoteFile(
	sessionID string,
	path string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.DeleteFile(
		session.SFTP,
		path,
	)
}

func (a *App) RenameRemoteFile(
	sessionID string,
	oldPath string,
	newPath string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.RenameFile(
		session.SFTP,
		oldPath,
		newPath,
	)
}

func (a *App) CreateRemoteDirectory(
	sessionID string,
	path string,
) error {

	session, ok :=
		a.sessionManager.Get(sessionID)

	if !ok {
		return fmt.Errorf("session not found")
	}

	return sftpservice.CreateDirectory(
		session.SFTP,
		path,
	)
}

func (a *App) ReadRemoteFile(
	sessionID string,
	path string,
) (string, error) {

	session, ok :=
		a.sessionManager.Get(
			sessionID,
		)

	if !ok {
		return "",
			fmt.Errorf(
				"session not found",
			)
	}


	if session.SFTP == nil {
		return "",
			fmt.Errorf(
				"sftp client nil",
			)
	}


	content, err :=
		sftpservice.ReadFile(
			session.SFTP,
			path,
		)


	if err != nil {
		return "", err
	}


	ext :=
		strings.ToLower(
			filepath.Ext(path),
		)


	switch ext {

	case ".png",
		".jpg",
		".jpeg",
		".gif",
		".webp",
		".bmp":

		return base64.StdEncoding.EncodeToString(
			[]byte(content),
		), nil


	default:

		return content, nil

	}

}


func (a *App) SaveRemoteFile(
	sessionID string,
	path string,
	content string,
) error {

	session, ok :=
		a.sessionManager.Get(
			sessionID,
		)

	if !ok {
		return fmt.Errorf(
			"session not found",
		)
	}

	if session.SFTP == nil {
		return fmt.Errorf(
			"sftp client nil",
		)
	}

	return sftpservice.WriteFile(
		session.SFTP,
		path,
		content,
	)
}

type SystemStats struct {
	CPUPercent  float64 `json:"cpuPercent"`
	MemUsedGB   float64 `json:"memUsedGB"`
	MemFreeGB   float64 `json:"memFreeGB"`
	MemTotalGB  float64 `json:"memTotalGB"`
	DiskUsedGB  float64 `json:"diskUsedGB"`
	DiskFreeGB  float64 `json:"diskFreeGB"`
	DiskTotalGB float64 `json:"diskTotalGB"`
	UptimeSecs  float64 `json:"uptimeSecs"`
	NetRxBps    float64 `json:"netRxBps"`
	NetTxBps    float64 `json:"netTxBps"`
	DiskReadBps float64 `json:"diskReadBps"`
	DiskWriteBps float64 `json:"diskWriteBps"`
}

func (a *App) GetSystemStats(sessionID string) (*SystemStats, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	// Cada comando emite su propia línea con \n; los dos últimos pares son
	// snapshots separados 1 s para calcular tasas de red y disco.
	cmd := `awk 'NR==1{idle=$5;total=$2+$3+$4+$5+$6+$7+$8;printf "%.0f\n",(1-idle/total)*100}' /proc/stat 2>/dev/null||echo 0` +
		`; free -b 2>/dev/null|awk '/Mem:/{printf "%d %d\n",$3,$2}'||echo '0 0'` +
		`; df -B1 / 2>/dev/null|awk 'NR==2{printf "%d %d\n",$3,$2}'||echo '0 0'` +
		`; awk '{printf "%.0f\n",$1}' /proc/uptime 2>/dev/null||echo 0` +
		`; awk 'NR>2{gsub(/:/,"",$1);if($1!="lo"){rx+=$2;tx+=$10}} END{print rx+0,tx+0}' /proc/net/dev 2>/dev/null||echo '0 0'` +
		`; awk '$3!~/^loop/{r+=$6;w+=$10} END{print r+0,w+0}' /proc/diskstats 2>/dev/null||echo '0 0'` +
		`; sleep 1` +
		`; awk 'NR>2{gsub(/:/,"",$1);if($1!="lo"){rx+=$2;tx+=$10}} END{print rx+0,tx+0}' /proc/net/dev 2>/dev/null||echo '0 0'` +
		`; awk '$3!~/^loop/{r+=$6;w+=$10} END{print r+0,w+0}' /proc/diskstats 2>/dev/null||echo '0 0'`

	output, err := cmdSession.Output(cmd)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) < 8 {
		return nil, fmt.Errorf("unexpected stats output")
	}

	const gb = 1073741824.0
	stats := &SystemStats{}

	fmt.Sscanf(strings.TrimSpace(lines[0]), "%f", &stats.CPUPercent)

	var memUsed, memTotal float64
	fmt.Sscanf(strings.TrimSpace(lines[1]), "%f %f", &memUsed, &memTotal)
	stats.MemUsedGB = memUsed / gb
	stats.MemTotalGB = memTotal / gb
	stats.MemFreeGB = (memTotal - memUsed) / gb

	var diskUsed, diskTotal float64
	fmt.Sscanf(strings.TrimSpace(lines[2]), "%f %f", &diskUsed, &diskTotal)
	stats.DiskUsedGB = diskUsed / gb
	stats.DiskTotalGB = diskTotal / gb
	stats.DiskFreeGB = (diskTotal - diskUsed) / gb

	fmt.Sscanf(strings.TrimSpace(lines[3]), "%f", &stats.UptimeSecs)

	// Tasas de red: bytes/s entre los dos snapshots (1 segundo de diferencia)
	var netRx1, netTx1, netRx2, netTx2 float64
	fmt.Sscanf(strings.TrimSpace(lines[4]), "%f %f", &netRx1, &netTx1)
	fmt.Sscanf(strings.TrimSpace(lines[6]), "%f %f", &netRx2, &netTx2)
	if rx := netRx2 - netRx1; rx > 0 { stats.NetRxBps = rx }
	if tx := netTx2 - netTx1; tx > 0 { stats.NetTxBps = tx }

	// Tasas de disco: sectores/s × 512 bytes/sector
	var diskR1, diskW1, diskR2, diskW2 float64
	fmt.Sscanf(strings.TrimSpace(lines[5]), "%f %f", &diskR1, &diskW1)
	fmt.Sscanf(strings.TrimSpace(lines[7]), "%f %f", &diskR2, &diskW2)
	if r := (diskR2 - diskR1) * 512; r > 0 { stats.DiskReadBps = r }
	if w := (diskW2 - diskW1) * 512; w > 0 { stats.DiskWriteBps = w }

	return stats, nil
}

func (a *App) emitLog(logType string, message string, class string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "log-event", map[string]string{
		"time": time.Now().Format("15:04:05"),
		"type": logType,
		"msg":  message,
		"cls":  class, // "success", "warn", "error" o ""
	})
}

var activeListeners = make(map[string]net.Listener)

var (
    tunnelsMutex  sync.RWMutex
    runtimeTunnels = make(map[string]*models.ActiveTunnel)
    // Almacén en memoria para los túneles registrados por Sesión/Conexión si no usas DB todavía
    registeredTunnels = make(map[string][]models.TunnelInfo) 
)

// GetTunnels obtiene los túneles vinculados a la conexión.
// Por ahora, si no deseas modificar la DB inmediatamente, puedes usar este almacén dinámico/mock
// GetTunnels obtiene la lista de túneles dinámicos guardados para la sesión activa
func (a *App) GetTunnels(sessionID string) ([]models.TunnelInfo, error) {
    tunnelsMutex.RLock()
    defer tunnelsMutex.RUnlock()

    list, exists := registeredTunnels[sessionID]
    if !exists {
        return []models.TunnelInfo{}, nil
    }

    // Sincronizar el flag de Active en tiempo real basándose en si el listener genérico sigue vivo
    for i := range list {
        list[i].Active = runtimeTunnels[list[i].ID] != nil
    }

    return list, nil
}

// AddTunnel registra un nuevo túnel dinámico en la sesión actual
func (a *App) AddTunnel(sessionID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
    tunnelsMutex.Lock()
    defer tunnelsMutex.Unlock()

    tunnelID := uuid.NewString()
    newTunnel := models.TunnelInfo{
        ID:         tunnelID,
        LocalPort:  localPort,
        RemoteHost: remoteHost,
        RemotePort: remotePort,
        Active:     false,
    }

    registeredTunnels[sessionID] = append(registeredTunnels[sessionID], newTunnel)
    a.emitLog("TUNNEL", fmt.Sprintf("Nuevo túnel registrado: :%d -> %s:%d", localPort, remoteHost, remotePort), "")
    
    return newTunnel, nil
}

// DeleteTunnel elimina un túnel del registro (y lo apaga si está encendido)
func (a *App) DeleteTunnel(sessionID string, tunnelID string) error {
    // Apagar primero si está corriendo
    _ = a.ToggleTunnel(sessionID, tunnelID, 0, "", 0, false)

    tunnelsMutex.Lock()
    defer tunnelsMutex.Unlock()

    list := registeredTunnels[sessionID]
    for i, t := range list {
        if t.ID == tunnelID {
            // Eliminar del slice
            registeredTunnels[sessionID] = append(list[:i], list[i+1:]...)
            a.emitLog("TUNNEL", fmt.Sprintf("Túnel :%d eliminado del registro.", t.LocalPort), "warn")
            break
        }
    }
    return nil
}

// EditTunnel modifica los parámetros de un túnel existente
func (a *App) EditTunnel(sessionID string, tunnelID string, localPort int, remoteHost string, remotePort int) (models.TunnelInfo, error) {
    // 1. Apagar el túnel primero si estuviera corriendo en tiempo de ejecución
    _ = a.ToggleTunnel(sessionID, tunnelID, 0, "", 0, false)

    tunnelsMutex.Lock()
    defer tunnelsMutex.Unlock()

    list, exists := registeredTunnels[sessionID]
    if !exists {
        return models.TunnelInfo{}, fmt.Errorf("no se encontraron túneles para esta sesión")
    }

    for i, t := range list {
        if t.ID == tunnelID {
            // 2. Actualizar valores
            list[i].LocalPort = localPort
            list[i].RemoteHost = remoteHost
            list[i].RemotePort = remotePort
            
            registeredTunnels[sessionID] = list
            a.emitLog("TUNNEL", fmt.Sprintf("Túnel editado con éxito: Mapeado a :%d", localPort), "success")
            return list[i], nil
        }
    }

    return models.TunnelInfo{}, fmt.Errorf("túnel no encontrado")
}

// ToggleTunnel Enciende o apaga el túnel SSH local port forwarding de forma asíncrona
func (a *App) ToggleTunnel(sessionID string, tunnelID string, localPort int, remoteHost string, remotePort int, activate bool) error {
    session, ok := a.sessionManager.Get(sessionID)
    if !ok {
        return fmt.Errorf("session not found")
    }

    tunnelsMutex.Lock()
    defer tunnelsMutex.Unlock()

    if !activate {
        // APAGAR TÚNEL
        if t, exists := runtimeTunnels[tunnelID]; exists {
            t.Listener.Close()
            delete(runtimeTunnels, tunnelID)
            a.emitLog("TUNNEL", fmt.Sprintf("Túnel local :%d cerrado de forma segura.", t.LocalPort), "warn")
        }
        return nil
    }

    // Si se activa por interfaz, pero no se pasaron parámetros directos, buscamos en el registro guardado
    if localPort == 0 {
        for _, t := range registeredTunnels[sessionID] {
            if t.ID == tunnelID {
                localPort = t.LocalPort
                remoteHost = t.RemoteHost
                remotePort = t.RemotePort
                break
            }
        }
    }

    // ENCENDER TÚNEL
    localAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
    listener, err := net.Listen("tcp", localAddr)
    if err != nil {
        a.emitLog("TUNNEL", fmt.Sprintf("Error abriendo puerto local :%d: %v", localPort, err), "error")
        return fmt.Errorf("no se pudo abrir el puerto local: %v", err)
    }

    runtimeTunnels[tunnelID] = &models.ActiveTunnel{
        Listener:   listener,
        LocalPort:  localPort,
        RemoteHost: remoteHost,
        RemotePort: remotePort,
    }
    
    a.emitLog("TUNNEL", fmt.Sprintf("Túnel activo: local :%d retransmitiendo a %s:%d", localPort, remoteHost, remotePort), "success")

    // Goroutine dedicada para la escucha bidireccional del flujo TCP cifrado
    go func(l net.Listener, rHost string, rPort int) {
        for {
            localConn, err := l.Accept()
            if err != nil {
                return // Listener cerrado externamente por el Close()
            }

            // Establecer canal seguro por dentro del cliente SSH de Wails hacia la máquina remota
            remoteConn, err := session.Client.Dial("tcp", fmt.Sprintf("%s:%d", rHost, rPort))
            if err != nil {
                localConn.Close()
                a.emitLog("TUNNEL", fmt.Sprintf("Fallo de reenvío TCP hacia %s:%d", rHost, rPort), "error")
                continue
            }

            // Intercambio asíncrono bidireccional continuo (Puente simétrico)
            go func() {
                defer localConn.Close()
                defer remoteConn.Close()
                _, _ = io.Copy(localConn, remoteConn)
            }()
            go func() {
                defer localConn.Close()
                defer remoteConn.Close()
                _, _ = io.Copy(remoteConn, localConn)
            }()
        }
    }(listener, remoteHost, remotePort)

    return nil
}

func (a *App) GetDockerContainers(sessionID string) ([]models.DockerContainer, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	output, err := cmdSession.Output("docker ps -a --format '{\"id\":\"{{.ID}}\",\"names\":\"{{.Names}}\",\"image\":\"{{.Image}}\",\"status\":\"{{.Status}}\",\"state\":\"{{.State}}\",\"ports\":\"{{.Ports}}\"}'")
	if err != nil {
		return nil, fmt.Errorf("docker no responde, puede que no esté instalado en el servidor remoto o requiera privilegios de sudo")
	}

	lines := strings.Split(string(output), "\n")
	var containers []models.DockerContainer

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var c models.DockerContainer
		c.ID     = extractJSONField(line, "id")
		c.Names  = extractJSONField(line, "names")
		c.Image  = extractJSONField(line, "image")
		c.Status = extractJSONField(line, "status")
		c.State  = extractJSONField(line, "state")
		c.Ports  = extractJSONField(line, "ports")

		if c.ID != "" {
			containers = append(containers, c)
		}
	}

	return containers, nil
}

func (a *App) GetListeningPorts(sessionID string) ([]models.PortInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return nil, err
	}
	defer cmdSession.Close()

	// Extrae proto, puerto, dirección y proceso de ss -tlnp / ss -ulnp
	awkProg := `NR>1{n=split($4,addr,":");port=addr[n];line=$0;proc="-";` +
		`if(index(line,"users:((\"")>0){sub(/.*users:\(\("/, "",line);sub(/".*/, "",line);proc=line};` +
		`print proto" "port" "$4" "proc}`

	cmd := `ss -tlnp 2>/dev/null | awk -v proto=TCP '` + awkProg + `'` +
		`; ss -ulnp 2>/dev/null | awk -v proto=UDP '` + awkProg + `'`

	output, err := cmdSession.Output(cmd)
	if err != nil {
		return nil, err
	}

	var ports []models.PortInfo
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}
		var port int
		fmt.Sscanf(parts[1], "%d", &port)
		if port == 0 {
			continue
		}
		proc := "-"
		if len(parts) >= 4 {
			proc = parts[3]
		}
		ports = append(ports, models.PortInfo{
			Proto:   parts[0],
			Port:    port,
			Address: parts[2],
			Process: proc,
		})
	}

	return ports, nil
}

// ToggleContainer ejecuta las acciones vitales: start, stop o restart
func (a *App) ToggleContainer(sessionID string, containerID string, action string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return "", err
	}
	defer cmdSession.Close()

	validActions := map[string]bool{"start": true, "stop": true, "restart": true}
	if !validActions[action] {
		return "", fmt.Errorf("acción inválida")
	}

	output, err := cmdSession.CombinedOutput(fmt.Sprintf("docker %s %s", action, containerID))
	if err == nil {
		a.emitLog("DOCKER", fmt.Sprintf("Contenedor %s ejecutó: %s", containerID, action), "success")
	} else {
		a.emitLog("DOCKER", fmt.Sprintf("Error en contenedor %s: %s", containerID, string(output)), "error")
	}
	return string(output), err
}

// StartDockerLogs abre docker logs -f en el contenedor y emite cada línea como evento Wails
func (a *App) StartDockerLogs(sessionID string, containerID string) error {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return fmt.Errorf("session not found")
	}

	a.StopDockerLogs(sessionID, containerID)

	cmdSession, err := session.Client.NewSession()
	if err != nil {
		return err
	}

	stdout, err := cmdSession.StdoutPipe()
	if err != nil {
		cmdSession.Close()
		return err
	}

	if err := cmdSession.Start(fmt.Sprintf("docker logs -f --tail=300 %s 2>&1", containerID)); err != nil {
		cmdSession.Close()
		return err
	}

	key := sessionID + ":" + containerID
	a.dockerLogMu.Lock()
	if a.dockerLogSessions == nil {
		a.dockerLogSessions = make(map[string]*ssh.Session)
	}
	a.dockerLogSessions[key] = cmdSession
	a.dockerLogMu.Unlock()

	go func() {
		defer func() {
			cmdSession.Close()
			a.dockerLogMu.Lock()
			delete(a.dockerLogSessions, key)
			a.dockerLogMu.Unlock()
			runtime.EventsEmit(a.ctx, "docker-log-end-"+containerID)
		}()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			runtime.EventsEmit(a.ctx, "docker-log-"+containerID, scanner.Text())
		}
	}()

	return nil
}

// StopDockerLogs detiene el stream de logs cerrando la sesión SSH
func (a *App) StopDockerLogs(sessionID string, containerID string) {
	key := sessionID + ":" + containerID
	a.dockerLogMu.Lock()
	sess, ok := a.dockerLogSessions[key]
	if ok {
		delete(a.dockerLogSessions, key)
	}
	a.dockerLogMu.Unlock()
	if ok {
		sess.Close()
	}
}

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

func (a *App) GetDatabases(sessionID string) ([]models.DatabaseInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}

	cmdSession, err := session.Client.NewSession()
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

// Función utilitaria limpia para parsear los campos string JSON devueltos por el comando docker sin romper el flujo
func extractJSONField(jsonStr, field string) string {
	key := fmt.Sprintf("\"%s\":\"", field)
	idx := strings.Index(jsonStr, key)
	if idx == -1 {
		return ""
	}
	start := idx + len(key)
	end := strings.Index(jsonStr[start:], "\"")
	if end == -1 {
		return ""
	}
	return jsonStr[start : start+end]
}
// ========================= DB EXPLORER =========================

func normalizeDbType(t string) string {
	l := strings.ToLower(t)
	if strings.Contains(l, "postgres") || strings.Contains(l, "timescale") {
		return "postgresql"
	}
	if strings.Contains(l, "mysql") || strings.Contains(l, "mariadb") {
		return "mysql"
	}
	return l
}

func (a *App) getDbConn(id string) *DbConn {
	a.dbConnMu.Lock()
	defer a.dbConnMu.Unlock()
	return a.dbConns[id]
}

// pgGetPool returns (creating if needed) a *pgxpool.Pool for the given database.
// Pools are safe for concurrent use, unlike *pgx.Conn.
func (conn *DbConn) pgGetPool(database string) (*pgxpool.Pool, error) {
	if database == "" {
		database = "postgres"
	}
	conn.pgMu.Lock()
	defer conn.pgMu.Unlock()

	if p, ok := conn.pgPools[database]; ok {
		return p, nil
	}

	cfg, err := pgxpool.ParseConfig(fmt.Sprintf(
		"host=127.0.0.1 port=%d user=%s password=%s dbname=%s sslmode=disable",
		conn.Port, conn.User, conn.Password, database))
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.DialFunc = func(ctx context.Context, network, addr string) (net.Conn, error) {
		return conn.sshClient.Dial("tcp", addr)
	}
	cfg.MaxConns = 4

	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	conn.pgPools[database] = p
	return p, nil
}

// myGetDB returns (creating if needed) a *sql.DB for the given MySQL database.
func (conn *DbConn) myGetDB(database string) (*sql.DB, error) {
	conn.myMu.Lock()
	defer conn.myMu.Unlock()

	if db, ok := conn.myDBs[database]; ok {
		return db, nil
	}

	cfg := mysql.NewConfig()
	cfg.User = conn.User
	cfg.Passwd = conn.Password
	cfg.Net = "condui-ssh"
	cfg.Addr = fmt.Sprintf("%s@127.0.0.1:%d", conn.ID, conn.Port)
	cfg.DBName = database
	cfg.ParseTime = true
	cfg.MultiStatements = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return nil, err
	}
	conn.myDBs[database] = db
	return db, nil
}

// pgExecResult runs sql on a pgxpool and returns a QueryResult.
func pgExecResult(pool *pgxpool.Pool, query string) models.QueryResult {
	rows, err := pool.Query(context.Background(), query)
	if err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	defer rows.Close()

	var res models.QueryResult
	for _, fd := range rows.FieldDescriptions() {
		res.Columns = append(res.Columns, fd.Name)
	}
	for rows.Next() {
		vals, err := rows.Values()
		if err != nil {
			break
		}
		row := make([]string, len(vals))
		for i, v := range vals {
			if v == nil {
				row[i] = ""
			} else {
				switch t := v.(type) {
				case []byte:
					row[i] = string(t)
				case time.Time:
					row[i] = t.Format(time.RFC3339)
				default:
					row[i] = fmt.Sprintf("%v", t)
				}
			}
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	res.RowCount = len(res.Rows)
	return res
}

// myExecResult runs sql on a *sql.DB and returns a QueryResult.
func myExecResult(db *sql.DB, query string) models.QueryResult {
	ctx := context.Background()
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		// Non-SELECT DML/DDL: fall back to Exec
		result, execErr := db.ExecContext(ctx, query)
		if execErr != nil {
			return models.QueryResult{Error: execErr.Error()}
		}
		affected, _ := result.RowsAffected()
		return models.QueryResult{RowCount: int(affected)}
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	var res models.QueryResult
	res.Columns = cols

	raw := make([]sql.RawBytes, len(cols))
	dest := make([]interface{}, len(cols))
	for i := range raw {
		dest[i] = &raw[i]
	}
	for rows.Next() {
		if err := rows.Scan(dest...); err != nil {
			break
		}
		row := make([]string, len(cols))
		for i, v := range raw {
			if v == nil {
				row[i] = ""
			} else {
				row[i] = string(v)
			}
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return models.QueryResult{Error: err.Error()}
	}
	res.RowCount = len(res.Rows)
	return res
}

// execQuery dispatches a SQL query to the right driver connection.
func (conn *DbConn) execQuery(database, query string) models.QueryResult {
	switch conn.DbType {
	case "postgresql":
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return models.QueryResult{Error: err.Error()}
		}
		return pgExecResult(pool, query)
	case "mysql":
		db, err := conn.myGetDB(database)
		if err != nil {
			return models.QueryResult{Error: err.Error()}
		}
		return myExecResult(db, query)
	}
	return models.QueryResult{Error: "unsupported database type"}
}

// firstColumn runs a single-column query and returns the values.
func (conn *DbConn) firstColumn(database, query string) ([]string, error) {
	r := conn.execQuery(database, query)
	if r.Error != "" {
		return nil, fmt.Errorf("%s", r.Error)
	}
	var out []string
	for _, row := range r.Rows {
		if len(row) > 0 {
			out = append(out, row[0])
		}
	}
	return out, nil
}

// DbConnect opens an SSH-tunneled database connection using native Go drivers.
func (a *App) DbConnect(sessionID, dbType string, port int, user, password string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("SSH session not found")
	}

	id := uuid.New().String()
	conn := &DbConn{
		ID:        id,
		SessionID: sessionID,
		DbType:    normalizeDbType(dbType),
		Port:      port,
		User:      user,
		Password:  password,
		sshClient: session.Client,
	}

	switch conn.DbType {
	case "postgresql":
		conn.pgPools = make(map[string]*pgxpool.Pool)
		// Test connection via pool ping
		pool, err := conn.pgGetPool("postgres")
		if err != nil {
			return "", err
		}
		if err := pool.Ping(context.Background()); err != nil {
			return "", err
		}

	case "mysql":
		conn.myDBs = make(map[string]*sql.DB)
		mysqlSSHClients.Store(id, session.Client)
		db, err := conn.myGetDB("")
		if err != nil {
			mysqlSSHClients.Delete(id)
			return "", err
		}
		if err := db.PingContext(context.Background()); err != nil {
			mysqlSSHClients.Delete(id)
			return "", err
		}

	default:
		return "", fmt.Errorf("tipo de base de datos no soportado: %s", dbType)
	}

	a.dbConnMu.Lock()
	if a.dbConns == nil {
		a.dbConns = make(map[string]*DbConn)
	}
	a.dbConns[id] = conn
	a.dbConnMu.Unlock()
	return id, nil
}

// DbDisconnect closes all driver connections and cleans up resources.
func (a *App) DbDisconnect(connID string) {
	a.dbConnMu.Lock()
	conn := a.dbConns[connID]
	delete(a.dbConns, connID)
	a.dbConnMu.Unlock()
	if conn == nil {
		return
	}
	conn.pgMu.Lock()
	for _, p := range conn.pgPools {
		p.Close()
	}
	conn.pgMu.Unlock()
	conn.myMu.Lock()
	for _, db := range conn.myDBs {
		db.Close()
	}
	conn.myMu.Unlock()
	if conn.DbType == "mysql" {
		mysqlSSHClients.Delete(connID)
	}
}

// DbListDatabases returns available databases over the SSH-tunneled connection.
func (a *App) DbListDatabases(connID string) ([]string, error) {
	conn := a.getDbConn(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	switch conn.DbType {
	case "postgresql":
		return conn.firstColumn("postgres",
			"SELECT datname FROM pg_database WHERE datistemplate=false ORDER BY datname")
	case "mysql":
		return conn.firstColumn("", "SHOW DATABASES")
	}
	return nil, fmt.Errorf("unsupported")
}

// DbListSchemas returns schemas for a given database.
func (a *App) DbListSchemas(connID, database string) ([]string, error) {
	conn := a.getDbConn(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	switch conn.DbType {
	case "postgresql":
		return conn.firstColumn(database,
			`SELECT schema_name FROM information_schema.schemata
			 WHERE schema_name NOT IN ('pg_catalog','information_schema','pg_toast')
			   AND schema_name NOT LIKE 'pg_temp%' AND schema_name NOT LIKE 'pg_toast_temp%'
			 ORDER BY schema_name`)
	case "mysql":
		return []string{database}, nil
	}
	return nil, fmt.Errorf("unsupported")
}

// DbListTables returns tables for a database/schema using parameterized queries.
func (a *App) DbListTables(connID, database, schema string) ([]string, error) {
	conn := a.getDbConn(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	ctx := context.Background()
	switch conn.DbType {
	case "postgresql":
		if schema == "" {
			schema = "public"
		}
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return nil, err
		}
		rows, err := pool.Query(ctx,
			`SELECT table_name FROM information_schema.tables
			 WHERE table_schema=$1 AND table_type='BASE TABLE'
			 ORDER BY table_name`, schema)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var t string
			rows.Scan(&t)
			tables = append(tables, t)
		}
		return tables, rows.Err()
	case "mysql":
		return conn.firstColumn(database, "SHOW TABLES")
	}
	return nil, fmt.Errorf("unsupported")
}

// DbGetColumns returns column metadata using parameterized queries.
func (a *App) DbGetColumns(connID, database, schema, table string) ([]models.QueryColumn, error) {
	conn := a.getDbConn(connID)
	if conn == nil {
		return nil, fmt.Errorf("connection not found")
	}
	ctx := context.Background()
	switch conn.DbType {
	case "postgresql":
		if schema == "" {
			schema = "public"
		}
		pool, err := conn.pgGetPool(database)
		if err != nil {
			return nil, err
		}
		rows, err := pool.Query(ctx, `
			SELECT c.column_name, c.data_type,
			       c.is_nullable='YES',
			       COALESCE(c.column_default,''),
			       CASE WHEN pk.column_name IS NOT NULL THEN 'PK' ELSE '' END
			FROM information_schema.columns c
			LEFT JOIN (
			    SELECT kcu.column_name
			    FROM information_schema.table_constraints tc
			    JOIN information_schema.key_column_usage kcu
			      ON tc.constraint_name=kcu.constraint_name AND tc.table_schema=kcu.table_schema
			    WHERE tc.constraint_type='PRIMARY KEY'
			      AND tc.table_schema=$1 AND tc.table_name=$2
			) pk ON c.column_name=pk.column_name
			WHERE c.table_schema=$1 AND c.table_name=$2
			ORDER BY c.ordinal_position`, schema, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []models.QueryColumn
		for rows.Next() {
			var c models.QueryColumn
			rows.Scan(&c.Name, &c.DataType, &c.Nullable, &c.Default, &c.Key)
			cols = append(cols, c)
		}
		return cols, rows.Err()

	case "mysql":
		db, err := conn.myGetDB(database)
		if err != nil {
			return nil, err
		}
		rows, err := db.QueryContext(ctx, `
			SELECT COLUMN_NAME, DATA_TYPE,
			       IF(IS_NULLABLE='YES','1','0'),
			       COALESCE(COLUMN_DEFAULT,''),
			       COLUMN_KEY
			FROM INFORMATION_SCHEMA.COLUMNS
			WHERE TABLE_SCHEMA=? AND TABLE_NAME=?
			ORDER BY ORDINAL_POSITION`, database, table)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var cols []models.QueryColumn
		for rows.Next() {
			var c models.QueryColumn
			var nullableStr string
			rows.Scan(&c.Name, &c.DataType, &nullableStr, &c.Default, &c.Key)
			c.Nullable = nullableStr == "1"
			cols = append(cols, c)
		}
		return cols, rows.Err()
	}
	return nil, fmt.Errorf("unsupported")
}

// DbQuery executes arbitrary SQL and returns the result.
func (a *App) DbQuery(connID, database, query string) (models.QueryResult, error) {
	conn := a.getDbConn(connID)
	if conn == nil {
		return models.QueryResult{Error: "connection not found"}, nil
	}
	return conn.execQuery(database, query), nil
}

// ─── DB Explorer HTML ────────────────────────────────────────────────────────

const dbExplorerHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="UTF-8">
<title>DB Explorer</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
:root{
  --bg:#0d1117;--s1:#161b22;--s2:#1c2128;
  --b1:#21262d;--b2:#30363d;
  --tx:#c9d1d9;--mu:#8b949e;
  --ac:#58a6ff;--gr:#3fb950;--rd:#f85149;--or:#e3b341;
  font-family:"JetBrains Mono","Fira Code","Cascadia Code",Consolas,monospace;font-size:12px
}
html,body{height:100%;overflow:hidden;background:var(--bg);color:var(--tx)}
input,select,textarea,button{font:inherit;color:inherit}

/* connect */
#cv{height:100%;display:flex;align-items:center;justify-content:center}
#cf{width:320px;background:var(--s1);border:1px solid var(--b1);border-radius:10px;padding:28px 30px;box-shadow:0 24px 64px rgba(0,0,0,.6)}
.cf-head{display:flex;align-items:center;gap:10px;margin-bottom:22px}
.cf-head h2{font-size:14px;font-weight:700}
.cf-head span{font-size:11px;color:var(--mu)}
.lbl{display:block;font-size:10px;font-weight:700;color:var(--mu);text-transform:uppercase;letter-spacing:.07em;margin:12px 0 4px}
.inp{width:100%;padding:7px 10px;border-radius:6px;border:1px solid var(--b2);background:var(--bg);color:var(--tx);outline:none;transition:border-color .15s,box-shadow .15s}
.inp:focus{border-color:var(--ac);box-shadow:0 0 0 3px rgba(88,166,255,.12)}
.chk-row{display:flex;align-items:center;gap:8px;margin-top:14px;font-size:11px;color:var(--mu);cursor:pointer;user-select:none}
#ce{display:none;margin-top:10px;padding:8px 12px;border-radius:6px;background:rgba(248,81,73,.1);border:1px solid rgba(248,81,73,.25);color:var(--rd);font-size:11px;line-height:1.55}
.btn-p{width:100%;margin-top:18px;padding:9px;border-radius:6px;background:var(--ac);color:#0d1117;border:none;cursor:pointer;font-weight:700;font-size:12px;transition:opacity .15s}
.btn-p:hover{opacity:.85}
.btn-p:disabled{opacity:.4;cursor:default}

/* explorer shell */
#ev{height:100%;display:none;flex-direction:column}

/* header */
#hdr{display:flex;align-items:center;gap:8px;padding:6px 14px;background:var(--s1);border-bottom:1px solid var(--b1);flex-shrink:0;min-height:38px}
#htype{font-weight:700;font-size:12px}
#hport,#hdb{font-size:11px;color:var(--mu)}
#hdb{color:var(--ac)}
.sp{flex:1}
.hbtn{padding:3px 10px;border-radius:4px;border:1px solid var(--b2);background:transparent;color:var(--mu);cursor:pointer;font-size:11px;transition:.12s}
.hbtn:hover{background:var(--s2);color:var(--tx)}
.hbtn.rd:hover{border-color:var(--rd);color:var(--rd);background:rgba(248,81,73,.08)}

/* tab bar */
#tabbar{display:flex;align-items:stretch;background:var(--s1);border-bottom:1px solid var(--b1);flex-shrink:0;overflow-x:auto;min-height:33px}
#tabbar::-webkit-scrollbar{height:3px}
#tabbar::-webkit-scrollbar-thumb{background:var(--b2)}
.tab{display:flex;align-items:center;gap:6px;padding:0 13px 0 11px;cursor:pointer;border-right:1px solid var(--b1);color:var(--mu);font-size:11px;white-space:nowrap;flex-shrink:0;position:relative;transition:color .1s,background .1s;user-select:none}
.tab:hover{color:var(--tx);background:rgba(255,255,255,.03)}
.tab.active{color:var(--tx);background:var(--bg)}
.tab.active::after{content:'';position:absolute;bottom:0;left:0;right:0;height:2px;background:var(--ac)}
.ticon{font-size:10px;opacity:.55}
.tcl{display:flex;align-items:center;justify-content:center;width:15px;height:15px;border-radius:3px;border:none;background:transparent;cursor:pointer;color:var(--mu);font-size:9px;opacity:0;transition:opacity .1s,background .1s;padding:0;margin-left:1px}
.tab:hover .tcl,.tab.active .tcl{opacity:.55}
.tcl:hover{opacity:1!important;background:var(--b2);color:var(--tx)}

/* body */
#bd{flex:1;display:flex;overflow:hidden}

/* sidebar */
#sb{width:210px;min-width:210px;background:var(--s1);border-right:1px solid var(--b1);display:flex;flex-direction:column;flex-shrink:0}
.sb-hd{padding:7px 12px;font-size:10px;font-weight:700;text-transform:uppercase;letter-spacing:.08em;color:var(--mu);border-bottom:1px solid var(--b1);flex-shrink:0}
#tree{flex:1;overflow-y:auto;padding:4px 0}
#tree::-webkit-scrollbar{width:4px}
#tree::-webkit-scrollbar-thumb{background:var(--b2);border-radius:2px}
.ti{display:flex;align-items:center;gap:5px;padding:3px 10px;cursor:pointer;font-size:11px;color:var(--tx);user-select:none;white-space:nowrap;overflow:hidden;transition:background .08s}
.ti:hover{background:rgba(255,255,255,.05)}
.ti.hi{background:rgba(88,166,255,.1);color:var(--ac)}
.tarr{font-size:7px;opacity:.45;width:10px;flex-shrink:0;text-align:center}
.tico{opacity:.5;flex-shrink:0;font-size:10px}
.tlbl{overflow:hidden;text-overflow:ellipsis;flex:1}
.l1{padding-left:22px;color:var(--mu);font-size:10.5px}
.l2{padding-left:36px}

/* workspace */
#ws{flex:1;display:flex;flex-direction:column;overflow:hidden}

/* SQL pane */
#sqlp{flex:1;display:flex;flex-direction:column;overflow:hidden}
#sqltb{display:flex;align-items:center;gap:8px;padding:6px 10px;background:var(--s1);border-bottom:1px solid var(--b1);flex-shrink:0}
#dbsel{padding:4px 8px;border-radius:4px;border:1px solid var(--b2);background:var(--bg);color:var(--tx);outline:none;font-size:11px;max-width:200px}
.rnbtn{display:flex;align-items:center;gap:5px;padding:4px 14px;border-radius:4px;background:var(--ac);color:#0d1117;border:none;cursor:pointer;font-size:11px;font-weight:700;transition:opacity .12s}
.rnbtn:hover{opacity:.85}
.rnbtn:disabled{opacity:.4;cursor:default}
.hint{font-size:10px;color:var(--mu)}
#sqled{resize:vertical;padding:10px 13px;background:#090d13;color:#ccd6f6;border:none;border-bottom:1px solid var(--b1);outline:none;line-height:1.8;font-size:12px;overflow:auto;height:140px;min-height:50px;max-height:380px;width:100%;flex-shrink:0;caret-color:var(--ac)}
#sqlr{flex:1;overflow:hidden}

/* table pane */
#tblp{flex:1;display:none;flex-direction:column;overflow:hidden}
.stabs{display:flex;gap:3px;padding:5px 10px;background:var(--s1);border-bottom:1px solid var(--b1);flex-shrink:0}
.stab{padding:3px 14px;border-radius:4px;border:1px solid transparent;color:var(--mu);cursor:pointer;font-size:11px;background:transparent;transition:.1s}
.stab:hover{color:var(--tx)}
.stab.on{background:var(--bg);border-color:var(--b2);color:var(--tx)}
#tblv{flex:1;overflow:hidden}

/* data / grid */
.gw{display:flex;flex-direction:column;height:100%;overflow:hidden}
.gst{font-size:10px;padding:4px 12px;color:var(--mu);background:var(--s1);border-bottom:1px solid var(--b1);flex-shrink:0}
.gs{flex:1;overflow:auto}
.gs::-webkit-scrollbar{width:7px;height:7px}
.gs::-webkit-scrollbar-track{background:var(--bg)}
.gs::-webkit-scrollbar-thumb{background:var(--b2);border-radius:4px}
.gs::-webkit-scrollbar-corner{background:var(--bg)}
table.g{border-collapse:collapse;white-space:nowrap;min-width:100%;font-size:11.5px}
table.g th{padding:5px 12px;background:var(--s2);border-bottom:2px solid var(--b2);border-right:1px solid var(--b1);color:var(--mu);font-weight:600;position:sticky;top:0;z-index:1;font-size:10px;text-transform:uppercase;letter-spacing:.05em}
table.g td{padding:4px 12px;border-bottom:1px solid var(--b1);border-right:1px solid rgba(33,38,45,.5);max-width:320px;overflow:hidden;text-overflow:ellipsis}
table.g tr:hover td{background:rgba(255,255,255,.025)}
.vnull{color:var(--mu);font-style:italic;font-size:10.5px}

/* struct table */
table.st{border-collapse:collapse;width:100%;font-size:11px}
table.st th{padding:5px 13px;background:var(--s2);border-bottom:2px solid var(--b2);color:var(--mu);font-weight:600;font-size:10px;text-transform:uppercase;letter-spacing:.05em;position:sticky;top:0;z-index:1;text-align:left}
table.st td{padding:5px 13px;border-bottom:1px solid var(--b1)}
.pk{color:var(--or);font-size:9px;font-weight:700;letter-spacing:.05em}
.dt{color:var(--ac);font-size:10.5px}

/* misc */
.empty{padding:44px 24px;text-align:center;color:var(--mu);font-size:12px;line-height:2.2}
.err{margin:8px;padding:11px 14px;background:rgba(248,81,73,.08);border-left:3px solid var(--rd);color:var(--rd);font-size:11.5px;line-height:1.55;border-radius:0 4px 4px 0}
.spin{padding:24px;text-align:center;color:var(--mu);font-size:11px;animation:fade 1.4s ease-in-out infinite}
@keyframes fade{0%,100%{opacity:.35}50%{opacity:1}}

/* status bar */
#sbar{padding:3px 14px;background:var(--s1);border-top:1px solid var(--b1);font-size:10px;color:var(--mu);flex-shrink:0;display:flex;gap:18px}
</style>
</head>
<body>

<!-- ── Connect ── -->
<div id="cv">
  <div id="cf">
    <div class="cf-head">
      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4.03 3-9 3S3 13.66 3 12"/><path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"/></svg>
      <div><h2 id="cf-type"></h2><span id="cf-port"></span></div>
    </div>
    <label class="lbl">Usuario</label>
    <input class="inp" id="u" autocomplete="username">
    <label class="lbl">Contrase&#241;a</label>
    <input class="inp" id="pw" type="password" autocomplete="current-password" onkeydown="if(event.key==='Enter')connect()">
    <label class="chk-row"><input type="checkbox" id="rem"> Recordar credenciales</label>
    <div id="ce"></div>
    <button class="btn-p" id="cbtn" onclick="connect()">Conectar</button>
  </div>
</div>

<!-- ── Explorer ── -->
<div id="ev" style="display:none">
  <div id="hdr">
    <svg id="hico" width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><ellipse cx="12" cy="5" rx="9" ry="3"/><path d="M21 12c0 1.66-4.03 3-9 3S3 13.66 3 12"/><path d="M3 5v14c0 1.66 4.03 3 9 3s9-1.34 9-3V5"/></svg>
    <span id="htype"></span><span id="hport"></span><span id="hdb"></span>
    <div class="sp"></div>
    <button class="hbtn" onclick="loadDbs()" title="Recargar">&#x21BA;</button>
    <button class="hbtn rd" onclick="disconnect()">Desconectar</button>
  </div>

  <div id="tabbar"></div>

  <div id="bd">
    <div id="sb">
      <div class="sb-hd">Bases de datos</div>
      <div id="tree"></div>
    </div>
    <div id="ws">
      <!-- SQL pane (always mounted, hidden when table tab active) -->
      <div id="sqlp">
        <div id="sqltb">
          <select id="dbsel"><option value="">&#8212; base de datos &#8212;</option></select>
          <div class="sp"></div>
          <span class="hint">Ctrl+Enter</span>
          <button class="rnbtn" id="rbtn" onclick="runSql()">&#9654; Ejecutar</button>
        </div>
        <textarea id="sqled" spellcheck="false" onkeydown="sqlKey(event)" placeholder="-- Escribe tu SQL aqui&#10;-- SELECT * FROM tabla LIMIT 100"></textarea>
        <div id="sqlr"><div class="empty">Ejecuta una consulta para ver resultados</div></div>
      </div>
      <!-- Table pane (shown when table tab active) -->
      <div id="tblp">
        <div class="stabs">
          <button class="stab on" id="stab-data" onclick="setSubTab('data')">Datos</button>
          <button class="stab" id="stab-struct" onclick="setSubTab('struct')">Estructura</button>
        </div>
        <div id="tblv"><div class="empty">Doble clic en una tabla para abrirla</div></div>
      </div>
    </div>
  </div>
  <div id="sbar"><span id="st-rows"></span><span id="st-time"></span></div>
</div>

<script>
var SESSION=PLACEHOLDER_SESSION,DB_TYPE=PLACEHOLDER_DBTYPE,DB_PORT=PLACEHOLDER_PORT,API=PLACEHOLDER_API;
var connId=null,ntType='',curDb='';
var tdata={},expDbs={},expSchs={};

// ── Tab state ───────────────────────────────────────────────
var tabs=[{id:0,type:'sql',label:'SQL'}];
var nextId=1;
var activeTabId=0;
var tabCache={}; // id -> {db,schema,tbl,subTab:'data'|'struct',dataRes,structRes,loading,error}

// ── Helpers ─────────────────────────────────────────────────
function tc(t){var m={postgresql:'#58a6ff',timescaledb:'#58a6ff',mysql:'#f97316',mariadb:'#c084fc'};return m[(t||'').toLowerCase()]||'#58a6ff';}
function nt(n){var l=(n||'').toLowerCase();if(l.indexOf('postgres')>=0||l.indexOf('timescale')>=0)return'postgresql';if(l.indexOf('mariadb')>=0)return'mariadb';if(l.indexOf('mysql')>=0)return'mysql';return l;}
function esc(s){return String(s).replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');}
function apiFetch(path,opts){
  return fetch(API+path,Object.assign({headers:{'Content-Type':'application/json'}},opts||{}))
    .then(function(r){return r.ok?r.json():r.text().then(function(t){throw new Error(t);});});
}

// ── Init ────────────────────────────────────────────────────
window.onload=function(){
  ntType=nt(DB_TYPE);
  var color=tc(ntType);
  document.getElementById('cf-type').textContent=DB_TYPE;
  document.getElementById('cf-port').textContent=':'+DB_PORT;
  document.getElementById('cf').style.borderTop='3px solid '+color;
  document.getElementById('htype').textContent=DB_TYPE;
  document.getElementById('htype').style.color=color;
  document.getElementById('hport').textContent=':'+DB_PORT;
  document.getElementById('hico').style.stroke=color;
  apiFetch('/db-api/credentials?type='+encodeURIComponent(DB_TYPE)+'&port='+DB_PORT)
    .then(function(c){
      if(c&&c.user){document.getElementById('u').value=c.user;document.getElementById('rem').checked=true;}
      if(c&&c.password)document.getElementById('pw').value=c.password;
      if(!(c&&c.user))document.getElementById('u').value=ntType==='postgresql'?'postgres':'root';
    }).catch(function(){document.getElementById('u').value=ntType==='postgresql'?'postgres':'root';});
  document.getElementById('u').focus();
  renderTabs();
};

// ── Connect/disconnect ───────────────────────────────────────
function connect(){
  var btn=document.getElementById('cbtn'),ce=document.getElementById('ce');
  btn.disabled=true;btn.textContent='Conectando...';ce.style.display='none';
  apiFetch('/db-api/connect',{method:'POST',body:JSON.stringify({
    session:SESSION,dbType:DB_TYPE,port:DB_PORT,
    user:document.getElementById('u').value,password:document.getElementById('pw').value
  })}).then(function(r){
    connId=r.connId;
    if(!connId){ce.textContent='Error interno';ce.style.display='block';return;}
    if(document.getElementById('rem').checked){
      apiFetch('/db-api/credentials',{method:'POST',body:JSON.stringify({
        type:DB_TYPE,port:DB_PORT,user:document.getElementById('u').value,password:document.getElementById('pw').value
      })}).catch(function(){});
    } else {
      apiFetch('/db-api/credentials',{method:'DELETE',body:JSON.stringify({type:DB_TYPE,port:DB_PORT})}).catch(function(){});
    }
    document.getElementById('cv').style.display='none';
    document.getElementById('ev').style.display='flex';
    loadDbs();
  }).catch(function(e){
    ce.textContent=String(e).replace('Error: ','');ce.style.display='block';
  }).finally(function(){btn.disabled=false;btn.textContent='Conectar';});
}
function disconnect(){
  if(connId)apiFetch('/db-api/disconnect',{method:'POST',body:JSON.stringify({connId:connId})}).catch(function(){});
  window.close();
}

// ── Database tree ────────────────────────────────────────────
function loadDbs(){
  apiFetch('/db-api/databases?connId='+connId).then(function(dbs){
    tdata.dbs=dbs||[];renderTree();
    var sel=document.getElementById('dbsel');
    sel.innerHTML='<option value="">&#8212; base de datos &#8212;</option>';
    (dbs||[]).forEach(function(d){var o=document.createElement('option');o.value=d;o.textContent=d;sel.appendChild(o);});
    if(curDb)sel.value=curDb;
    sel.onchange=function(){curDb=sel.value;document.getElementById('hdb').textContent=curDb?'/'+curDb:'';};
  });
}
function toggleDb(db){
  expDbs[db]=!expDbs[db];
  if(expDbs[db]){
    if(ntType==='postgresql'&&!tdata['s_'+db]){
      apiFetch('/db-api/schemas?connId='+connId+'&db='+encodeURIComponent(db)).then(function(s){tdata['s_'+db]=s||[];renderTree();});
    } else if(ntType!=='postgresql'&&!tdata['t_'+db+'_']){
      apiFetch('/db-api/tables?connId='+connId+'&db='+encodeURIComponent(db)+'&schema=').then(function(t){tdata['t_'+db+'_']=t||[];renderTree();});
    }
  }
  renderTree();
}
function toggleSch(db,sch){
  var k=db+'|'+sch;
  expSchs[k]=!expSchs[k];
  if(expSchs[k]&&!tdata['t_'+db+'_'+sch]){
    apiFetch('/db-api/tables?connId='+connId+'&db='+encodeURIComponent(db)+'&schema='+encodeURIComponent(sch)).then(function(t){tdata['t_'+db+'_'+sch]=t||[];renderTree();});
  }
  renderTree();
}
function renderTree(){
  var el=document.getElementById('tree');
  var dbs=tdata.dbs||[];
  if(!dbs.length){el.innerHTML='<div class="ti" style="padding:16px 12px;color:var(--mu)">Sin bases de datos</div>';return;}
  var h='';
  dbs.forEach(function(db){
    var exp=expDbs[db];
    h+='<div class="ti" data-tdb="'+esc(db)+'" title="'+esc(db)+'"><span class="tarr">'+(exp?'&#9660;':'&#9658;')+'</span><span class="tico">&#10753;</span><span class="tlbl">'+esc(db)+'</span></div>';
    if(!exp)return;
    if(ntType==='postgresql'){
      (tdata['s_'+db]||[]).forEach(function(s){
        var k=db+'|'+s,sexp=expSchs[k];
        h+='<div class="ti l1" data-tsch="'+esc(db)+'|'+esc(s)+'" title="'+esc(s)+'"><span class="tarr">'+(sexp?'&#9660;':'&#9658;')+'</span><span class="tico" style="font-size:9px">&#11041;</span><span class="tlbl">'+esc(s)+'</span></div>';
        if(!sexp)return;
        (tdata['t_'+db+'_'+s]||[]).forEach(function(t){
          var isHi=isTabOpen(db,s,t);
          h+='<div class="ti l2'+(isHi?' hi':'')+'" data-ttbl="'+esc(db)+'|'+esc(s)+'|'+esc(t)+'" title="'+esc(t)+'"><span class="tico">&#9636;</span><span class="tlbl">'+esc(t)+'</span></div>';
        });
      });
    } else {
      (tdata['t_'+db+'_']||[]).forEach(function(t){
        var isHi=isTabOpen(db,'',t);
        h+='<div class="ti l2'+(isHi?' hi':'')+'" data-ttbl="'+esc(db)+'||'+esc(t)+'" title="'+esc(t)+'"><span class="tico">&#9636;</span><span class="tlbl">'+esc(t)+'</span></div>';
      });
    }
  });
  el.innerHTML=h;
}
function isTabOpen(db,schema,tbl){
  return tabs.some(function(tab){
    var c=tabCache[tab.id];
    return c&&c.db===db&&c.schema===schema&&c.tbl===tbl;
  });
}

// ── Tab management ───────────────────────────────────────────
function renderTabs(){
  var h='';
  tabs.forEach(function(tab){
    var isSql=tab.id===0;
    var active=tab.id===activeTabId;
    h+='<div class="tab'+(active?' active':'')+'" data-tabid="'+tab.id+'">';
    h+='<span class="ticon">'+(isSql?'&#9654;':'&#9636;')+'</span>';
    h+='<span>'+esc(tab.label)+'</span>';
    if(!isSql)h+='<button class="tcl" data-close="1" title="Cerrar">&#10005;</button>';
    h+='</div>';
  });
  document.getElementById('tabbar').innerHTML=h;
}
function activateTab(id){
  activeTabId=id;
  renderTabs();
  if(id===0){
    document.getElementById('sqlp').style.display='flex';
    document.getElementById('tblp').style.display='none';
  } else {
    document.getElementById('sqlp').style.display='none';
    document.getElementById('tblp').style.display='flex';
    var c=tabCache[id];
    if(c)renderTblPane(id,c);
  }
  renderTree();
}
function closeTab(id){
  var idx=tabs.findIndex(function(t){return t.id===id;});
  tabs.splice(idx,1);
  delete tabCache[id];
  if(activeTabId===id){
    var nid=tabs[Math.min(idx,tabs.length-1)].id;
    activateTab(nid);
  } else {
    renderTabs();
    renderTree();
  }
}
function openTableTab(db,schema,tbl){
  // find existing tab
  var existing=tabs.find(function(t){
    var c=tabCache[t.id];
    return c&&c.db===db&&c.schema===schema&&c.tbl===tbl;
  });
  if(existing){activateTab(existing.id);return;}
  var id=nextId++;
  var label=tbl.length>15?tbl.slice(0,13)+'…':tbl;
  tabs.push({id:id,type:'table',label:label});
  tabCache[id]={db:db,schema:schema,tbl:tbl,subTab:'data',dataRes:null,structRes:null,loading:true,error:null};
  activateTab(id);
  loadTabData(id);
}
function loadTabData(id){
  var c=tabCache[id];
  if(!c)return;
  document.getElementById('tblv').innerHTML='<div class="spin">Cargando…</div>';
  var db=c.db,sch=c.schema,tbl=c.tbl;
  var bt=String.fromCharCode(96);
  var q=sch?'"'+sch+'"."'+tbl+'"':bt+tbl+bt;
  Promise.all([
    apiFetch('/db-api/columns?connId='+connId+'&db='+encodeURIComponent(db)+'&schema='+encodeURIComponent(sch||'')+'&table='+encodeURIComponent(tbl)),
    apiFetch('/db-api/query',{method:'POST',body:JSON.stringify({connId:connId,db:db,sql:'SELECT * FROM '+q+' LIMIT 500'})})
  ]).then(function(rs){
    c.structRes=rs[0];
    c.dataRes=rs[1];
    c.loading=false;
    if(activeTabId===id)renderTblPane(id,c);
    renderTree();
  }).catch(function(e){
    c.loading=false;c.error=String(e).replace('Error: ','');
    if(activeTabId===id)renderTblPane(id,c);
  });
}
function renderTblPane(id,c){
  document.getElementById('stab-data').className='stab'+(c.subTab==='data'?' on':'');
  document.getElementById('stab-struct').className='stab'+(c.subTab==='struct'?' on':'');
  if(c.loading){document.getElementById('tblv').innerHTML='<div class="spin">Cargando…</div>';return;}
  if(c.error){document.getElementById('tblv').innerHTML='<div class="err">'+esc(c.error)+'</div>';return;}
  if(c.subTab==='data'){
    var r=c.dataRes;
    if(!r||r.error){document.getElementById('tblv').innerHTML='<div class="err">'+(r?esc(r.error):'Sin datos')+'</div>';return;}
    document.getElementById('st-rows').textContent=(r.rowCount||0)+' filas';
    document.getElementById('tblv').innerHTML=renderGrid(r);
  } else {
    var cols=c.structRes||[];
    if(!cols.length){document.getElementById('tblv').innerHTML='<div class="empty">Sin columnas</div>';return;}
    document.getElementById('tblv').innerHTML=renderStructTable(cols);
  }
}
function setSubTab(sub){
  if(activeTabId===0)return;
  var c=tabCache[activeTabId];
  if(!c)return;
  c.subTab=sub;
  renderTblPane(activeTabId,c);
}

// ── SQL execution ────────────────────────────────────────────
function runSql(){
  var sql=document.getElementById('sqled').value.trim();
  if(!sql||!connId)return;
  var db=document.getElementById('dbsel').value||'';
  if(!db&&ntType==='postgresql')db='postgres';
  var btn=document.getElementById('rbtn');
  btn.disabled=true;
  var t0=Date.now();
  document.getElementById('sqlr').innerHTML='<div class="spin">Ejecutando…</div>';
  apiFetch('/db-api/query',{method:'POST',body:JSON.stringify({connId:connId,db:db,sql:sql})})
    .then(function(r){
      var ms=Date.now()-t0;
      document.getElementById('st-time').textContent=ms+'ms';
      document.getElementById('st-rows').textContent=(r.rowCount||0)+' filas';
      if(r.error){document.getElementById('sqlr').innerHTML='<div class="err">'+esc(r.error)+'</div>';return;}
      document.getElementById('sqlr').innerHTML=(r.columns&&r.columns.length)?renderGrid(r):'<div class="empty">Consulta ejecutada. '+esc((r.rowCount||0)+' fila'+(r.rowCount===1?'':'s')+' afectada'+(r.rowCount===1?'':'s'))+'</div>';
    }).catch(function(e){
      document.getElementById('sqlr').innerHTML='<div class="err">'+esc(String(e).replace('Error: ',''))+'</div>';
    }).finally(function(){btn.disabled=false;});
}
function sqlKey(e){if((e.ctrlKey||e.metaKey)&&e.key==='Enter'){e.preventDefault();runSql();}}

// ── Render helpers ───────────────────────────────────────────
function renderGrid(r){
  if(!r.columns||!r.columns.length)return'<div class="empty">Sin columnas</div>';
  var h='<div class="gw"><div class="gst">'+esc(r.rowCount)+' fila'+(r.rowCount===1?'':'s')+'</div><div class="gs"><table class="g"><thead><tr>';
  r.columns.forEach(function(c){h+='<th>'+esc(c)+'</th>';});
  h+='</tr></thead><tbody>';
  (r.rows||[]).forEach(function(row){
    h+='<tr>';
    row.forEach(function(v){h+='<td>'+(v===''||v===null?'<span class="vnull">NULL</span>':esc(String(v)))+'</td>';});
    h+='</tr>';
  });
  h+='</tbody></table></div></div>';
  return h;
}
function renderStructTable(cols){
  var h='<div class="gw"><div class="gs"><table class="st"><thead><tr><th>Columna</th><th>Tipo</th><th>Nulo</th><th>Default</th><th>Clave</th></tr></thead><tbody>';
  cols.forEach(function(c){
    h+='<tr>';
    h+='<td>'+(c.key==='PK'?'<span class="pk">PK</span> ':'')+esc(c.name||'')+'</td>';
    h+='<td><span class="dt">'+esc(c.dataType||'')+'</span></td>';
    h+='<td style="color:var(--gr);font-size:11px">'+(c.nullable?'&#10003;':'')+'</td>';
    h+='<td style="color:var(--mu);font-size:10.5px">'+esc(c.default||'')+'</td>';
    h+='<td>'+(c.key?'<span class="pk">'+esc(c.key)+'</span>':'')+'</td>';
    h+='</tr>';
  });
  h+='</tbody></table></div></div>';
  return h;
}

// ── Event delegation ─────────────────────────────────────────
document.addEventListener('click',function(e){
  // Tab close button
  var closeBtn=e.target.closest('[data-close]');
  if(closeBtn){
    var tabEl=closeBtn.closest('[data-tabid]');
    if(tabEl){e.stopPropagation();closeTab(parseInt(tabEl.dataset.tabid));}
    return;
  }
  // Tab click
  var tabEl=e.target.closest('[data-tabid]');
  if(tabEl){activateTab(parseInt(tabEl.dataset.tabid));return;}
  // Tree item
  var ti=e.target.closest('.ti');
  if(!ti)return;
  if(ti.dataset.tdb){toggleDb(ti.dataset.tdb);return;}
  if(ti.dataset.tsch){var p=ti.dataset.tsch.split('|');toggleSch(p[0],p[1]);return;}
});
document.addEventListener('dblclick',function(e){
  var ti=e.target.closest('[data-ttbl]');
  if(!ti)return;
  var p=ti.dataset.ttbl.split('|');
  openTableTab(p[0],p[1]||'',p[2]!==undefined?p[2]:(p[1]||''));
});
</script>
</body>
</html>`
