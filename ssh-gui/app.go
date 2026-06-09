package main

import (
	"fmt"
	"bufio"
	"context"
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