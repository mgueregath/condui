package rdpdr

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Windows FILETIME epoch: January 1, 1601 UTC
// Difference between Unix epoch (1970-01-01) and Windows epoch (1601-01-01) in 100ns intervals.
const windowsEpochDiff = 116444736000000000

// DiskDevice represents a redirected local directory.
type DiskDevice struct {
	id       uint32
	name     string
	root     string // cleaned absolute path
	readOnly bool
	log      *slog.Logger

	mu      sync.Mutex
	files   map[uint32]*openFile
	nextID  uint32
	readBuf []byte // reusable read buffer (avoids per-read allocation)
}

// openFile tracks an open file handle.
type openFile struct {
	f             *os.File
	path          string
	isDir         bool
	deleteOnClose bool

	// Directory enumeration state
	dirEntries []fs.DirEntry
	dirIndex   int
	dirPattern string
	dirStarted bool
}

// ID returns the device ID.
func (d *DiskDevice) ID() uint32 { return d.id }

// Type returns DeviceTypeDisk.
func (d *DiskDevice) Type() uint32 { return DeviceTypeDisk }

// Name returns the device display name.
func (d *DiskDevice) Name() string { return d.name }

// HandleIRP dispatches an I/O request to the appropriate handler.
func (d *DiskDevice) HandleIRP(h *Handler, req *IORequest) {
	switch req.MajorFn {
	case IrpCreate:
		d.handleCreate(h, req)
	case IrpClose:
		d.handleClose(h, req)
	case IrpRead:
		d.handleRead(h, req)
	case IrpWrite:
		d.handleWrite(h, req)
	case IrpQueryInfo:
		d.handleQueryInfo(h, req)
	case IrpSetInfo:
		d.handleSetInfo(h, req)
	case IrpQueryVolume:
		d.handleQueryVolumeInfo(h, req)
	case IrpDirControl:
		switch req.MinorFn {
		case IrpMnQueryDir:
			d.handleQueryDirectory(h, req)
		case IrpMnNotifyDir:
			var notifyOut [4]byte
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, notifyOut[:])
		default:
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
		}
	case IrpLockControl:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, nil)
	case IrpDeviceControl:
		var ioCtl uint32
		if len(req.Payload) >= 12 {
			ioCtl = binary.LittleEndian.Uint32(req.Payload[8:12])
			d.log.LogAttrs(context.Background(), slog.LevelDebug, "DeviceControl",
				slog.String("ioctl", fmt.Sprintf("0x%X", ioCtl)),
				slog.Int("fileID", int(req.FileID)))
		}
		var ctlOut [4]byte // OutputBufferLength = 0
		switch ioCtl {
		case 0x900A8: // FSCTL_GET_REPARSE_POINT — not a reparse point
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotAReparsePoint, ctlOut[:])
		default:
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, ctlOut[:])
		}
	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// NewDiskDevice creates a new disk device.
func NewDiskDevice(id uint32, name, localPath string, readOnly bool, log *slog.Logger) *DiskDevice {
	root, _ := filepath.Abs(filepath.Clean(localPath))
	// Resolve symlinks/junctions so the prefix check uses the real path.
	// This prevents symlinks under root from escaping the share boundary.
	if real, err := filepath.EvalSymlinks(root); err == nil {
		root = real
	}
	return &DiskDevice{
		id:       id,
		name:     name,
		root:     root,
		readOnly: readOnly,
		log:      log.With("device", name),
		files:    make(map[uint32]*openFile),
		nextID:   1,
	}
}

