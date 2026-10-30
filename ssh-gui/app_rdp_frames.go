package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/png"
	"strconv"
	"sync"
	"time"

	rdp "github.com/bouncyball-git/gopher-rdp"
	"github.com/wailsapp/wails/v3/pkg/application"
)

type RDPFrame struct {
	SessionID string `json:"sessionId"`
	Image     string `json:"image,omitempty"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Cursor    string `json:"cursor,omitempty"`
	Sequence  uint64 `json:"sequence"`
}
type rdpSession struct {
	id      string
	client  *rdp.Client
	ctx     context.Context
	cancel  context.CancelFunc
	images  chan *image.RGBA
	frameMu sync.RWMutex
	frame   RDPFrame
	inputMu sync.Mutex
	pressed map[string]RDPInput
}

func newRDPSession(id string, client *rdp.Client, ctx context.Context, cancel context.CancelFunc) *rdpSession {
	return &rdpSession{id: id, client: client, ctx: ctx, cancel: cancel, images: make(chan *image.RGBA, 1), frame: RDPFrame{SessionID: id, Width: 1280, Height: 720, Cursor: "default"}, pressed: make(map[string]RDPInput)}
}

// Called after a paint batch, on the protocol reader's goroutine, before the
// next write to the framebuffer. The queued image owns its pixel data.
func (s *rdpSession) capture() {
	if s.ctx.Err() != nil {
		return
	}
	pixels, w, h := s.client.FramebufferTopDown()
	if w <= 0 || h <= 0 || w > 8192 || h > 8192 || len(pixels) != w*h*4 {
		return
	}
	frame := &image.RGBA{Pix: pixels, Stride: w * 4, Rect: image.Rect(0, 0, w, h)}
	select {
	case s.images <- frame:
	default:
		select {
		case <-s.images:
		default:
		}
		select {
		case s.images <- frame:
		default:
		}
	}
}
func (s *rdpSession) captureCursor(pointer *rdp.PointerUpdate) {
	if pointer == nil || s.ctx.Err() != nil {
		return
	}
	cursor := "default"
	if pointer.Type == rdp.PointerShape && pointer.Width > 0 && pointer.Height > 0 && pointer.Width <= 128 && pointer.Height <= 128 && len(pointer.Data) == int(pointer.Width)*int(pointer.Height)*4 {
		im := &image.RGBA{Pix: pointer.Data, Stride: int(pointer.Width) * 4, Rect: image.Rect(0, 0, int(pointer.Width), int(pointer.Height))}
		var data bytes.Buffer
		if png.Encode(&data, im) == nil {
			cursor = "url(data:image/png;base64," + base64.StdEncoding.EncodeToString(data.Bytes()) + ") " + strconv.Itoa(min(int(pointer.HotSpotX), int(pointer.Width)-1)) + " " + strconv.Itoa(min(int(pointer.HotSpotY), int(pointer.Height)-1)) + ", default"
		}
	}
	// Keep a local pointer even when a server sends an empty cursor.
	s.frameMu.Lock()
	s.frame.Cursor = cursor
	s.frameMu.Unlock()
	application.Get().Event.Emit("rdp-cursor", map[string]any{"sessionId": s.id, "cursor": cursor})
}
func (s *rdpSession) stream() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var latest *image.RGBA
	for {
		select {
		case <-s.ctx.Done():
			return
		case latest = <-s.images:
		case <-ticker.C:
			if latest == nil {
				continue
			}
			var data bytes.Buffer
			if err := png.Encode(&data, latest); err != nil {
				latest = nil
				continue
			}
			s.frameMu.Lock()
			s.frame.Image = base64.StdEncoding.EncodeToString(data.Bytes())
			s.frame.Width = latest.Rect.Dx()
			s.frame.Height = latest.Rect.Dy()
			s.frame.Sequence++
			frame := s.frame
			s.frameMu.Unlock()
			latest = nil
			if s.ctx.Err() == nil {
				application.Get().Event.Emit("rdp-frame", frame)
			}
		}
	}
}
