package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"ssh-gui/backend/models"
)

// CancelTransfer requests cancellation without interrupting other transfers.
func (a *App) CancelTransfer(id string) error {
	if value, ok := a.activeTransfers.Load(id); ok {
		value.(context.CancelFunc)()
	}
	return nil
}

func (a *App) runFileTransfer(fileName, direction string, transfer func(context.Context, func(int64) io.Writer) error) error {
	id := uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	a.activeTransfers.Store(id, cancel)
	defer a.activeTransfers.Delete(id)
	defer cancel()
	progress := &models.ProgressWriter{
		ID: id, FileName: fileName, Direction: direction, StartedAt: time.Now(),
	}
	start, failure, success := "Iniciando subida de: ", "Error al subir ", "Subida completada: "
	if direction == "download" {
		start, failure, success = "Iniciando descarga de: ", "Error al descargar ", "Descarga completada: "
	}
	a.emitLog("SFTP", start+fileName, "")
	progress.Emit("active")
	err := transfer(ctx, func(total int64) io.Writer {
		progress.Total = total
		progress.Emit("active")
		return progress
	})
	status := "done"
	if err != nil {
		status = "error"
		if errors.Is(err, context.Canceled) {
			status = "cancelled"
		}
	}
	progress.Emit(status)
	if err != nil {
		if status == "cancelled" {
			a.emitLog("SFTP", "Transferencia cancelada: "+fileName, "")
			return fmt.Errorf("transferencia cancelada: %w", err)
		}
		a.emitLog("SFTP", failure+fileName, "error")
		return err
	}
	a.emitLog("SFTP", success+fileName, "success")
	return nil
}
