package rdp

import (
	"crypto/tls"
	"fmt"
	"log/slog"
	"os"
	"os/user"
	"runtime"
	"time"
)

// LevelTrace is a custom slog level for per-tile/pixel diagnostic dumps.
const LevelTrace = slog.LevelDebug - 4

// KeyboardMode controls how keyboard input is sent to the server.
type KeyboardMode int

const (
	KeyboardScancode KeyboardMode = iota // Send physical scancodes (remote layout determines characters)
	KeyboardUnicode                      // Send unicode codepoints for printable chars (local layout)
)

// Options configures the RDP client connection
type Options struct {
	// TLSConfig is used by ConnectContext. Nil verifies against system roots.
	TLSConfig *tls.Config

	// DisableTLS explicitly selects Standard RDP Security without TLS or NLA.
	DisableTLS bool

	// AllowUntrusted explicitly permits TLS without server certificate verification.
	AllowUntrusted bool

	// Connection
	Host string // Target hostname or IP
	Port int    // Target port (default: 3389)

	// Authentication
	Username string // Login username
	Password string // Login password
	Domain   string // Windows domain (optional)

	// Display
	Width              uint16 // Desktop width in pixels
	Height             uint16 // Desktop height in pixels
	Depth              uint16 // Color depth (8, 15, 16, 24, or 32)
	DesktopScaleFactor uint32 // DPI scale percent (100–500, 0 = don't send)
	DeviceScaleFactor  uint32 // Device scale (100, 140, or 180, 0 = don't send)
	GFX                bool   // Enable RDPGFX graphics pipeline (EGFX)
	NoAVC              bool   // Disable H.264/AVC codec (force RemoteFX/ClearCodec)
	NoDPR              bool   // SDL: don't scale RDP resolution to physical pixels
	Interpolate        bool   // Use bilinear interpolation instead of nearest-neighbor scaling

	// Experience (visual effects)
	Wallpaper          bool // Show desktop wallpaper
	FullWindowDrag     bool // Show window contents while dragging
	MenuAnimations     bool // Enable menu animations
	Theming            bool // Enable visual themes (Aero/Luna)
	CursorShadow       bool // Show cursor shadow
	CursorSettings     bool // Apply cursor blink settings
	FontSmoothing      bool // Enable ClearType font smoothing
	DesktopComposition bool // Enable Desktop Window Manager (Aero Glass)

	// Redirect
	Clipboard   bool               // Redirect clipboard (default: true)
	AudioOut    *AudioConfig       // Audio output config (nil = disabled)
	AudioIn     *AudioConfig       // Audio input config (nil = disabled)
	RemoteAudio bool               // Audio plays on server instead of client (audio-mode:1)
	Drives      []DriveRedirect    // Redirect local directories as network drives
	Serials     []SerialRedirect   // Redirect local serial ports
	Parallels   []ParallelRedirect // Redirect local parallel ports
	Printers    []PrinterRedirect  // Redirect printers (save print jobs to files)
	Smartcard   *SmartcardRedirect // Smartcard redirection (nil = disabled)
	Camera      bool               // Webcam redirection via RDPECAM (web viewer only)
	USBDevices  []USBRedirect      // Redirect USB devices by VID/PID

	// Session
	ConsoleSession bool // Connect to the console (admin) session

	// Heartbeat — detects silent connection loss. Sends keepalive at half
	// the timeout to provoke server responses. 0 = disabled.
	HeartbeatTimeout time.Duration // Max time without server data before disconnect (default: 10s)

	// Auto-reconnect
	AutoReconnect        bool // Reconnect automatically on connection loss
	MaxReconnectAttempts int  // Max reconnect attempts (0 = unlimited)

	// Multi-monitor
	// When set, defines the pre-connect monitor topology sent via TS_UD_CS_MONITOR.
	// Exactly one monitor must be marked Primary and must be at position (0,0).
	// When nil, single-monitor behavior is used (Width x Height).
	Monitors []MonitorConfig

	// Logging
	Logger *slog.Logger // nil = silent (discard handler)

	// Input
	KeyboardMode KeyboardMode // Keyboard input mode (default: KeyboardScancode)

	// Advanced
	Cookie string // Routing cookie (optional)
}