// resolvePath maps an RDP-style path (backslash-separated) to a local absolute path.
// Returns the resolved path and true if it stays within root; empty string and false otherwise.
// Symlinks, junctions, and mount points are resolved to their real paths to prevent
// a symlink under root from escaping the share boundary.
func (d *DiskDevice) resolvePath(rdpPath string) (string, bool) {
	// Convert backslashes to forward slashes
	p := strings.ReplaceAll(rdpPath, "\\", "/")
	// Strip leading slash
	p = strings.TrimLeft(p, "/")
	// Clean the path
	p = filepath.Clean(p)

	// Reject any path that attempts to escape root via ..
	if p == ".." || strings.HasPrefix(p, "../") || strings.Contains(p, "/../") {
		return "", false
	}

	full := filepath.Join(d.root, p)

	// Resolve symlinks/junctions to get the real path, then verify it
	// stays within root. If the target doesn't exist yet (create), resolve
	// the parent directory instead.
	real, err := filepath.EvalSymlinks(full)
	if err != nil {
		// Target doesn't exist — resolve parent to catch symlinked directories
		parent := filepath.Dir(full)
		realParent, perr := filepath.EvalSymlinks(parent)
		if perr != nil {
			// Parent doesn't exist either; fall back to lexical check
			if !hasPathPrefix(full, d.root) {
				return "", false
			}
			return full, true
		}
		realFull := filepath.Join(realParent, filepath.Base(full))
		if !hasPathPrefix(realFull, d.root) {
			return "", false
		}
		return full, true
	}

	if !hasPathPrefix(real, d.root) {
		return "", false
	}
	return full, true
}

// allocFileID returns the next available file handle ID.
// Caller must hold d.mu.
func (d *DiskDevice) allocFileID() uint32 {
	id := d.nextID
	d.nextID++
	return id
}

// handleCreate processes IRP_MJ_CREATE.
func (d *DiskDevice) handleCreate(h *Handler, req *IORequest) {
	// DR_CREATE_RSP always requires FileId(4) + Information(1) even on error
	var createOut [5]byte

	// MS-RDPEFS 2.2.1.4.1: DR_CREATE_REQ
	// desiredAccess(4) + allocationSize(8) + fileAttributes(4) + sharedAccess(4) +
	// createDisposition(4) + createOptions(4) + pathLength(4) + path(variable)
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, createOut[:])
		return
	}

	desiredAccess := binary.LittleEndian.Uint32(req.Payload[0:4])
	// allocationSize at [4:12]
	// fileAttributes at [12:16]
	// sharedAccess at [16:20]
	createDisposition := binary.LittleEndian.Uint32(req.Payload[20:24])
	createOptions := binary.LittleEndian.Uint32(req.Payload[24:28])
	pathLen := binary.LittleEndian.Uint32(req.Payload[28:32])

	if uint32(len(req.Payload)) < 32+pathLen {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, createOut[:])
		return
	}

	rdpPath := decodeUTF16LE(req.Payload[32 : 32+pathLen])

	d.log.LogAttrs(context.Background(), slog.LevelDebug, "Create",
		slog.String("path", rdpPath),
		slog.String("desiredAccess", fmt.Sprintf("0x%X", desiredAccess)),
		slog.String("disposition", fmt.Sprintf("0x%X", createDisposition)),
		slog.String("options", fmt.Sprintf("0x%X", createOptions)),
		slog.Bool("deleteOnClose", createOptions&FileDeleteOnClose != 0))

	// Resolve path safely
	localPath, ok := d.resolvePath(rdpPath)
	if !ok {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, createOut[:])
		return
	}

	isDir := createOptions&FileDirectoryFile != 0
	deleteOnClose := createOptions&FileDeleteOnClose != 0

	// When neither FILE_DIRECTORY_FILE nor FILE_NON_DIRECTORY_FILE is set,
	// detect the actual type from the filesystem (MS-FSCC behavior).
	if !isDir && createOptions&FileNonDirectoryFile == 0 {
		if fi, serr := os.Stat(localPath); serr == nil && fi.IsDir() {
			isDir = true
		}
	}

	// FILE_NON_DIRECTORY_FILE on a directory must fail with STATUS_FILE_IS_A_DIRECTORY.
	if createOptions&FileNonDirectoryFile != 0 {
		if fi, serr := os.Stat(localPath); serr == nil && fi.IsDir() {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusFileIsADirectory, createOut[:])
			return
		}
	}

	var f *os.File
	var info uint8
	var err error

	if isDir {
		info, err = d.createDirectory(localPath, createDisposition)
		if err == nil {
			f, err = os.Open(localPath)
		}
	} else {
		f, info, err = d.createFile(localPath, createDisposition)
	}

	if err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), createOut[:])
		return
	}

	d.mu.Lock()
	// Cap open file handles to prevent fd exhaustion from a buggy/malicious server.
	const maxOpenFiles = 4096
	if len(d.files) >= maxOpenFiles {
		d.mu.Unlock()
		f.Close()
		d.log.LogAttrs(context.Background(), slog.LevelWarn, "open file limit reached", slog.Int("limit", maxOpenFiles))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusTooManyOpenedFiles, createOut[:])
		return
	}
	fileID := d.allocFileID()
	d.files[fileID] = &openFile{
		f:             f,
		path:          localPath,
		isDir:         isDir,
		deleteOnClose: deleteOnClose,
	}
	d.mu.Unlock()

	// Response: fileID(4) + information(1)
	binary.LittleEndian.PutUint32(createOut[0:4], fileID)
	createOut[4] = info
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, createOut[:])
}

