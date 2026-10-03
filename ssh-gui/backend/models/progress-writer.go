package models

import (
	"fmt"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type ProgressWriter struct {
	Total       int64
	Transferred int64
	ID          string
	FileName    string
	Direction   string
	StartedAt   time.Time
	lastEmit    time.Time
}

func (pw *ProgressWriter) Emit(status string) {
	now := time.Now()
	if pw.StartedAt.IsZero() {
		pw.StartedAt = now
	}
	elapsed := now.Sub(pw.StartedAt).Seconds()
	speed := float64(0)
	if elapsed > 0 {
		speed = float64(pw.Transferred) / elapsed
	}
	percentage := int64(0)
	if pw.Total > 0 {
		percentage = pw.Transferred * 100 / pw.Total
	}
	if percentage > 99 && status != "done" {
		percentage = 99
	}
	if status == "done" {
		percentage = 100
	}
	application.Get().Event.Emit("transfer-status", map[string]any{
		"id":             pw.ID,
		"name":           pw.FileName,
		"progress":       percentage,
		"size":           fmt.Sprintf("%.2f MiB", float64(pw.Total)/(1024*1024)),
		"status":         status,
		"direction":      pw.Direction,
		"transferred":    pw.Transferred,
		"total":          pw.Total,
		"bytesPerSecond": speed,
		"elapsedSeconds": elapsed,
	})
	pw.lastEmit = now
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	pw.Transferred += int64(len(p))
	if time.Since(pw.lastEmit) >= 200*time.Millisecond {
		pw.Emit("active")
	}
	return len(p), nil
}