// AudioConfig configures audio format filtering for output or input.
type AudioConfig struct {
	BufMs   int    // buffer/pre-buffer in ms
	Stereo  bool   // true = only accept channels >= 2
	MinRate uint32 // 0 = any, e.g. 44100 = reject lower sample rates
	PCMOnly bool   // true = reject ADPCM formats
}

// DriveRedirect configures a local directory to be shared as a network drive.
type DriveRedirect struct {
	Name      string // Drive name visible to the server (truncated to 8 chars)
	LocalPath string // Local directory path to share
	ReadOnly  bool   // If true, the server cannot modify files
}

// SerialRedirect configures a local serial port for redirection.
type SerialRedirect struct {
	Name string // Port name on server (e.g. "COM3")
	Path string // Local device path (e.g. "/dev/ttyUSB0")
}

// ParallelRedirect configures a local parallel port for redirection.
type ParallelRedirect struct {
	Name string // Port name on server (e.g. "LPT1")
	Path string // Local device path (e.g. "/dev/lp0")
}

// PrinterRedirect configures a printer device for redirection.
// Print jobs are saved to OutputDir as .prn files and/or submitted via IPP.
// At least one of OutputDir or IPPURL must be set.
type PrinterRedirect struct {
	Name       string // Printer name visible on server
	DriverName string // Windows driver name (default: "MS Publisher Imagesetter")
	OutputDir  string // Directory to save print job files (empty = no file output)
	IPPURL     string // IPP printer URL (empty = no IPP submission)
	IsDefault  bool   // Announce as default printer
}

// SmartcardRedirect configures smartcard redirection via pcsclite.
type SmartcardRedirect struct {
	SocketPath string // pcscd socket path (default: /var/run/pcscd/pcscd.comm)
}

// USBRedirect configures a USB device for raw USB redirection (MS-RDPEUSB).
// Devices are matched by VID/PID. Use VID=0 and PID=0 to match by bus/address.
type USBRedirect struct {
	VID     uint16 // USB Vendor ID (0 = any)
	PID     uint16 // USB Product ID (0 = any)
	BusNum  uint8  // USB bus number (used when VID/PID are 0)
	DevAddr uint8  // USB device address (used when VID/PID are 0)
}

// MonitorConfig describes a single monitor in a multi-monitor topology.
type MonitorConfig struct {
	X, Y          int  // Position in virtual desktop (pixels)
	Width, Height int  // Resolution (pixels)
	Primary       bool // Primary monitor (exactly one required)
}

// DefaultOptions returns Options with sensible defaults.
// Experience defaults: theming, cursor, and font smoothing enabled for
// correct rendering; wallpaper, window drag, menu animations, and desktop
// composition disabled to reduce bandwidth. Use -enable-all-visuals for
// full desktop fidelity.
func DefaultOptions() *Options {
	return &Options{
		Logger:           slog.New(slog.DiscardHandler),
		Port:             3389,
		Width:            1600,
		Height:           900,
		Depth:            32,
		Clipboard:        true,
		Theming:          true,
		CursorShadow:     true,
		CursorSettings:   true,
		FontSmoothing:    true,
		HeartbeatTimeout: 10 * time.Second,
	}
}

