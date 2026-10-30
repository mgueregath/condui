package main

import (
	"fmt"
	"strconv"

	"github.com/bouncyball-git/gopher-rdp/protocol/pdu"
)

type RDPInput struct {
	Kind   string `json:"kind"`
	Code   int    `json:"code"`
	Down   bool   `json:"down"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	Button int    `json:"button"`
	Delta  int    `json:"delta"`
}

func (a *App) SendRDPInput(sessionID string, events []RDPInput) error {
	if len(events) > 256 {
		return fmt.Errorf("RDP input batch too large")
	}
	value, ok := a.rdpSessions.Load(sessionID)
	if !ok {
		return fmt.Errorf("RDP session unavailable")
	}
	session := value.(*rdpSession)
	session.inputMu.Lock()
	defer session.inputMu.Unlock()
	for _, event := range events {
		if err := session.input(event); err != nil {
			return err
		}
	}
	return nil
}
func (s *rdpSession) input(event RDPInput) error {
	if s.ctx.Err() != nil {
		return fmt.Errorf("RDP session closed")
	}
	s.frameMu.RLock()
	w, h := s.frame.Width, s.frame.Height
	s.frameMu.RUnlock()
	event.X = max(0, min(event.X, w-1))
	event.Y = max(0, min(event.Y, h-1))
	key := event.Kind + ":" + strconv.Itoa(event.Code)
	switch event.Kind {
	case "key", "unicode":
		if event.Code < 0 || event.Code > 65535 {
			return fmt.Errorf("invalid RDP key")
		}
		if event.Down {
			s.pressed[key] = event
		} else {
			delete(s.pressed, key)
		}
		if event.Kind == "unicode" {
			return s.client.SendUnicode(uint16(event.Code), event.Down)
		}
		return s.client.SendKeyboard(uint16(event.Code), event.Down)
	case "move":
		return s.client.SendMouse(event.X, event.Y, pdu.PtrFlagsMove)
	case "button":
		buttons := []uint16{pdu.PtrFlagsButton1, pdu.PtrFlagsButton3, pdu.PtrFlagsButton2}
		if event.Button < 0 || event.Button >= len(buttons) {
			return fmt.Errorf("invalid RDP mouse button")
		}
		key = "button:" + strconv.Itoa(event.Button)
		flags := buttons[event.Button]
		if event.Down {
			flags |= pdu.PtrFlagsDown
			s.pressed[key] = event
		} else {
			delete(s.pressed, key)
		}
		return s.client.SendMouse(event.X, event.Y, flags)
	case "wheel":
		return s.client.SendMouseWheel(event.X, event.Y, max(-120, min(event.Delta, 120)), false)
	case "release":
		for _, pressed := range s.pressed {
			pressed.Down = false
			if err := s.input(pressed); err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported RDP input")
	}
}
