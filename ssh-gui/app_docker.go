package main

import (
	"bufio"
	"fmt"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"ssh-gui/backend/dockerinfo"
	"ssh-gui/backend/models"
)

func (a *App) GetDockerContainers(sessionID string) ([]models.DockerContainer, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetContainers(session.Client)
}

func (a *App) GetListeningPorts(sessionID string) ([]models.PortInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetListeningPorts(session.Client)
}

// ToggleContainer ejecuta las acciones vitales: start, stop o restart
func (a *App) ToggleContainer(sessionID string, containerID string, action string) (string, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return "", fmt.Errorf("session not found")
	}

	output, err := dockerinfo.ToggleContainer(session.Client, containerID, action)
	if err == nil {
		a.emitLog("DOCKER", fmt.Sprintf("Contenedor %s ejecutó: %s", containerID, action), "success")
	} else {
		a.emitLog("DOCKER", fmt.Sprintf("Error en contenedor %s: %s", containerID, output), "error")
	}
	return output, err
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

func (a *App) GetDatabases(sessionID string) ([]models.DatabaseInfo, error) {
	session, ok := a.sessionManager.Get(sessionID)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return dockerinfo.GetDatabases(session.Client)
}

type SystemStats struct {
	CPUPercent   float64 `json:"cpuPercent"`
	MemUsedGB    float64 `json:"memUsedGB"`
	MemFreeGB    float64 `json:"memFreeGB"`
	MemTotalGB   float64 `json:"memTotalGB"`
	DiskUsedGB   float64 `json:"diskUsedGB"`
	DiskFreeGB   float64 `json:"diskFreeGB"`
	DiskTotalGB  float64 `json:"diskTotalGB"`
	UptimeSecs   float64 `json:"uptimeSecs"`
	NetRxBps     float64 `json:"netRxBps"`
	NetTxBps     float64 `json:"netTxBps"`
	DiskReadBps  float64 `json:"diskReadBps"`
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
	if rx := netRx2 - netRx1; rx > 0 {
		stats.NetRxBps = rx
	}
	if tx := netTx2 - netTx1; tx > 0 {
		stats.NetTxBps = tx
	}

	// Tasas de disco: sectores/s × 512 bytes/sector
	var diskR1, diskW1, diskR2, diskW2 float64
	fmt.Sscanf(strings.TrimSpace(lines[5]), "%f %f", &diskR1, &diskW1)
	fmt.Sscanf(strings.TrimSpace(lines[7]), "%f %f", &diskR2, &diskW2)
	if r := (diskR2 - diskR1) * 512; r > 0 {
		stats.DiskReadBps = r
	}
	if w := (diskW2 - diskW1) * 512; w > 0 {
		stats.DiskWriteBps = w
	}

	return stats, nil
}