// createDirectory handles directory creation for IRP_MJ_CREATE.
func (d *DiskDevice) createDirectory(path string, disposition uint32) (uint8, error) {
	switch disposition {
	case FileOpen:
		fi, err := os.Stat(path)
		if err != nil {
			return 0, err
		}
		if !fi.IsDir() {
			return 0, &os.PathError{Op: "open", Path: path, Err: errors.New("not a directory")}
		}
		return FileOpened, nil
	case FileCreate:
		if d.readOnly {
			return 0, os.ErrPermission
		}
		if err := os.Mkdir(path, 0755); err != nil {
			return 0, err
		}
		return FileCreated, nil
	case FileOpenIf:
		fi, err := os.Stat(path)
		if err == nil && fi.IsDir() {
			return FileOpened, nil
		}
		if d.readOnly {
			return 0, os.ErrPermission
		}
		if err := os.MkdirAll(path, 0755); err != nil {
			return 0, err
		}
		return FileCreated, nil
	default:
		return FileOpened, nil
	}
}

// createFile handles file creation for IRP_MJ_CREATE.
func (d *DiskDevice) createFile(path string, disposition uint32) (*os.File, uint8, error) {
	switch disposition {
	case FileOpen:
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			// Try read-only
			f, err = os.Open(path)
			if err != nil {
				return nil, 0, err
			}
		}
		return f, FileOpened, nil
	case FileCreate:
		if d.readOnly {
			return nil, 0, os.ErrPermission
		}
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return nil, 0, err
		}
		return f, FileCreated, nil
	case FileOpenIf:
		if d.readOnly {
			f, err := os.Open(path)
			if err != nil {
				return nil, 0, os.ErrPermission
			}
			return f, FileOpened, nil
		}
		_, err := os.Stat(path)
		if err == nil {
			f, err := os.OpenFile(path, os.O_RDWR, 0)
			if err != nil {
				return nil, 0, err
			}
			return f, FileOpened, nil
		}
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, 0, err
		}
		return f, FileCreated, nil
	case FileOverwrite, FileOverwriteIf:
		if d.readOnly {
			return nil, 0, os.ErrPermission
		}
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return nil, 0, err
		}
		return f, FileOverwritten, nil
	case FileSupersede:
		if d.readOnly {
			return nil, 0, os.ErrPermission
		}
		_ = os.Remove(path)
		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0644)
		if err != nil {
			return nil, 0, err
		}
		return f, FileSuperseded, nil
	default:
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err != nil {
			f, err = os.Open(path)
			if err != nil {
				return nil, 0, err
			}
		}
		return f, FileOpened, nil
	}
}

// handleClose processes IRP_MJ_CLOSE.
func (d *DiskDevice) handleClose(h *Handler, req *IORequest) {
	d.mu.Lock()
	of, ok := d.files[req.FileID]
	if ok {
		delete(d.files, req.FileID)
	}
	d.mu.Unlock()

	if ok && of.f != nil {
		of.f.Close()
		d.log.LogAttrs(context.Background(), slog.LevelDebug, "Close",
			slog.String("path", of.path),
			slog.Bool("deleteOnClose", of.deleteOnClose),
			slog.Int("fileID", int(req.FileID)))
		if of.deleteOnClose {
			if err := os.RemoveAll(of.path); err != nil {
				d.log.LogAttrs(context.Background(), slog.LevelWarn, "delete on close failed",
					slog.String("path", of.path), slog.Any("err", err))
			} else {
				d.log.LogAttrs(context.Background(), slog.LevelInfo, "deleted",
					slog.String("path", of.path))
			}
		}
	}

	// Response: padding(5)
	var pad [5]byte
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, pad[:])
}

