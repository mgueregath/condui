// Printer redirection device (MS-RDPEPC).
// Print jobs are accumulated in memory and written to local files on close.
// Optionally, jobs are submitted to a network printer via IPP (RFC 8010/8011).

package rdpdr

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// PrinterDevice represents a redirected printer.
type PrinterDevice struct {
	id           uint32
	name         string // Printer name visible on server
	driverName   string // Windows driver name
	outputDir    string // Directory to save print jobs (empty = no file output)
	ippURL       string // IPP printer URL (empty = no IPP submission)
	isDefault    bool   // Announce as default printer
	log          *slog.Logger
	mu           sync.Mutex
	nextID       uint32
	jobs         map[uint32]*printJob
	ippReqID     atomic.Uint32
	cachedConfig []byte // CachedPrinterConfigData from server (MS-RDPEPC 2.2.2.3)
}

type printJob struct {
	buf  []byte
	path string
}

// NewPrinterDevice creates a new printer device.
func NewPrinterDevice(id uint32, name, driverName, outputDir, ippURL string, isDefault bool, log *slog.Logger) *PrinterDevice {
	if driverName == "" {
		driverName = "MS Publisher Imagesetter"
	}
	return &PrinterDevice{
		id:         id,
		name:       name,
		driverName: driverName,
		outputDir:  outputDir,
		ippURL:     ippURL,
		isDefault:  isDefault,
		log:        log.With("device", name),
		jobs:       make(map[uint32]*printJob),
	}
}

// ID returns the device ID.
func (p *PrinterDevice) ID() uint32 { return p.id }

// Type returns DeviceTypePrinter.
func (p *PrinterDevice) Type() uint32 { return DeviceTypePrinter }

// Name returns the printer display name.
func (p *PrinterDevice) Name() string { return p.name }

// DeviceData encodes the printer announce data per MS-RDPEPC 2.2.2.1.
// Format: Flags(4) + CodePage(4) + PnpNameLen(4) + DriverNameLen(4) +
//
//	PrinterNameLen(4) + CachedFieldsLen(4) + DriverName(UTF-16LE) + PrinterName(UTF-16LE)
func (p *PrinterDevice) DeviceData() []byte {
	driverUTF16 := encodeUTF16LE(p.driverName)
	driverUTF16 = append(driverUTF16, 0, 0) // null terminator
	nameUTF16 := encodeUTF16LE(p.name)
	nameUTF16 = append(nameUTF16, 0, 0) // null terminator

	// MS-RDPEPC 2.2.2.1 Printer Announce Flags (no TSPRINTER, no ASCII)
	var flags uint32
	if p.isDefault {
		flags = 0x00000002 // RDPDR_PRINTER_ANNOUNCE_FLAG_DEFAULTPRINTER
	}

	p.mu.Lock()
	cachedConfig := p.cachedConfig
	p.mu.Unlock()

	hdr := 24 // 6 * uint32
	buf := make([]byte, hdr+len(driverUTF16)+len(nameUTF16)+len(cachedConfig))
	binary.LittleEndian.PutUint32(buf[0:4], flags)
	// CodePage = 0 (buf[4:8])
	// PnpNameLen = 0 (buf[8:12]) — no PnP name
	binary.LittleEndian.PutUint32(buf[12:16], uint32(len(driverUTF16)))
	binary.LittleEndian.PutUint32(buf[16:20], uint32(len(nameUTF16)))
	binary.LittleEndian.PutUint32(buf[20:24], uint32(len(cachedConfig)))
	copy(buf[hdr:], driverUTF16)
	copy(buf[hdr+len(driverUTF16):], nameUTF16)
	copy(buf[hdr+len(driverUTF16)+len(nameUTF16):], cachedConfig)

	return buf
}

