# gopher-rdp

A pure Go RDP library implementing the Microsoft Remote Desktop Protocol (MS-RDPBCGR and related extensions). Zero external dependencies for the core protocol — standard library only.

It includes, a proof of concept, RDP client with two viewer modes - GUI ([Ebiten](https://github.com/hajimehoshi/ebiten) powered) and WEB (listens on port and runs in any browser). Supports .conf files (example included)

Compiles and runs on Linux (primarily developed for), Windows and MacOS (with some limitations, mostly redirections)

## Contents

- [Quick Start](#quick-start)
- [Features](#features)
  - [Connection & Security](#connection--security)
  - [Graphics Pipeline](#graphics-pipeline)
  - [Virtual Channels](#virtual-channels)
  - [Input](#input)
  - [Display Modes](#display-modes)
- [Library API](#library-api)
  - [Basic Usage](#basic-usage)
  - [Configuration](#configuration)
  - [Client Methods](#client-methods)
  - [Callbacks](#callbacks)
  - [Data Types](#data-types)
  - [Connection States](#connection-states)
  - [Errors](#errors)
- [Architecture](#architecture)
- [CLI Reference](#cli-reference)
- [Protocol Specifications Implemented](#protocol-specifications-implemented)
- [Project Structure](#project-structure)
- [Build & Test](#build--test)
- [Changelog](#changelog)
- [License](#license)

## Quick Start

```bash
# Build (vet + test + compile)
./build.sh

# Or just compile
go build -o gopher-rdp ./client

# Connect with GUI viewer (Ebiten)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -gui

# GUI at custom resolution
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -gui 1920x1080

# Connect with web viewer (auto-detects browser resolution)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -web 8080
# Then open http://localhost:8080 in a browser

# Multi-display web viewer (each browser tab = one display)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -web 8080 -display 2
# 3 displays, second is primary
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -web 8080 -display 3,1

# Headless (no display)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret

# NLA authentication is enabled by default (TLS+CredSSP/NTLMv2)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -gui

# Custom resolution
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -gui 1920x1080

# High DPI (150% scale)
./gopher-rdp -host 192.168.1.100 -user admin -pass secret -desktop-scale 150 -gui

```

## Features

### Connection & Security

- **TLS + NLA/CredSSP v6** — Default: TLS with NTLMv2 authentication, SPNEGO wrapping, SHA-256 public key binding
- **Standard RDP Security** — Legacy RC4/SHA1/MD5 encryption (for older servers)
- **Auto-reconnect** — Configurable retry attempts on connection loss
- **Heartbeat** — Detects silent connection loss (sends keepalive, disconnects on timeout)

### Graphics Pipeline

**RDPGFX (RDP 8.0+ Graphics Pipeline Extension)** — enabled by default (disable with `-no-gfx`):
- Capability versions 8.0, 8.1, 10.0–10.7 with AVC/H.264 pass-through (web viewer only, disable with `-no-avc`)
- RemoteFX Progressive codec with tile caching and DWT extrapolation
- ClearCodec with residual, band, and subcodec layers plus vbar caching
- NSCodec with YCoCg color transform and per-plane RLE
- Planar codec (RDP 6.0) with nibble-based run-length encoding
- Uncompressed pixel blitting
- ZGFX bulk compression (Huffman + LZ77, 2.5 MB history buffer)
- MPPC bulk decompression (RDP 4.0/5.0 legacy compression)
- Surface management, frame acknowledgment, and cache eviction

**Legacy Drawing Orders (16 primary orders):**
- Blit orders: DstBlt, PatBlt, ScrBlt, MemBlt, Mem3Blt, SaveBitmap (full ROP3)
- Shape orders: OpaqueRect, LineTo, Polyline, PolygonSC, PolygonCB, EllipseSC, EllipseCB
- Glyph orders: GlyphIndex, FastIndex, FastGlyph
- ROP2 (binary) and ROP3 (ternary) raster operation support

**Bitmap Codecs:**
- Interleaved RLE decompression (8/15/16/24/32 bpp)
- RDP 6.0 Planar codec (32 bpp compressed bitmaps)

**Pointer Updates:**
- Color and new pointer updates with AND/XOR mask decoding

### Virtual Channels

- **Clipboard** (MS-RDPECLIP) — Bidirectional text and bitmap clipboard with long format name support
- **Audio Output** (MS-RDPSND) — PCM audio sample streaming via callback, configurable format filtering (stereo/mono, sample rate, PCM-only)
- **Audio Input** (MS-RDPEAI) — Microphone capture with browser-to-server PCM streaming, real-time resampling, and MS-ADPCM/IMA ADPCM encoding
- **Drive Redirection** (MS-RDPEFS/MS-RDPDR) — Share local directories as network drives (read-write or read-only), with full file I/O, directory enumeration, and filesystem metadata
- **Printer Redirection** (MS-RDPEPC/MS-RDPDR) — Redirect printers to the remote session, save print jobs as local files and/or submit via IPP to a CUPS/network printer
- **Serial Port Redirection** (MS-RDPESP/MS-RDPDR) — Redirect local serial ports (e.g. `/dev/ttyUSB0`) as COM ports in the remote session, with full termios-based baud/parity/flow control
- **Parallel Port Redirection** (MS-RDPESP/MS-RDPDR) — Redirect local parallel ports (e.g. `/dev/lp0`) as LPT ports in the remote session
- **Smartcard Redirection** (MS-RDPESC/MS-RDPDR) — Redirect local smartcard readers via pcsclite, with full SCard API passthrough for authentication and signing applications
- **USB Redirection** (MS-RDPEUSB) — Raw USB device forwarding over DRDYNVC, with URB-level transfer support (control, bulk, interrupt) via Linux usbdevfs. PnP hotplug: auto-detect newly inserted devices, with class-based exclusion (HID, hub, audio, smartcard auto-excluded)
- **Webcam Redirection** (MS-RDPECAM) — Browser webcam capture via WebCodecs VideoEncoder (H.264 Annex B), streamed to the remote session as a virtual camera device. Works with Chrome, Firefox, Opera, and Safari (web viewer only)
- **Display Resize** (MS-RDPEDISP) — Dynamic session resize over DRDYNVC, with rate limiting
- **Dynamic Virtual Channels** (DRDYNVC) — Channel multiplexing with DataFirst/Data reassembly

### Input

- Keyboard scancodes (via GUI or web frontend) — physical key mapping, remote layout determines characters
- Keyboard unicode mode — send typed characters directly, local layout determines characters
- Mouse movement, buttons, and wheel (via GUI or web frontend)

### Display Modes

| Mode | Flag | Description |
|------|------|-------------|
| Headless | *(default)* | No display output, callbacks only |
| GUI | `-gui` | Desktop viewer using [Ebiten](https://ebitengine.org/) with keyboard/mouse input and audio playback |
| GUI multi-display | `-gui -display N[,P]` | One Ebiten window per monitor, broker-spawned child processes |
| Web | `-web <port>` | HTTP server with WebGL rendering (Canvas 2D fallback), WebSocket transport for bitmaps, cursor, clipboard, audio, and microphone. DPR-aware scaling with bilinear/nearest-neighbor filtering |
| Web multi-display | `-web <port> -display N[,P]` | Each browser tab is a separate monitor; resolution auto-detected |

## Library API

Use gopher-rdp as a Go library to build custom RDP applications.

### Basic Usage

```go
package main

import (
    "log"
    "gopher-rdp"
)

func main() {
    opts := rdp.DefaultOptions()
    opts.Host = "192.168.1.100"
    opts.Username = "admin"
    opts.Password = "secret"
    opts.Width = 1920
    opts.Height = 1080

    client, err := rdp.NewClient(opts)
    if err != nil {
        log.Fatal(err)
    }
    defer client.Close()

    // Receive screen updates
    client.OnBitmap(func(u *rdp.BitmapUpdate) {
        // u.X, u.Y, u.Width, u.Height — region on screen
        // u.Data — pixel data (BGRX, may be RLE-compressed)
        // u.TopDown — true for RDPGFX, false for legacy
    })

    // Receive cursor changes
    client.OnPointer(func(u *rdp.PointerUpdate) {
        // u.Type — PointerNull, PointerDefault, PointerShape, or PointerCached
        // u.Data — top-down RGBA pixels (for PointerShape)
    })

    // Handle disconnection
    client.OnDisconnect(func(err error) {
        log.Printf("disconnected: %v", err)
    })

    if err := client.Connect(); err != nil {
        log.Fatal(err)
    }
    <-client.Done() // block until connection ends
}
```

### Configuration

`DefaultOptions()` returns sensible defaults (1600x900, 32bpp, theming/cursors/font-smoothing on, clipboard enabled, no audio). Security protocol is negotiated automatically (TLS+NLA → TLS → RDP Security). Override fields as needed:

```go
opts := rdp.DefaultOptions()
opts.Host = "10.0.0.5"
opts.Username = "user"
opts.Password = "pass"
opts.Domain = "CORP"
opts.Width = 2560
opts.Height = 1440
opts.Depth = 32
opts.DesktopScaleFactor = 150       // 150% DPI
opts.ConsoleSession = true          // admin session
opts.HeartbeatTimeout = 10 * time.Second
opts.AutoReconnect = true
opts.MaxReconnectAttempts = 5

// Redirect a printer (save jobs locally and/or submit via IPP)
opts.Printers = []rdp.PrinterRedirect{{
    Name:      "MyPrinter",
    OutputDir: "/tmp/print",                              // save .prn files here
    IPPURL:    "http://127.0.0.1:631/printers/hp",        // also submit via IPP
    IsDefault: true,
}}

// Redirect USB devices (raw URB forwarding, Linux only)
opts.USBDevices = []rdp.USBRedirect{
    {VID: 0x1234, PID: 0x5678},     // match by vendor:product ID
    {BusNum: 1, DevAddr: 5},        // match by bus number and device address
    {},                              // auto: redirect all newly inserted devices
}
```

#### Options Fields

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `Host` | `string` | *(required)* | Server hostname or IP |
| `Port` | `int` | 3389 | Server port |
| `Username` | `string` | OS user | Login username |
| `Password` | `string` | `""` | Login password |
| `Domain` | `string` | `""` | Windows domain |
| `Width` | `uint16` | 1600 | Desktop width (pixels) |
| `Height` | `uint16` | 900 | Desktop height (pixels) |
| `Depth` | `uint16` | 32 | Color depth (8, 15, 16, 24, 32) |
| `DesktopScaleFactor` | `uint32` | 0 | DPI zoom % (100/125/150/175/200/225/250/300/350/400/450/500, 0 = off) |
| `DeviceScaleFactor` | `uint32` | 0 | Physical DPI (100, 140, 180) |
| `Clipboard` | `bool` | true | Enable clipboard channel |
| `AudioOut` | `*AudioConfig` | nil | Audio output config (nil = no audio) |
| `AudioIn` | `*AudioConfig` | nil | Audio input config (nil = disabled) |
| `RemoteAudio` | `bool` | false | Audio plays on server instead of redirecting to client |
| `Drives` | `[]DriveRedirect` | nil | Local directories shared as network drives |
| `Printers` | `[]PrinterRedirect` | nil | Printer redirects (save jobs to file and/or submit via IPP) |
| `Serials` | `[]SerialRedirect` | nil | Serial port redirects (e.g. COM3 → /dev/ttyUSB0) |
| `Parallels` | `[]ParallelRedirect` | nil | Parallel port redirects (e.g. LPT1 → /dev/lp0) |
| `Smartcard` | `*SmartcardRedirect` | nil | Smartcard redirection via pcsclite (nil = disabled) |
| `USBDevices` | `[]USBRedirect` | nil | Raw USB device redirects by VID/PID, bus/addr, or auto (all zeros) |
| `Camera` | `bool` | false | Enable webcam redirection via RDPECAM (web viewer only) |
| `ConsoleSession` | `bool` | false | Admin/console session |
| `Wallpaper` | `bool` | false | Show desktop wallpaper |
| `FullWindowDrag` | `bool` | false | Window contents while dragging |
| `MenuAnimations` | `bool` | false | Menu animations |
| `Theming` | `bool` | true | Visual themes (Aero/Luna) |
| `CursorShadow` | `bool` | true | Show cursor shadow |
| `CursorSettings` | `bool` | true | Apply cursor blink settings |
| `FontSmoothing` | `bool` | true | ClearType fonts |
| `DesktopComposition` | `bool` | false | Desktop composition (Aero Glass) |
| `GFX` | `bool` | false | Enable RDPGFX graphics pipeline (CLI enables by default) |
| `NoAVC` | `bool` | false | Disable H.264/AVC codec (force RemoteFX/ClearCodec) |
| `NoDPR` | `bool` | false | Don't scale RDP resolution to physical pixels |
| `Interpolate` | `bool` | false | Use bilinear interpolation instead of nearest-neighbor |
| `KeyboardMode` | `KeyboardMode` | `KeyboardScancode` | Keyboard input mode (scancode or unicode) |
| `HeartbeatTimeout` | `time.Duration` | 10s | Max time without server data before disconnect (0 = off) |
| `AutoReconnect` | `bool` | false | Auto-reconnect on disconnect |
| `MaxReconnectAttempts` | `int` | 0 | Max retries (0 = unlimited) |
| `Monitors` | `[]MonitorConfig` | nil | Multi-monitor topology |
| `Cookie` | `string` | `""` | Routing cookie |
| `Logger` | `*slog.Logger` | nil (discard) | Structured logger (nil = silent) |

### Client Methods

#### Lifecycle

```go
func NewClient(opts *Options) (*Client, error)  // Create client
func (c *Client) Connect() error                // Connect (blocks through handshake, then spawns receive loop)
func (c *Client) Close() error                  // Close connection
func (c *Client) Done() <-chan struct{}          // Channel closed when connection ends
func (c *Client) State() ConnectionState        // Current state (Disconnected → Connected)
```

#### Input

```go
func (c *Client) SendKeyboard(scancode uint16, pressed bool) error
func (c *Client) SendUnicode(codepoint uint16, pressed bool) error
func (c *Client) SendMouse(x, y int, buttons uint16) error
func (c *Client) SendMouseWheel(x, y int, delta int, horizontal bool) error
```

#### Display

```go
func (c *Client) Resize(width, height int) error                    // Dynamic resize via MS-RDPEDISP
func (c *Client) ResizeMulti(monitors []disp.MonitorLayout) error   // Multi-monitor resize
func (c *Client) RefreshRect(x, y, width, height int) error         // Request server redraw
```

#### Clipboard

```go
func (c *Client) SetClipboard(text string) error          // Push text to server clipboard
func (c *Client) SetClipboardImage(pngData []byte) error   // Push PNG image to server clipboard
func (c *Client) RequestClipboard() error                  // Request server clipboard text (async → OnClipboardText)
func (c *Client) RequestClipboardImage() error             // Request server clipboard image (async → OnClipboardImage)
func (c *Client) SetClipboardEnabled(enabled bool)         // Enable/disable clipboard channel
```

#### Audio Input

```go
func (c *Client) SendAudioInput(pcm []byte) error                     // Send PCM audio data to server microphone
func (c *Client) AudioInputFormat() (audin.AudioFormat, bool)          // Negotiated format and active status
```

#### Framebuffer Access

```go
func (c *Client) FramebufferTopDown() (pix []byte, w, h int)          // Internal framebuffer as top-down RGBA
func (c *Client) FramebufferDims() (int, int)                          // Framebuffer width and height
func (c *Client) FramebufferWriteTopDown(dst []byte) (int, int)        // Copy framebuffer into dst, return dims
```

### Callbacks

Register callbacks **before** calling `Connect()`:

```go
client.OnBitmap(func(u *rdp.BitmapUpdate) { ... })                    // Screen updates
client.OnStridedBitmap(func(x, y, w, h int, data []byte, stride int) { ... }) // Screen updates with stride
client.OnBeginPaint(func() { ... })                                    // Start of paint batch
client.OnEndPaint(func() { ... })                                      // End of paint batch
client.OnPointer(func(u *rdp.PointerUpdate) { ... })                  // Cursor changes
client.OnResize(func(width, height int) { ... })                       // Server completed resize
client.OnDisconnect(func(err error) { ... })                           // Connection lost
client.OnReconnecting(func() { ... })                                  // Reconnect attempt starting
client.OnReconnected(func() { ... })                                   // Reconnect succeeded
client.OnClipboardUpdate(func(hasText, hasImage bool) { ... })         // Remote clipboard changed
client.OnClipboardText(func(text string) { ... })                      // Clipboard text received
client.OnClipboardImage(func(pngData []byte) { ... })                  // Clipboard image received
client.OnAudioData(func(s *rdpsnd.AudioSample) { ... })                // PCM audio data
client.OnAudioClose(func() { ... })                                    // Audio channel closed
client.OnAudioInputOpen(func(fmt audin.AudioFormat) { ... })           // Server requests microphone
client.OnAudioInputClose(func() { ... })                               // Server stops microphone
client.OnDynChannel(func(name string, ch *dvc.DynChannel) { ... })    // Dynamic channel opened
```

### Data Types

#### BitmapUpdate

```go
type BitmapUpdate struct {
    X, Y         int    // Position on screen
    Width        int    // Bitmap width
    Height       int    // Bitmap height
    BitsPerPixel int    // Color depth
    IsCompressed bool   // Data is RLE-compressed
    TopDown      bool   // true = RDPGFX (top-down), false = legacy (bottom-up)
    Data         []byte // Pixel data (BGRX, may be compressed)
}
```

#### PointerUpdate

```go
type PointerUpdate struct {
    Type       byte   // PointerNull(0), PointerDefault(1), PointerShape(2), PointerCached(3)
    CacheIndex uint16 // Cache slot
    HotSpotX   uint16 // Hotspot X
    HotSpotY   uint16 // Hotspot Y
    Width      uint16 // Cursor width
    Height     uint16 // Cursor height
    Data       []byte // Top-down RGBA pixels (PointerShape only)
}
```

#### AudioSample (rdpsnd package)

```go
type AudioSample struct {
    Format         AudioFormat // Negotiated format (channels, sample rate, bits)
    WaveTimestamp  uint16      // Server timestamp
    AudioTimestamp uint32      // Extended timestamp (SNDC_WAVE2 only)
    Data           []byte      // Raw PCM audio data
}
```

#### DriveRedirect

```go
type DriveRedirect struct {
    Name      string // Drive name visible to the server (truncated to 8 chars)
    LocalPath string // Local directory path to share
    ReadOnly  bool   // If true, the server cannot modify files
}
```

#### PrinterRedirect

```go
type PrinterRedirect struct {
    Name       string // Printer name visible on server
    DriverName string // Windows driver name (default: "MS Publisher Imagesetter")
    OutputDir  string // Directory to save print job files (empty = no file output)
    IPPURL     string // IPP printer URL (empty = no IPP submission)
    IsDefault  bool   // Announce as default printer
}
```

#### SerialRedirect

```go
type SerialRedirect struct {
    Name string // Port name on server (e.g. "COM3")
    Path string // Local device path (e.g. "/dev/ttyUSB0")
}
```

#### ParallelRedirect

```go
type ParallelRedirect struct {
    Name string // Port name on server (e.g. "LPT1")
    Path string // Local device path (e.g. "/dev/lp0")
}
```

#### SmartcardRedirect

```go
type SmartcardRedirect struct {
    SocketPath string // pcscd socket path (default: /var/run/pcscd/pcscd.comm)
}
```

#### USBRedirect

```go
type USBRedirect struct {
    VID     uint16 // USB Vendor ID (0 = any)
    PID     uint16 // USB Product ID (0 = any)
    BusNum  uint8  // USB bus number (used when VID/PID are 0)
    DevAddr uint8  // USB device address (used when VID/PID are 0)
    // All zeros = auto mode: redirect any newly inserted device
    // (HID, hub, audio, and smartcard classes are auto-excluded)
}
```

#### MonitorConfig

```go
type MonitorConfig struct {
    X, Y          int  // Position in virtual desktop (pixels)
    Width, Height int  // Resolution (pixels)
    Primary       bool // Primary monitor (exactly one required, must be at 0,0)
}
```

### Connection States

```go
const (
    StateDisconnected    // Initial / after close
    StateConnecting      // TCP connect in progress
    StateX224Negotiation // X.224 CR/CC exchange
    StateTLSHandshake    // TLS upgrade
    StateCredSSPAuth     // NLA/CredSSP authentication
    StateMCSConnect      // MCS Connect Initial/Response
    StateSecurityExchange // Security Exchange PDU
    StateLicensing       // Server licensing
    StateCapabilities    // Demand Active / Confirm Active
    StateFinalization    // Synchronize, Control, Font List
    StateConnected       // Fully connected, receive loop running
)
```

### Errors

```go
var (
    ErrNoHost                 // "host is required"
    ErrConnectionFailed       // TCP connect failed
    ErrNegotiationFailed      // X.224 negotiation failed
    ErrTLSFailed              // TLS handshake failed
    ErrAuthFailed             // NLA/CredSSP authentication failed
    ErrMCSConnectFailed       // MCS Connect failed
    ErrMCSChannelJoinFailed   // MCS Channel Join failed
    ErrSecurityExchangeFailed // Security Exchange failed
    ErrLicensingFailed        // Licensing failed
    ErrCapabilitiesFailed     // Capabilities exchange failed
    ErrFinalizationFailed     // Finalization failed
    ErrDisconnected           // Disconnected by server
    ErrNotConnected           // Operation requires active connection
    ErrReconnectFailed        // Auto-reconnect exhausted
    ErrHeartbeatTimeout       // No server data within timeout
    ErrServerRedirect         // Server redirection
    ErrMonitorPrimaryCount    // Multi-monitor: wrong primary count
    ErrMonitorPrimaryPosition // Multi-monitor: primary not at (0,0)
    ErrDriveNameRequired      // Drive redirect: name is required
)
```

## Architecture

```
┌──────────────────────────────────────────────────────────────┐
│                      Application Layer                       │
│                    (client.go, options.go)                    │
├──────────────────────────────────────────────────────────────┤
│  Graphics  │   Input    │  Channels  │        Auth           │
│  (bitmap,  │ (kbd/mouse)│ (clipboard,│  (TLS, NLA/CredSSP,   │
│   orders,  │            │  audio,    │   Standard RDP Sec)   │
│   codecs)  │            │  resize)   │                       │
├──────────────────────────────────────────────────────────────┤
│                     RDP Core (MS-RDPBCGR)                    │
│           Capabilities · Licensing · Fast-Path               │
├──────────────────────────────────────────────────────────────┤
│      MCS (T.125)      │     GCC (T.124)    │    Security     │
├──────────────────────────────────────────────────────────────┤
│                     X.224 (ISO 8073)                         │
├──────────────────────────────────────────────────────────────┤
│                     TPKT (RFC 1006)                          │
├──────────────────────────────────────────────────────────────┤
│                     TCP Connection                           │
└──────────────────────────────────────────────────────────────┘
```

## CLI Reference

### General

| Flag | Default | Description |
|------|---------|-------------|
| `-config file` | `gopher-rdp.conf` | Load options from config file |

### Connection

| Flag | Default | Description |
|------|---------|-------------|
| `-host` | *(required)* | RDP server hostname or IP |
| `-port` | 3389 | RDP server port |
| `-user` | OS user | Username (omit for Windows login screen) |
| `-pass` | | Password (omit for Windows login screen) |
| `-domain` | | Windows domain |
| `-cookie` | | Routing cookie |

### Display

| Flag | Default | Description |
|------|---------|-------------|
| `-display N[,P]` | | Display count, P=primary index (default 0, `-web` or `-gui`) |
| `-depth` | 32 | Color depth: 8, 15, 16, 24, or 32 |
| `-desktop-scale` | 0 | DPI zoom percent (100/125/150/175/200/225/250/300/350/400/450/500, 0 = off) |
| `-device-scale` | 0 | Physical DPI tier (100, 140, or 180) |
| `-no-gfx` | false | Disable RDPGFX graphics pipeline (EGFX) |
| `-no-avc` | false | Disable H.264/AVC codec (force RemoteFX/ClearCodec) |
| `-no-dpr` | false | Don't scale RDP resolution to physical pixels (match web behavior) |
| `-interpolate` | false | Use bilinear interpolation instead of nearest-neighbor (all viewers) |
| `-keyboard` | scancode | Keyboard input mode: `scancode` (remote layout) or `unicode` (local layout) |
| `-gui [WxH]` | | Graphical desktop viewer (default 1600x900) |
| `-web [port]` | 8080 | Web viewer (default port 8080) |

### Session

| Flag | Default | Description |
|------|---------|-------------|
| `-admin` | false | Connect to console (admin) session |
| `-heartbeat` | 10 | Heartbeat timeout in seconds (0 = disable) |
| `-auto-reconnect` | on | Toggle automatic reconnection |
| `-reconnect-attempts` | 0 | Max reconnect attempts (0 = unlimited) |

### Visual Effects

Pass a flag to toggle its default. Theming, cursors, and font smoothing are on by default; the rest are off.

| Flag | Default | Description |
|------|---------|-------------|
| `-wallpaper` | off | Desktop wallpaper |
| `-window-drag` | off | Window contents while dragging |
| `-menu-animations` | off | Menu animations |
| `-theming` | on | Visual themes (Aero/Luna) |
| `-cursor-shadow` | on | Cursor shadow |
| `-cursor-settings` | on | Cursor blink settings |
| `-font-smoothing` | on | ClearType font smoothing |
| `-desktop-composition` | off | Desktop composition (Aero Glass) |
| `-enable-all-visuals` | | Enable all visual effects |

### Redirect

| Flag | Default | Description |
|------|---------|-------------|
| `-no-clipboard` | false | Disable clipboard redirection |
| `-audio-out [opts]` | no audio | Audio output. Omit = no audio at all. `remote` = play on server. Default suboptions: `stereo,hirate,15ms`. Other suboptions: `mono`, `lorate`, `Nms`, `pcm`, Hz value |
| `-audio-in [opts]` | disabled | Audio input (default `mono,lorate,5ms`, `-web` only). Same suboptions as `-audio-out` |
| `-drive name:/path` | | Share local directory as network drive (repeatable). Append `:ro` for read-only |
| `-printer name:/path` | | Redirect printer (repeatable). Save jobs to dir and/or submit via IPP |
| `-serial name:/dev/path` | | Redirect serial port (repeatable). Example: `-serial COM3:/dev/ttyUSB0` |
| `-parallel name:/dev/path` | | Redirect parallel port (repeatable). Example: `-parallel LPT1:/dev/lp0` |
| `-smartcard [socket]` | | Enable smartcard redirection via pcsclite (default socket: `/var/run/pcscd/pcscd.comm`) |
| `-usb auto` | | Redirect all newly inserted USB devices (HID, hub, audio, smartcard auto-excluded) |
| `-usb vid:pid` | | Redirect USB device by vendor:product ID in hex (repeatable). Example: `-usb 1234:5678` |
| `-usb bus,addr` | | Redirect USB device by bus number and address in decimal. Example: `-usb 1,5` |
| `-camera` | false | Enable webcam redirection via RDPECAM (web viewer only) |

**Audio suboption examples:**

```
                                 # (omit -audio-out entirely = no audio at all)
-audio-out                       # redirect to client: stereo, 44100+ Hz, 15ms buffer
-audio-out remote                # play audio on server (no redirection)
-audio-out mono,lorate,30ms      # redirect to client: mono, any sample rate, 30ms buffer
-audio-out pcm,48000             # redirect to client: PCM only (reject ADPCM), exact 48000 Hz
-audio-in                        # defaults: mono, any rate, 5ms buffer
-audio-in stereo,hirate,10ms     # stereo, 44100+ Hz, 10ms buffer
-audio-out -audio-in             # output + microphone together
```

**Printer redirection:**

Print jobs from the remote Windows session are captured and delivered via one or both of:

| Mode | Description |
|------|-------------|
| **File** | Save raw PostScript/PCL to a local directory as `.prn` files |
| **IPP** | Submit to a CUPS or network printer via Internet Printing Protocol (RFC 8010) |
| **Both** | Save locally and submit via IPP simultaneously |

Options: `:driver=X` overrides the Windows driver name (default: `MS Publisher Imagesetter`). `:default` announces as the default printer.

```
-printer MyPrn:/tmp/print                                    # file only: save to /tmp/print/
-printer MyPrn:ipp=http://127.0.0.1:631/printers/hp               # IPP only: no local file
-printer MyPrn:/tmp/print:ipp=http://127.0.0.1:631/printers/hp    # both: save + submit via IPP
-printer MyPrn:/tmp/print:driver=HP LaserJet:default         # custom driver, default printer
-printer MyPrn:ipp=http://127.0.0.1:631/printers/hp:default       # IPP only, default printer
```

Config file equivalent (one printer per line):

```
printer MyPrn:/tmp/print:default
printer HPNet:ipp=http://127.0.0.1:631/printers/hp
printer Both:/tmp/print:ipp=http://127.0.0.1:631/printers/hp
```

**Smartcard redirection examples:**

```
-smartcard                              # use default pcscd socket
-smartcard /custom/path/pcscd.comm      # custom pcscd socket path
```

**USB redirection examples:**

```
-usb auto                              # redirect any newly inserted device (PnP)
-usb 1234:5678                          # redirect device with VID=1234, PID=5678
-usb 1,5                               # redirect device on bus 1, address 5
-usb 1234:5678 -usb abcd:ef01          # redirect multiple devices
-usb auto -usb 1234:5678               # auto + explicit (explicit bypasses class filter)
```

### Logging

| Flag | Default | Description |
|------|---------|-------------|
| `-log [COMP,...]` | disabled | Enable logging (default: all components, info level, stderr) |
| `-log-level` | info | Minimum level: `all`/`trace`, `debug`, `info`, `warn`, `error` |
| `-log-file [name]` | stderr | Write logs to file (default `gopher-rdp.log`) |

Components: `RDP`, `TPKT`, `NLA`, `X224`, `MCS`, `SEC`, `LIC`, `PDU`, `FASTPATH`, `POINTER`, `EGFX`, `CLEARCODEC`, `DVC`, `DISP`, `CLIPRDR`, `RDPSND`, `AUDIN`, `RDPDR`, `URBDRC`, `RDPECAM`, `BITMAP`, `GUI`, `WEB`

### Profiling

| Flag | Default | Description |
|------|---------|-------------|
| `-cpu-profile file` | | Write CPU profile to file |
| `-mem-profile file` | | Write heap profile to file on exit |

A pprof server is always available at `http://localhost:6060`.

## Protocol Specifications Implemented

| Specification | Coverage |
|---------------|----------|
| MS-RDPBCGR | Core protocol: X.224, MCS/GCC, licensing, capabilities, fast-path, slow-path, auto-reconnect |
| MS-RDPEGDI | 16 primary drawing orders, bitmap caches, glyph caches |
| MS-RDPEGFX | Graphics pipeline v8/v8.1/v10.0–10.7: surfaces, codec dispatch, frame acknowledge, cache management, AVC420/AVC444 H.264 pass-through (web viewer) |
| MS-RDPRFX | RemoteFX Progressive codec: RLGR, DWT, ICT, tile caching, coefficient diff |
| MS-RDPNSC | NSCodec: YCoCg transform, per-plane RLE |
| MS-RDPCCOD | ClearCodec: residual, bands, subcodec, vbar caching |
| MS-RDPECLIP | Clipboard virtual channel: text and bitmap formats, long format names, capability negotiation |
| MS-RDPSND | Audio output virtual channel: PCM sample streaming |
| MS-RDPEAI | Audio input virtual channel: microphone capture, format negotiation, ADPCM encoding |
| MS-RDPEFS | File system virtual channel: drive redirection, file I/O, directory enumeration |
| MS-RDPEPC | Printer redirection: job capture and IPP submission |
| MS-RDPESP | Serial and parallel port redirection |
| MS-RDPESC | Smartcard redirection: SCard API passthrough via pcsclite |
| MS-RDPEUSB | USB device redirection: raw URB forwarding over DRDYNVC (control, bulk, interrupt), PnP hotplug with class filtering |
| MS-RDPECAM | Webcam redirection: browser H.264 capture via WebCodecs, streamed as virtual camera device |
| MS-RDPEDISP | Display control virtual channel: dynamic resize |
| MS-RDPEDYC | Dynamic virtual channel transport: multiplexing, reassembly |
| MS-RDPELE | Licensing: server license negotiation |

## Project Structure

```
client.go                      # Main RDP client, connection state machine
options.go                     # Client configuration (Options struct)
errors.go                      # Error sentinels
framebuffer.go                 # RGBA framebuffer with rect read/write/copy
client/                        # CLI binary (main.go)
protocol/
  tpkt/                        # TPKT transport framing (RFC 1006)
  x224/                        # X.224 connection negotiation (ISO 8073)
  ber/                         # ASN.1 BER encoding/decoding
  mcs/                         # MCS (T.125) + GCC (T.124) conference structures
  sec/                         # Standard RDP security (RC4, key derivation, MAC)
  nla/                         # NLA/CredSSP v6 with NTLMv2 and SPNEGO
  lic/                         # RDP licensing
  pdu/                         # Share Control PDU framing
  caps/                        # Capability set encoding (22 capability types)
  fastpath/                    # Fast-Path output update decoder
  orders/                      # Primary drawing orders (16 order types)
  rle/                         # Interleaved RLE + RDP 6.0 Planar codec
  pointer/                     # Pointer/cursor update decoder
  egfx/                        # RDPGFX graphics pipeline extension
  rfx/                         # RemoteFX Progressive codec
  clearcodec/                  # ClearCodec bitmap decoder
  nscodec/                     # NSCodec bitmap decoder
  zgfx/                        # RDP8 ZGFX bulk compression
  mppc/                        # MPPC bulk decompression (RDP 4.0/5.0)
  dvc/                         # Dynamic Virtual Channel multiplexing
  cliprdr/                     # Clipboard virtual channel
  rdpsnd/                      # Audio output virtual channel
  audin/                       # Audio input virtual channel (MS-RDPEAI)
  rdpdr/                       # Drive redirection virtual channel (MS-RDPEFS)
  urbdrc/                      # USB device redirection virtual channel (MS-RDPEUSB)
  rdpecam/                     # Webcam redirection virtual channel (MS-RDPECAM)
  disp/                        # Display resize virtual channel
display/
  gui/                         # Ebiten-based desktop GUI viewer
  web/                         # Web viewer (HTTP + WebSocket)
  clipboard.go                 # OS clipboard abstraction
  pixconv.go                   # Pixel format conversion
```

## Build & Test

```bash
# Build
go build ./...                        # Build all packages
go build -o gopher-rdp ./client       # Build CLI (web-only, no GUI)

# Test
go test ./...                         # Run all tests
go test -count=1 ./...                # Run all tests (no cache)
go test -cover ./...                  # Tests with coverage
go vet ./...                          # Static analysis

# Build script
./build.sh prod                       # Production GUI+web binary (requires CGO/X11)
./build.sh web                        # Web-only, no GUI/CGO dependency
./build.sh debug                      # Debug build with symbols (for dlv/gdb)
./build.sh all                        # Cross-compile web-only for all platforms into dist/
./build.sh race                       # Web-only with race detector
./build.sh race-gui                   # GUI+web with race detector
./build.sh test                       # Run go vet + go test
./build.sh test-race                  # Run go vet + go test with race detector

# Single-target cross-compilation
./build.sh web windows/amd64          # Windows
./build.sh web windows/arm64          # Windows ARM
./build.sh web darwin/amd64           # macOS Intel
./build.sh web darwin/arm64           # macOS Apple Silicon
./build.sh web linux/arm64            # Linux ARM64
```

## Changelog

### v1.0.3

- **Device redirection fixed on Windows 10 / Server 2016–2022.** The `rdpsnd`
  static channel is now advertised in the GCC Client Network Data whenever any
  device redirection (drive, printer, serial, parallel, or smartcard) is
  configured. Windows 10 and Server 2016–2022 only initialize the `rdpdr` device
  redirector when `rdpsnd` is also present — without it the server joins the
  `rdpdr` channel but never sends the RDPDR Server Announce, so redirection
  silently failed. (Windows 11 dropped this coupling, which is why it worked
  there.) The channel is advertised solely to satisfy this gate; audio still
  flows over the `AUDIO_PLAYBACK_DVC` dynamic channel.
- **Drive redirection:** device announce `DeviceData` is now encoded as ASCII
  rather than UTF-16, matching the MS reference client.

### v1.0.2

- GUI viewer: mouse cursor is now rendered visibly (white background) in editor
  contexts.

### v1.0.1

- GUI viewer DPI behavior aligned with the web client: `-gui WxH` now specifies
  the *logical* window size, with the RDP session rendered at `W*DPR x H*DPR`.
  Both viewers default to nearest-neighbor filtering; `-interpolate` switches to
  bilinear, and `-no-dpr` forces `DPR=1.0`.

## License

MIT License - see [LICENSE](LICENSE)