// handleRead processes IRP_MJ_READ.
func (d *DiskDevice) handleRead(h *Handler, req *IORequest) {
	if len(req.Payload) < 20 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	offset := binary.LittleEndian.Uint64(req.Payload[4:12])

	d.mu.Lock()
	of, ok := d.files[req.FileID]
	d.mu.Unlock()

	if !ok || of.f == nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	// Reuse read buffer to avoid per-read allocation.
	// Layout: length(4) + data, so read directly at offset 4.
	need := 4 + int(length)
	if cap(d.readBuf) < need {
		d.readBuf = make([]byte, need)
	}
	out := d.readBuf[:need]

	n, err := of.f.ReadAt(out[4:], int64(offset))
	if n == 0 && err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusEndOfFile, nil)
		return
	}

	// Response: length(4) + data
	binary.LittleEndian.PutUint32(out[0:4], uint32(n))
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:4+n])
}

// handleWrite processes IRP_MJ_WRITE.
func (d *DiskDevice) handleWrite(h *Handler, req *IORequest) {
	if d.readOnly {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, nil)
		return
	}

	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	length := binary.LittleEndian.Uint32(req.Payload[0:4])
	offset := binary.LittleEndian.Uint64(req.Payload[4:12])
	// padding at [12:32]
	writeData := req.Payload[32:]
	if uint32(len(writeData)) < length {
		length = uint32(len(writeData))
	}

	d.mu.Lock()
	of, ok := d.files[req.FileID]
	d.mu.Unlock()

	if !ok || of.f == nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	n, err := of.f.WriteAt(writeData[:length], int64(offset))
	if err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), nil)
		return
	}

	// Response: length(4) + padding(1)
	var out [5]byte
	binary.LittleEndian.PutUint32(out[0:4], uint32(n))
	h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])
}

