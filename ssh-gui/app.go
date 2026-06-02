package main

import (
	"fmt"
	"context"
	"time"
	"io"
	"os"
	"net"
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

	// Comando óptimo que formatea el output nativo a filas JSON procesables
	output, err := cmdSession.Output("docker ps -a --format '{\"id\":\"{{.ID}}\",\"names\":\"{{.Names}}\",\"image\":\"{{.Image}}\",\"status\":\"{{.Status}}\",\"state\":\"{{.State}}\"}'")
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
		// Deserializar línea o mapearla manualmente de forma segura
		// Nota: En entornos de desarrollo si el string falla, puedes parsearlo usando json.Unmarshal
		var c models.DockerContainer
		// Implementación de parseo rápido basado en el formato JSON inyectado:
		c.ID = extractJSONField(line, "id")
		c.Names = extractJSONField(line, "names")
		c.Image = extractJSONField(line, "image")
		c.Status = extractJSONField(line, "status")
		c.State = extractJSONField(line, "state")
		
		if c.ID != "" {
			containers = append(containers, c)
		}
	}

	return containers, nil
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