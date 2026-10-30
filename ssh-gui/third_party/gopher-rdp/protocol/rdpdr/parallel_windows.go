//go:build windows

package rdpdr

import (
	"context"
	"encoding/binary"
	"log/slog"
	"strings"
	"syscall"
)

// handleCreate opens the parallel device.
func (p *ParallelDevice) handleCreate(h *Handler, req *IORequest) {
	var createOut [5]byte

	p.mu.Lock()
	if p.fd != invalidPortHandle {
		binary.LittleEndian.PutUint32(createOut[0:4], p.id)
		createOut[4] = FileOpened
		p.mu.Unlock()
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
		return
	}
	p.mu.Unlock()

	// Prepend \\.\  for device path if not already present
	path := p.path
	if !strings.HasPrefix(path, `\\.\`) {
		path = `\\.\` + path
	}

	pathp, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		p.log.LogAttrs(context.Background(), slog.LevelError, "parallel path encode failed",
			slog.String("path", p.path), slog.Any("err", err))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, createOut[:])
		return
	}

	handle, err := syscall.CreateFile(pathp,
		syscall.GENERIC_READ|syscall.GENERIC_WRITE,
		0, nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		p.log.LogAttrs(context.Background(), slog.LevelError, "parallel open failed",
			slog.String("path", p.path), slog.Any("err", err))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, createOut[:])
		return
	}

	p.mu.Lock()
	p.fd = uintptr(handle)
	p.mu.Unlock()

	p.log.LogAttrs(context.Background(), slog.LevelInfo, "parallel opened",
		slog.String("path", p.path))

	binary.LittleEndian.PutUint32(createOut[0:4], p.id)
	createOut[4] = FileOpened
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
}

// handleClose closes the parallel device.
func (p *ParallelDevice) handleClose(h *Handler, req *IORequest) {
	p.mu.Lock()
	fd := p.fd
	p.fd = invalidPortHandle
	p.mu.Unlock()

	if fd != invalidPortHandle {
		syscall.CloseHandle(syscall.Handle(fd))
		p.log.LogAttrs(context.Background(), slog.LevelInfo, "parallel closed")
	}

	var pad [5]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, pad[:])
}

// handleRead reads data from the parallel port.
func (p *ParallelDevice) handleRead(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])

	p.mu.Lock()
	fd := p.fd
	p.mu.Unlock()

	if fd == invalidPortHandle {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	buf := make([]byte, 4+length)
	var n uint32
	err := syscall.ReadFile(syscall.Handle(fd), buf[4:], &n, nil)
	if err != nil && n == 0 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, nil)
		return
	}

	binary.LittleEndian.PutUint32(buf[0:4], n)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, buf[:4+n])
}

// handleWrite writes data to the parallel port in a loop until all data is written.
func (p *ParallelDevice) handleWrite(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	data := req.Payload[32:]
	if uint32(len(data)) < length {
		length = uint32(len(data))
	}

	p.mu.Lock()
	fd := p.fd
	p.mu.Unlock()

	if fd == invalidPortHandle {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	handle := syscall.Handle(fd)
	total := uint32(0)
	remaining := data[:length]
	for len(remaining) > 0 {
		var n uint32
		err := syscall.WriteFile(handle, remaining, &n, nil)
		if err != nil {
			if total == 0 {
				h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusUnsuccessful, nil)
				return
			}
			break
		}
		total += n
		remaining = remaining[n:]
	}

	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], total)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}