// handleQueryVolumeInfo processes IRP_MJ_QUERY_VOLUME_INFORMATION.
func (d *DiskDevice) handleQueryVolumeInfo(h *Handler, req *IORequest) {
	if len(req.Payload) < 4 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	infoClass := binary.LittleEndian.Uint32(req.Payload[0:4])

	d.log.LogAttrs(context.Background(), slog.LevelDebug, "QueryVolume",
		slog.String("infoClass", fmt.Sprintf("0x%X", infoClass)),
		slog.Int("fileID", int(req.FileID)))

	switch infoClass {
	case FileFsVolumeInformation:
		// MS-FSCC 2.5.9: Length(4) + VolumeCreationTime(8) + VolumeSerialNumber(4) +
		// VolumeLabelLength(4) + SupportsObjects(1) + VolumeLabel(variable)
		label := encodeUTF16LE(d.name)
		const hdr = 17 // no padding between SupportsObjects and VolumeLabel
		out := make([]byte, 4+hdr+len(label))
		binary.LittleEndian.PutUint32(out[0:4], uint32(hdr+len(label)))
		binary.LittleEndian.PutUint32(out[12:16], 0x12345678) // serial
		binary.LittleEndian.PutUint32(out[16:20], uint32(len(label)))
		copy(out[4+hdr:], label)
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out)

	case FileFsSizeInformation:
		// MS-FSCC 2.5.8: Length(4) + TotalAllocationUnits(8) + AvailableAllocationUnits(8) +
		// SectorsPerAllocationUnit(4) + BytesPerSector(4)
		var out [4 + 24]byte
		binary.LittleEndian.PutUint32(out[0:4], 24)
		binary.LittleEndian.PutUint64(out[4:12], 1024*1024) // total ~1TB
		binary.LittleEndian.PutUint64(out[12:20], 512*1024)  // available ~512GB
		binary.LittleEndian.PutUint32(out[20:24], 8)         // sectors per unit
		binary.LittleEndian.PutUint32(out[24:28], 512)       // bytes per sector
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileFsFullSizeInformation:
		// MS-FSCC 2.5.4: Length(4) + TotalAllocationUnits(8) + CallerAvailable(8) +
		// ActualAvailable(8) + SectorsPerAllocationUnit(4) + BytesPerSector(4)
		var out [4 + 32]byte
		binary.LittleEndian.PutUint32(out[0:4], 32)
		binary.LittleEndian.PutUint64(out[4:12], 1024*1024)
		binary.LittleEndian.PutUint64(out[12:20], 512*1024)
		binary.LittleEndian.PutUint64(out[20:28], 512*1024)
		binary.LittleEndian.PutUint32(out[28:32], 8)
		binary.LittleEndian.PutUint32(out[32:36], 512)
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileFsAttributeInformation:
		// MS-FSCC 2.5.1: Length(4) + FileSystemAttributes(4) + MaxComponentNameLen(4) +
		// FileSystemNameLength(4) + FileSystemName(variable)
		fsName := encodeUTF16LE("FAT32")
		const hdr = 12
		out := make([]byte, 4+hdr+len(fsName))
		binary.LittleEndian.PutUint32(out[0:4], uint32(hdr+len(fsName)))
		// CASE_SENSITIVE_SEARCH | CASE_PRESERVED_NAMES | UNICODE_ON_DISK
		binary.LittleEndian.PutUint32(out[4:8], 0x00000007)
		binary.LittleEndian.PutUint32(out[8:12], 260)         // max component name length
		binary.LittleEndian.PutUint32(out[12:16], uint32(len(fsName)))
		copy(out[4+hdr:], fsName)
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out)

	case FileFsDeviceInformation:
		// MS-FSCC 2.5.10: Length(4) + DeviceType(4) + Characteristics(4)
		var out [4 + 8]byte
		binary.LittleEndian.PutUint32(out[0:4], 8)
		binary.LittleEndian.PutUint32(out[4:8], 0x00000007) // FILE_DEVICE_DISK
		// Characteristics = 0 (no FILE_REMOTE_DEVICE)
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// handleQueryInfo processes IRP_MJ_QUERY_INFORMATION.
func (d *DiskDevice) handleQueryInfo(h *Handler, req *IORequest) {
	if len(req.Payload) < 4 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	infoClass := binary.LittleEndian.Uint32(req.Payload[0:4])

	d.log.LogAttrs(context.Background(), slog.LevelDebug, "QueryInfo",
		slog.String("infoClass", fmt.Sprintf("0x%X", infoClass)),
		slog.Int("fileID", int(req.FileID)))

	d.mu.Lock()
	of, ok := d.files[req.FileID]
	d.mu.Unlock()

	if !ok {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	fi, err := os.Stat(of.path)
	if err != nil {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), nil)
		return
	}

	switch infoClass {
	case FileBasicInformation:
		// MS-FSCC 2.4.7: CreationTime(8) + LastAccessTime(8) + LastWriteTime(8) +
		// ChangeTime(8) + FileAttributes(4) = 36 bytes
		const basicInfoLen = 36
		var out [4 + basicInfoLen]byte
		binary.LittleEndian.PutUint32(out[0:4], basicInfoLen)
		ft := timeToFiletime(fi.ModTime())
		binary.LittleEndian.PutUint64(out[4:12], ft)  // creation time
		binary.LittleEndian.PutUint64(out[12:20], ft)  // last access
		binary.LittleEndian.PutUint64(out[20:28], ft)  // last write
		binary.LittleEndian.PutUint64(out[28:36], ft)  // change time
		binary.LittleEndian.PutUint32(out[36:40], fileInfoToAttributes(fi))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileStandardInformation:
		// MS-FSCC 2.4.41: AllocationSize(8) + EndOfFile(8) + NumberOfLinks(4) +
		// DeletePending(1) + Directory(1) = 22 bytes
		const stdInfoLen = 22
		var out [4 + stdInfoLen]byte
		binary.LittleEndian.PutUint32(out[0:4], stdInfoLen)
		if !fi.IsDir() {
			size := fi.Size()
			allocSize := (size + 4095) &^ 4095
			binary.LittleEndian.PutUint64(out[4:12], uint64(allocSize))
			binary.LittleEndian.PutUint64(out[12:20], uint64(size))
		}
		binary.LittleEndian.PutUint32(out[20:24], 1) // number of links
		if of.deleteOnClose {
			out[24] = 1
		}
		if fi.IsDir() {
			out[25] = 1
		}
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileAttributeTagInformation:
		// MS-FSCC 2.4.6: Length(4) + FileAttributes(4) + ReparseTag(4)
		var out [4 + 8]byte
		binary.LittleEndian.PutUint32(out[0:4], 8)
		binary.LittleEndian.PutUint32(out[4:8], fileInfoToAttributes(fi))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// handleSetInfo processes IRP_MJ_SET_INFORMATION.
func (d *DiskDevice) handleSetInfo(h *Handler, req *IORequest) {
	if len(req.Payload) < 8 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	infoClass := binary.LittleEndian.Uint32(req.Payload[0:4])
	var setLen uint32
	if len(req.Payload) >= 8 {
		setLen = binary.LittleEndian.Uint32(req.Payload[4:8])
	}
	// padding at [8:32], SetBuffer at [32:]
	var setData []byte
	if len(req.Payload) > 32 {
		setData = req.Payload[32:]
	}

	d.log.LogAttrs(context.Background(), slog.LevelDebug, "SetInfo",
		slog.String("infoClass", fmt.Sprintf("0x%X", infoClass)),
		slog.Int("setLen", int(setLen)),
		slog.Int("setDataLen", len(setData)),
		slog.Int("fileID", int(req.FileID)))

	d.mu.Lock()
	of, ok := d.files[req.FileID]
	d.mu.Unlock()

	if !ok {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	switch infoClass {
	case FileEndOfFileInformation:
		if d.readOnly {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, nil)
			return
		}
		if len(setData) < 8 {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
			return
		}
		newSize := int64(binary.LittleEndian.Uint64(setData[0:8]))
		if err := of.f.Truncate(newSize); err != nil {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), nil)
			return
		}
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileDispositionInformation:
		if d.readOnly {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, nil)
			return
		}
		d.mu.Lock()
		if len(setData) >= 1 {
			of.deleteOnClose = setData[0] != 0
		} else {
			// Empty SetBuffer means "mark for deletion" (MS-RDPEFS directory delete pattern).
			of.deleteOnClose = true
		}
		d.mu.Unlock()
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileRenameInformation:
		if d.readOnly {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, nil)
			return
		}
		// MS-FSCC 2.4.34: ReplaceIfExists(1) + RootDirectory(1) + FileNameLength(4) + FileName(variable)
		if len(setData) < 6 {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
			return
		}
		// replaceIfExists := setData[0]
		nameLen := binary.LittleEndian.Uint32(setData[2:6])
		if uint32(len(setData)) < 6+nameLen {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
			return
		}
		newRdpPath := decodeUTF16LE(setData[6 : 6+nameLen])
		newPath, ok := d.resolvePath(newRdpPath)
		if !ok {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusAccessDenied, nil)
			return
		}
		if err := os.Rename(of.path, newPath); err != nil {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), nil)
			return
		}
		d.mu.Lock()
		of.path = newPath
		d.mu.Unlock()
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	case FileBasicInformation:
		// Timestamps — acknowledge but don't enforce (os.Chtimes is limited)
		var out [4]byte
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out[:])

	default:
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNotSupported, nil)
	}
}

