package models

import (
	"context"
	"fmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type ProgressWriter struct {
	Total       int64           `json:"total"`
	Transferred int64           `json:"transferred"`
	ID          string          `json:"id"`
	FileName    string          `json:"fileName"`
	Direction   string          `json:"direction"`
	AppCtx      context.Context `json:"-"` // Evita que Wails intente exportar el contexto a TS
}

// El método Write DEBE estar aquí, en el paquete models
func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Transferred += int64(n)
	percentage := int64(0)
	if pw.Total > 0 {
		percentage = (pw.Transferred * 100) / pw.Total
	}

	// Emitir estado en tiempo real hacia el Frontend
	runtime.EventsEmit(pw.AppCtx, "transfer-status", map[string]any{
		"id":        pw.ID,
		"name":      pw.FileName,
		"progress":  percentage,
		"size":      fmt.Sprintf("%.2f MB", float64(pw.Total)/(1024*1024)),
		"status":    "active",
		"direction": pw.Direction,
	})
	return n, nil
}