// Validate checks that required options are set
func (o *Options) Validate() error {
	if o.Host == "" {
		return ErrNoHost
	}
	if o.Port <= 0 || o.Port > 65535 {
		o.Port = 3389
	}
	if o.Width == 0 {
		o.Width = 1024
	}
	if o.Height == 0 {
		o.Height = 768
	}
	if o.Depth == 0 {
		o.Depth = 32
	}
	// Validate DPI scale: only Windows-supported levels are accepted.
	if o.DesktopScaleFactor != 0 {
		switch o.DesktopScaleFactor {
		case 100, 125, 150, 175, 200, 225, 250, 300, 350, 400, 450, 500:
			// valid
		default:
			return fmt.Errorf("desktop-scale %d is not supported; use 100, 125, 150, 175, 200, 225, 250, 300, 350, 400, 450, or 500", o.DesktopScaleFactor)
		}
	}
	if o.DeviceScaleFactor != 0 && o.DeviceScaleFactor != 100 && o.DeviceScaleFactor != 140 && o.DeviceScaleFactor != 180 {
		o.DeviceScaleFactor = 0
	}
	// Validate multi-monitor topology
	if len(o.Monitors) > 0 {
		primaryCount := 0
		for i := range o.Monitors {
			m := &o.Monitors[i]
			if m.Primary {
				primaryCount++
				if m.X != 0 || m.Y != 0 {
					return ErrMonitorPrimaryPosition
				}
			}
			// Force even width
			m.Width &^= 1
		}
		if primaryCount != 1 {
			return ErrMonitorPrimaryCount
		}
	}
	// Validate drive redirects
	for i := range o.Drives {
		d := &o.Drives[i]
		if d.Name == "" {
			return ErrDriveNameRequired
		}
		// Do not truncate Name to 8 here: the 8-byte limit belongs only to the
		// PreferredDosName, which the device-announce builder derives separately
		// (7 ASCII chars + null). Name feeds the variable-length DeviceData
		// friendly name Windows displays, so it must keep its full length.
		fi, err := os.Stat(d.LocalPath)
		if err != nil {
			return fmt.Errorf("drive %q: %w", d.Name, err)
		}
		if !fi.IsDir() {
			return fmt.Errorf("drive %q: %s is not a directory", d.Name, d.LocalPath)
		}
	}
	// Validate serial redirects
	for i := range o.Serials {
		s := &o.Serials[i]
		if s.Name == "" {
			return fmt.Errorf("serial %d: name is required", i)
		}
		// Not truncated: the 8-byte cap is the PreferredDosName's; s.Name feeds
		// the full-length DeviceData name (see drive note above).
		if runtime.GOOS != "windows" {
			fi, err := os.Stat(s.Path)
			if err != nil {
				return fmt.Errorf("serial %q: %w", s.Name, err)
			}
			if fi.Mode()&os.ModeCharDevice == 0 {
				return fmt.Errorf("serial %q: %s is not a character device", s.Name, s.Path)
			}
		}
	}
	// Validate parallel redirects
	for i := range o.Parallels {
		p := &o.Parallels[i]
		if p.Name == "" {
			return fmt.Errorf("parallel %d: name is required", i)
		}
		// Not truncated: the 8-byte cap is the PreferredDosName's; p.Name feeds
		// the full-length DeviceData name (see drive note above).
		if runtime.GOOS != "windows" {
			if _, err := os.Stat(p.Path); err != nil {
				return fmt.Errorf("parallel %q: %w", p.Name, err)
			}
		}
	}
	// Validate printer redirects
	for i := range o.Printers {
		pr := &o.Printers[i]
		if pr.Name == "" {
			return fmt.Errorf("printer %d: name is required", i)
		}
		if pr.OutputDir == "" && pr.IPPURL == "" {
			return fmt.Errorf("printer %q: at least one of output directory or IPP URL is required", pr.Name)
		}
		if pr.OutputDir != "" {
			fi, err := os.Stat(pr.OutputDir)
			if err != nil {
				return fmt.Errorf("printer %q: %w", pr.Name, err)
			}
			if !fi.IsDir() {
				return fmt.Errorf("printer %q: %s is not a directory", pr.Name, pr.OutputDir)
			}
		}
	}
	// Validate smartcard redirect
	if o.Smartcard != nil {
		if o.Smartcard.SocketPath == "" {
			o.Smartcard.SocketPath = "/var/run/pcscd/pcscd.comm"
		}
	}
	if o.Logger == nil {
		o.Logger = slog.New(slog.DiscardHandler)
	}
	// Default username to local OS user.
	// Servers use this for session routing and login screen display.
	if o.Username == "" {
		if u, err := user.Current(); err == nil {
			o.Username = u.Username
		}
	}
	return nil
}