// handleQueryDirectory processes IRP_MJ_DIRECTORY_CONTROL / IRP_MN_QUERY_DIRECTORY.
// Returns one entry per call (MS-RDPEFS 2.2.3.4.10). The server sends repeated
// requests with InitialQuery=0 until we return STATUS_NO_MORE_FILES.
func (d *DiskDevice) handleQueryDirectory(h *Handler, req *IORequest) {
	// MS-RDPEFS 2.2.3.3.10: FsInformationClass(4) + InitialQuery(1) + PathLength(4) + Padding(23) + Path(variable)
	if len(req.Payload) < 32 {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusInvalidParameter, nil)
		return
	}

	infoClass := binary.LittleEndian.Uint32(req.Payload[0:4])
	initialQuery := req.Payload[4] != 0
	pathLen := binary.LittleEndian.Uint32(req.Payload[5:9])
	// padding at [9:32]

	d.log.LogAttrs(context.Background(), slog.LevelDebug, "QueryDirectory",
		slog.String("infoClass", fmt.Sprintf("0x%X", infoClass)),
		slog.Bool("initial", initialQuery),
		slog.Int("fileID", int(req.FileID)),
		slog.Int("pathLen", int(pathLen)))

	d.mu.Lock()
	of, ok := d.files[req.FileID]
	d.mu.Unlock()

	if !ok {
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoSuchFile, nil)
		return
	}

	if initialQuery {
		var pattern string
		if uint32(len(req.Payload)) >= 32+pathLen && pathLen > 0 {
			pattern = decodeUTF16LE(req.Payload[32 : 32+pathLen])
			// Extract just the filename part for matching
			if idx := strings.LastIndexAny(pattern, "\\/"); idx >= 0 {
				pattern = pattern[idx+1:]
			}
		}

		d.log.LogAttrs(context.Background(), slog.LevelDebug, "QueryDirectory pattern",
			slog.String("pattern", pattern),
			slog.Int("fileID", int(req.FileID)))

		entries, err := os.ReadDir(of.path)
		if err != nil {
			h.sendIOCompletion(req.DeviceID, req.CompletionID, osErrToNTStatus(err), nil)
			return
		}

		d.mu.Lock()
		of.dirEntries = entries
		of.dirIndex = 0
		of.dirPattern = pattern
		of.dirStarted = true
		d.mu.Unlock()
	}

	// Determine whether "." and ".." should be included.
	// Only include them for wildcard patterns that match everything.
	d.mu.Lock()
	pattern := of.dirPattern
	includeMetaDirs := pattern == "" || pattern == "*" || pattern == "*.*"
	totalEntries := len(of.dirEntries) + 2 // +2 for "." and ".."
	for {
		idx := of.dirIndex
		if !of.dirStarted || idx >= totalEntries {
			d.mu.Unlock()
			// Length(4)=0 + Padding(1)=0 per MS-RDPEFS 2.2.3.4.10
			var noMore [5]byte
			h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusNoMoreFiles, noMore[:])
			return
		}
		of.dirIndex++
		d.mu.Unlock()

		var name string
		var info fs.FileInfo
		var err error

		switch {
		case idx == 0: // "."
			if !includeMetaDirs {
				d.mu.Lock()
				continue
			}
			name = "."
			info, err = os.Stat(of.path)
		case idx == 1: // ".."
			if !includeMetaDirs {
				d.mu.Lock()
				continue
			}
			name = ".."
			parent := filepath.Dir(of.path)
			info, err = os.Stat(parent)
		default:
			entry := of.dirEntries[idx-2]
			name = entry.Name()
			if !includeMetaDirs && !matchPattern(pattern, name) {
				d.mu.Lock()
				continue
			}
			// Use os.Stat (not entry.Info/Lstat) so symlinks report target attributes.
			info, err = os.Stat(filepath.Join(of.path, name))
		}
		if err != nil {
			// Skip this entry (broken symlink, etc.) and try the next one.
			d.mu.Lock()
			continue
		}

		out := buildDirInfo(infoClass, name, info)
		d.log.LogAttrs(context.Background(), slog.LevelDebug, "QueryDirectory entry",
			slog.String("name", name),
			slog.String("pattern", pattern),
			slog.Int("outLen", len(out)))
		h.sendIOCompletion(req.DeviceID, req.CompletionID, StatusSuccess, out)
		return
	}
}