// HandleIRP dispatches an I/O request to the appropriate handler.
func (p *PrinterDevice) HandleIRP(h *Handler, req *IORequest) {
	switch req.MajorFn {
	case IrpCreate:
		p.handleCreate(h, req)
	case IrpWrite:
		p.handleWrite(h, req)
	case IrpClose:
		p.handleClose(h, req)
	case IrpDeviceControl:
		// No IOCTLs — always return success with empty output
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// handleCreate allocates a new print job and returns a FileID.
func (p *PrinterDevice) handleCreate(h *Handler, req *IORequest) {
	p.mu.Lock()
	// Cap concurrent print jobs to prevent unbounded memory growth.
	const maxPrintJobs = 256
	if len(p.jobs) >= maxPrintJobs {
		p.mu.Unlock()
		p.log.LogAttrs(context.Background(), slog.LevelWarn, "print job limit reached", slog.Int("limit", maxPrintJobs))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusTooManyOpenedFiles, nil)
		return
	}
	fileID := p.nextID
	p.nextID++
	var path string
	if p.outputDir != "" {
		ts := time.Now().Format("20060102_150405")
		path = filepath.Join(p.outputDir, fmt.Sprintf("%s_%s_%03d.prn", p.name, ts, fileID))
	}
	p.jobs[fileID] = &printJob{path: path}
	p.mu.Unlock()

	p.log.LogAttrs(context.Background(), slog.LevelInfo, "print job created",
		slog.Int("fileID", int(fileID)), slog.String("path", path))

	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], fileID)
	out[4] = FileCreated
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// handleWrite accumulates data into the print job buffer.
func (p *PrinterDevice) handleWrite(h *Handler, req *IORequest) {
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	// offset at Payload[4:12] (uint64)
	data := req.Payload[32:]
	if uint32(len(data)) < length {
		length = uint32(len(data))
	}

	p.mu.Lock()
	job, ok := p.jobs[req.FileID]
	if ok {
		job.buf = append(job.buf, data[:length]...)
	}
	p.mu.Unlock()

	if !ok {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidDeviceRequest, nil)
		return
	}

	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], length)
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// handleClose writes the accumulated print job to disk and/or submits via IPP.
func (p *PrinterDevice) handleClose(h *Handler, req *IORequest) {
	p.mu.Lock()
	job, ok := p.jobs[req.FileID]
	delete(p.jobs, req.FileID)
	p.mu.Unlock()

	if ok && len(job.buf) > 0 {
		if job.path != "" {
			if err := os.WriteFile(job.path, job.buf, 0644); err != nil {
				p.log.LogAttrs(context.Background(), slog.LevelError, "print job write failed",
					slog.String("path", job.path), slog.Any("err", err))
			} else {
				p.log.LogAttrs(context.Background(), slog.LevelInfo, "print job saved",
					slog.String("path", job.path), slog.Int("bytes", len(job.buf)))
			}
		}
		if p.ippURL != "" {
			if err := p.submitIPP(job); err != nil {
				p.log.LogAttrs(context.Background(), slog.LevelError, "IPP submit failed",
					slog.String("url", p.ippURL), slog.Any("err", err))
			} else {
				p.log.LogAttrs(context.Background(), slog.LevelInfo, "IPP job submitted",
					slog.String("url", p.ippURL), slog.Int("bytes", len(job.buf)))
			}
		}
	}

	var pad [5]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, pad[:])
}

// submitIPP sends an IPP Print-Job request (RFC 8010) to the configured URL.
func (p *PrinterDevice) submitIPP(job *printJob) error {
	reqID := p.ippReqID.Add(1)

	// Build IPP request body: attributes header + document data
	var buf bytes.Buffer
	buf.Grow(256 + len(job.buf))

	// Version 2.0
	buf.WriteByte(0x02)
	buf.WriteByte(0x00)
	// Operation: Print-Job (0x0002)
	buf.WriteByte(0x00)
	buf.WriteByte(0x02)
	// Request ID
	var rid [4]byte
	binary.BigEndian.PutUint32(rid[:], reqID)
	buf.Write(rid[:])

	// Operation attributes group tag
	buf.WriteByte(0x01)

	// attributes-charset = utf-8
	ippWriteAttr(&buf, 0x47, "attributes-charset", "utf-8")
	// attributes-natural-language = en
	ippWriteAttr(&buf, 0x48, "attributes-natural-language", "en")
	// printer-uri
	ippWriteAttr(&buf, 0x45, "printer-uri", p.ippURL)
	// document-format
	ippWriteAttr(&buf, 0x49, "document-format", "application/octet-stream")
	// job-name
	ts := time.Now().Format("20060102_150405")
	jobName := fmt.Sprintf("%s_%s", p.name, ts)
	ippWriteAttr(&buf, 0x42, "job-name", jobName)

	// End of attributes
	buf.WriteByte(0x03)

	// Document data
	buf.Write(job.buf)

	// Convert ipp:// to http:// for net/http
	httpURL := p.ippURL
	if strings.HasPrefix(httpURL, "ipp://") {
		httpURL = "http://" + httpURL[6:]
	}

	req, err := http.NewRequest("POST", httpURL, &buf)
	if err != nil {
		return fmt.Errorf("ipp: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/ipp")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("ipp: post: %w", err)
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ipp: server returned %s", resp.Status)
	}
	return nil
}

// ippWriteAttr writes a single IPP attribute: tag(1) + nameLen(2) + name + valueLen(2) + value.
func ippWriteAttr(buf *bytes.Buffer, tag byte, name, value string) {
	buf.WriteByte(tag)
	var lenBuf [2]byte
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(name)))
	buf.Write(lenBuf[:])
	buf.WriteString(name)
	binary.BigEndian.PutUint16(lenBuf[:], uint16(len(value)))
	buf.Write(lenBuf[:])
	buf.WriteString(value)
}
