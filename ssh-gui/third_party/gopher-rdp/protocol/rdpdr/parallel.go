// Parallel port redirection device (MS-RDPESP).
// Much simpler than serial — no IOCTL processing needed.

package rdpdr

import (
	"log/slog"
	"sync"
)

// ParallelDevice represents a redirected parallel port.
type ParallelDevice struct {
	id   uint32
	name string // e.g. "LPT1"
	path string // e.g. "/dev/lp0" or "LPT1"
	log  *slog.Logger
	mu   sync.Mutex
	fd   uintptr // platform handle, invalidPortHandle when closed
}

// NewParallelDevice creates a new parallel port device.
func NewParallelDevice(id uint32, name, path string, log *slog.Logger) *ParallelDevice {
	return &ParallelDevice{
		id:   id,
		name: name,
		path: path,
		log:  log.With("device", name),
		fd:   invalidPortHandle,
	}
}

// ID returns the device ID.
func (p *ParallelDevice) ID() uint32 { return p.id }

// Type returns DeviceTypeParallel.
func (p *ParallelDevice) Type() uint32 { return DeviceTypeParallel }

// Name returns the device display name.
func (p *ParallelDevice) Name() string { return p.name }

// HandleIRP dispatches an I/O request to the appropriate handler.
func (p *ParallelDevice) HandleIRP(h *Handler, req *IORequest) {
	switch req.MajorFn {
	case IrpCreate:
		p.handleCreate(h, req)
	case IrpClose:
		p.handleClose(h, req)
	case IrpRead:
		p.handleRead(h, req)
	case IrpWrite:
		p.handleWrite(h, req)
	case IrpDeviceControl:
		// No IOCTLs — always return success with empty output
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}