// buildDirInfo builds a directory information entry with a Length(4) prefix
// for use in DR_DRIVE_QUERY_DIRECTORY_RSP. Supports FileDirectoryInformation (1),
// FileFullDirectoryInformation (2), FileBothDirectoryInformation (3), and
// FileNamesInformation (12).
func buildDirInfo(infoClass uint32, name string, info fs.FileInfo) []byte {
	nameUTF16 := encodeUTF16LE(name)
	ft := timeToFiletime(info.ModTime())
	var fileSize uint64
	if !info.IsDir() {
		fileSize = uint64(info.Size())
	}
	attrs := fileInfoToAttributes(info)
	allocSize := (fileSize + 4095) &^ 4095

	// FileNamesInformation (class 12): NextEntryOffset(4) + FileIndex(4) + FileNameLength(4) + FileName
	if infoClass == FileNamesInformation {
		const fixed = 12
		out := make([]byte, 4+fixed+len(nameUTF16))
		binary.LittleEndian.PutUint32(out[0:4], uint32(fixed+len(nameUTF16)))
		binary.LittleEndian.PutUint32(out[12:16], uint32(len(nameUTF16)))
		copy(out[4+fixed:], nameUTF16)
		return out
	}

	// Common base for classes 1, 2, 3: NextEntryOffset(4) + FileIndex(4) +
	// CreationTime(8) + LastAccessTime(8) + LastWriteTime(8) + ChangeTime(8) +
	// EndOfFile(8) + AllocationSize(8) + FileAttributes(4) + FileNameLength(4) = 64
	const baseFixed = 64
	var fixedLen int
	switch infoClass {
	case FileFullDirectoryInformation:
		fixedLen = baseFixed + 4 // + EaSize(4) = 68
	case FileBothDirectoryInformation:
		fixedLen = baseFixed + 4 + 25 // + EaSize(4) + ShortNameLength(1) + ShortName(24) = 93
	default: // FileDirectoryInformation or unknown — use base
		fixedLen = baseFixed
	}

	out := make([]byte, 4+fixedLen+len(nameUTF16))
	binary.LittleEndian.PutUint32(out[0:4], uint32(fixedLen+len(nameUTF16)))
	// out[4:8] = NextEntryOffset = 0 (single entry)
	// out[8:12] = FileIndex = 0
	binary.LittleEndian.PutUint64(out[12:20], ft)            // CreationTime
	binary.LittleEndian.PutUint64(out[20:28], ft)            // LastAccessTime
	binary.LittleEndian.PutUint64(out[28:36], ft)            // LastWriteTime
	binary.LittleEndian.PutUint64(out[36:44], ft)            // ChangeTime
	binary.LittleEndian.PutUint64(out[44:52], fileSize)      // EndOfFile
	binary.LittleEndian.PutUint64(out[52:60], allocSize)     // AllocationSize
	binary.LittleEndian.PutUint32(out[60:64], attrs)         // FileAttributes
	binary.LittleEndian.PutUint32(out[64:68], uint32(len(nameUTF16))) // FileNameLength
	// EaSize, ShortNameLength, ShortName left as zero
	copy(out[4+fixedLen:], nameUTF16)
	return out
}

