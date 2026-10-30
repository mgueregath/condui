//go:build !linux && !darwin

package rdpdr

import "errors"

// NewPCSCLiteBackend returns an error on unsupported platforms.
func NewPCSCLiteBackend(socketPath string) (*PCSCLiteBackend, error) {
	return nil, errors.New("pcsclite not supported on this platform")
}

// PCSCLiteBackend is a stub for unsupported platforms.
type PCSCLiteBackend struct{}

func (p *PCSCLiteBackend) EstablishContext(scope uint32) (uint32, uint32)   { return 0, ScardENoService }
func (p *PCSCLiteBackend) ReleaseContext(ctx uint32) uint32                  { return ScardENoService }
func (p *PCSCLiteBackend) IsValidContext(ctx uint32) uint32                  { return ScardENoService }
func (p *PCSCLiteBackend) ListReaders(ctx uint32, groups []byte) ([]byte, uint32) {
	return nil, ScardENoService
}
func (p *PCSCLiteBackend) GetStatusChange(ctx uint32, timeout uint32, states []ScardReaderState) ([]ScardReaderState, uint32) {
	return nil, ScardENoService
}
func (p *PCSCLiteBackend) Connect(ctx uint32, reader string, shareMode, preferredProtocol uint32) (uint32, uint32, uint32) {
	return 0, 0, ScardENoService
}
func (p *PCSCLiteBackend) Disconnect(handle uint32, disposition uint32) uint32 { return ScardENoService }
func (p *PCSCLiteBackend) Reconnect(handle uint32, shareMode, preferredProtocol, disposition uint32) (uint32, uint32) {
	return 0, ScardENoService
}
func (p *PCSCLiteBackend) BeginTransaction(handle uint32) uint32            { return ScardENoService }
func (p *PCSCLiteBackend) EndTransaction(handle uint32, disposition uint32) uint32 {
	return ScardENoService
}
func (p *PCSCLiteBackend) Status(handle uint32) (string, uint32, uint32, []byte, uint32) {
	return "", 0, 0, nil, ScardENoService
}
func (p *PCSCLiteBackend) Transmit(handle uint32, sendPCI, sendBuf []byte) ([]byte, []byte, uint32) {
	return nil, nil, ScardENoService
}
func (p *PCSCLiteBackend) Control(handle uint32, controlCode uint32, inBuf []byte) ([]byte, uint32) {
	return nil, ScardENoService
}
func (p *PCSCLiteBackend) GetAttrib(handle uint32, attrID uint32) ([]byte, uint32) {
	return nil, ScardENoService
}
func (p *PCSCLiteBackend) Cancel(ctx uint32) uint32 { return ScardENoService }
func (p *PCSCLiteBackend) Close() error             { return nil }