func timeToFiletime(t time.Time) uint64 {
	if t.IsZero() {
		return 0
	}
	return uint64(t.UnixNano()/100) + windowsEpochDiff
}

// matchPattern performs Windows-style case-insensitive filename matching.
// Supports * (match zero or more characters) and ? (match exactly one character).
func matchPattern(pattern, name string) bool {
	// Case-insensitive: compare in lower case
	matched, _ := filepath.Match(strings.ToLower(pattern), strings.ToLower(name))
	return matched
}

// filetimeToTime converts a Windows FILETIME to time.Time.
func filetimeToTime(ft uint64) time.Time {
	if ft == 0 || ft < windowsEpochDiff {
		return time.Time{}
	}
	nsec := (int64(ft) - windowsEpochDiff) * 100
	return time.Unix(nsec/1e9, nsec%1e9)
}

// fileInfoToAttributes converts os.FileInfo to NT file attributes.
func fileInfoToAttributes(fi fs.FileInfo) uint32 {
	attrs := uint32(0)
	if fi.IsDir() {
		attrs |= FileAttrDirectory
	}
	if fi.Mode()&0200 == 0 {
		attrs |= FileAttrReadOnly
	}
	if isHidden(fi.Name(), fi) {
		attrs |= FileAttrHidden
	}
	if attrs == 0 {
		attrs = FileAttrNormal
	}
	return attrs
}

// osErrToNTStatus maps common os errors to NTSTATUS codes.
func osErrToNTStatus(err error) uint32 {
	if err == nil {
		return StatusSuccess
	}
	if errors.Is(err, os.ErrNotExist) {
		return StatusObjectNameNotFound
	}
	if errors.Is(err, os.ErrExist) {
		return StatusObjectNameCollision
	}
	if errors.Is(err, os.ErrPermission) {
		return StatusAccessDenied
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		if errors.Is(pathErr.Err, errors.New("not a directory")) {
			return StatusNotADirectory
		}
	}
	return StatusUnsuccessful
}
