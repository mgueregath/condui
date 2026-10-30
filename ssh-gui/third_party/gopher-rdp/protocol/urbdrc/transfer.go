package urbdrc

import (
	"context"
	"encoding/binary"
	"fmt"
	"log/slog"
	"time"
)

// TS_URB function codes (MS-RDPEUSB 2.2.10).
const (
	tsURBSelectConfiguration             uint16 = 0x0000
	tsURBSelectInterface                 uint16 = 0x0001
	tsURBPipeRequest                     uint16 = 0x0002
	tsURBGetCurrentFrameNumber           uint16 = 0x0007
	tsURBControlTransfer                 uint16 = 0x0008
	tsURBBulkOrInterruptTransfer         uint16 = 0x0009
	tsURBIsochTransfer                   uint16 = 0x000A
	tsURBGetDescriptorFromDevice         uint16 = 0x000B
	tsURBSetDescriptorToDevice           uint16 = 0x000C
	tsURBSetFeatureToDevice              uint16 = 0x000D
	tsURBSetFeatureToInterface           uint16 = 0x000E
	tsURBSetFeatureToEndpoint            uint16 = 0x000F
	tsURBClearFeatureToDevice            uint16 = 0x0010
	tsURBClearFeatureToInterface         uint16 = 0x0011
	tsURBClearFeatureToEndpoint          uint16 = 0x0012
	tsURBGetStatusFromDevice             uint16 = 0x0013
	tsURBGetStatusFromInterface          uint16 = 0x0014
	tsURBGetStatusFromEndpoint           uint16 = 0x0015
	tsURBVendorDevice                    uint16 = 0x0017
	tsURBVendorInterface                 uint16 = 0x0018
	tsURBVendorEndpoint                  uint16 = 0x0019
	tsURBClassDevice                     uint16 = 0x001A
	tsURBClassInterface                  uint16 = 0x001B
	tsURBClassEndpoint                   uint16 = 0x001C
	tsURBSyncResetPipeAndClearStall      uint16 = 0x001E
	tsURBClassOther                      uint16 = 0x001F
	tsURBVendorOther                     uint16 = 0x0020
	tsURBGetStatusFromOther              uint16 = 0x0021
	tsURBClearFeatureToOther             uint16 = 0x0022
	tsURBSetFeatureToOther               uint16 = 0x0023
	tsURBGetDescriptorFromEndpoint       uint16 = 0x0024
	tsURBSetDescriptorToEndpoint         uint16 = 0x0025
	tsURBControlGetConfigurationRequest  uint16 = 0x0026
	tsURBControlGetInterfaceRequest      uint16 = 0x0027
	tsURBGetDescriptorFromInterface      uint16 = 0x0028
	tsURBSetDescriptorToInterface        uint16 = 0x0029
	tsURBGetOSFeatureDescriptorRequest   uint16 = 0x002A
	tsURBSyncResetPipe                   uint16 = 0x0030
	tsURBSyncClearStall                  uint16 = 0x0031
	tsURBControlTransferEX               uint16 = 0x0032
)

// USBD status codes (MS-RDPEUSB 2.2.10.1.1).
const (
	usbdStatusSuccess           uint32 = 0x00000000
	usbdStatusPending           uint32 = 0x40000000
	usbdStatusCanceled          uint32 = 0xC0010000
	usbdStatusStallPID          uint32 = 0xC0000004
	usbdStatusDevNotResponding  uint32 = 0xC0000005
	usbdStatusNotSupported      uint32 = 0xC0000E00
	usbdStatusInvalidURBFunc    uint32 = 0x80000200
	usbdStatusInvalidParameter  uint32 = 0x80000300
	usbdStatusRequestFailed     uint32 = 0x80000500
)

// USB transfer flags.
const (
	usbdTransferDirection     uint32 = 0x00000001 // 1=IN (device→host)
	usbdShortTransferOK       uint32 = 0x00000002
	usbdStartIsoTransferASAP  uint32 = 0x00000004
	usbdDefaultPipeTransfer   uint32 = 0x00000008
)

// USB standard request codes.
const (
	usbRequestGetStatus        uint8 = 0x00
	usbRequestClearFeature     uint8 = 0x01
	usbRequestSetFeature       uint8 = 0x03
	usbRequestGetDescriptor    uint8 = 0x06
	usbRequestSetDescriptor    uint8 = 0x07
	usbRequestGetConfiguration uint8 = 0x08
	usbRequestGetInterface     uint8 = 0x0A
)

// Pipe operations.
const (
	pipeCancel = 0
	pipeReset  = 1
)

const urbControlTransferExternal = 0x1

// handleTransferRequest dispatches TRANSFER_IN_REQUEST / TRANSFER_OUT_REQUEST.
func (h *Handler) handleTransferRequest(channelID uint32, ds *deviceState, messageID uint32, body []byte, isIn bool) {
	if len(body) < 8 {
		return
	}

	cbTsUrb := binary.LittleEndian.Uint32(body[0:4])

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "transfer parse",
		slog.Int("bodyLen", len(body)),
		slog.String("cbTsUrb", fmt.Sprintf("%d", cbTsUrb)))

	if cbTsUrb < 8 || len(body) < 4+int(cbTsUrb) {
		return
	}

	// TS_URB starts at body[4], layout: CbSize(2) + URB_Function(2) + RequestId(4) + URB-specific
	_ = binary.LittleEndian.Uint16(body[4:6]) // CbSize
	urbFunction := binary.LittleEndian.Uint16(body[6:8])

	if len(body) < 12 {
		return
	}
	requestField := binary.LittleEndian.Uint32(body[8:12])
	noAck := (requestField & 0x80000000) != 0
	requestID := requestField & 0x7FFFFFFF

	// Pass everything after the TS_URB header as urbBody. Handlers read
	// their URB-specific fields followed by OutputBufferSize sequentially,
	// crossing the TS_URB boundary — matching reference implementation
	// behavior (data_transfer.c handlers read from a single stream).
	// For OUT requests, transferData contains the output payload.
	urbBody := body[12:]
	transferData := body[4+cbTsUrb:]

	transferDir := 1 // IN
	if !isIn {
		transferDir = 0 // OUT
	}

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "URB transfer",
		slog.String("function", fmt.Sprintf("%04x", urbFunction)),
		slog.Int("requestId", int(requestID)),
		slog.Bool("noAck", noAck),
		slog.Int("dir", transferDir),
		slog.Int("urbBodyLen", len(urbBody)),
		slog.Int("transferDataLen", len(transferData)))

	switch urbFunction {
	case tsURBSelectConfiguration:
		h.urbSelectConfiguration(channelID, ds, messageID, requestID, noAck, urbBody, transferDir)
	case tsURBSelectInterface:
		h.urbSelectInterface(channelID, ds, messageID, requestID, noAck, urbBody)
	case tsURBPipeRequest:
		h.urbPipeRequest(channelID, ds, messageID, requestID, noAck, urbBody, pipeCancel)
	case tsURBGetCurrentFrameNumber:
		h.urbGetCurrentFrameNumber(channelID, ds, messageID, requestID, noAck, urbBody)
	case tsURBControlTransfer:
		h.urbControlTransfer(channelID, ds, messageID, requestID, noAck, urbBody, transferData, false)
	case tsURBControlTransferEX:
		h.urbControlTransfer(channelID, ds, messageID, requestID, noAck, urbBody, transferData, true)
	case tsURBBulkOrInterruptTransfer:
		h.urbBulkOrInterruptTransfer(channelID, ds, messageID, requestID, noAck, urbBody, transferData)
	case tsURBIsochTransfer:
		h.urbIsochTransfer(channelID, ds, messageID, requestID, noAck, urbBody, transferData)
	case tsURBGetDescriptorFromDevice, tsURBSetDescriptorToDevice:
		h.urbControlDescriptorRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x00, urbFunction)
	case tsURBGetDescriptorFromEndpoint, tsURBSetDescriptorToEndpoint:
		h.urbControlDescriptorRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x02, urbFunction)
	case tsURBGetDescriptorFromInterface, tsURBSetDescriptorToInterface:
		h.urbControlDescriptorRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x01, urbFunction)
	case tsURBSetFeatureToDevice:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x00, false)
	case tsURBSetFeatureToInterface:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x01, false)
	case tsURBSetFeatureToEndpoint:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x02, false)
	case tsURBSetFeatureToOther:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x03, false)
	case tsURBClearFeatureToDevice:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x00, true)
	case tsURBClearFeatureToInterface:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x01, true)
	case tsURBClearFeatureToEndpoint:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x02, true)
	case tsURBClearFeatureToOther:
		h.urbControlFeatureRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x03, true)
	case tsURBGetStatusFromDevice:
		h.urbControlGetStatusRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x00)
	case tsURBGetStatusFromInterface:
		h.urbControlGetStatusRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x01)
	case tsURBGetStatusFromEndpoint:
		h.urbControlGetStatusRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x02)
	case tsURBGetStatusFromOther:
		h.urbControlGetStatusRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, 0x03)
	case tsURBVendorDevice:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, true, 0x00)
	case tsURBVendorInterface:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, true, 0x01)
	case tsURBVendorEndpoint:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, true, 0x02)
	case tsURBVendorOther:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, true, 0x03)
	case tsURBClassDevice:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, false, 0x00)
	case tsURBClassInterface:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, false, 0x01)
	case tsURBClassEndpoint:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, false, 0x02)
	case tsURBClassOther:
		h.urbControlVendorOrClassRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData, false, 0x03)
	case tsURBSyncResetPipeAndClearStall, tsURBSyncResetPipe, tsURBSyncClearStall:
		h.urbPipeRequest(channelID, ds, messageID, requestID, noAck, urbBody, pipeReset)
	case tsURBControlGetConfigurationRequest:
		h.urbControlGetConfigurationRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData)
	case tsURBControlGetInterfaceRequest:
		h.urbControlGetInterfaceRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData)
	case tsURBGetOSFeatureDescriptorRequest:
		h.urbOSFeatureDescriptorRequest(channelID, ds, messageID, requestID, noAck, urbBody, transferData)
	default:
		h.log.LogAttrs(context.Background(), slog.LevelWarn, "unsupported URB function",
			slog.String("function", fmt.Sprintf("%04x", urbFunction)))
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidURBFunc)
	}
}

// --- URB handlers ---

func (h *Handler) urbSelectConfiguration(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody []byte, transferDir int) {
	if transferDir == 0 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	if len(urbBody) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	configDescValid := urbBody[0]
	// 3 bytes padding
	numInterfaces := binary.LittleEndian.Uint32(urbBody[4:8])

	var msConfig *MSUSBConfig
	usbdStatus := usbdStatusSuccess

	if configDescValid != 0 {
		var n int
		msConfig, n = ReadMSUSBConfig(urbBody[8:], numInterfaces)
		if msConfig == nil {
			h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
			return
		}
		_ = n

		if err := ds.dev.SelectConfiguration(msConfig.BConfigurationValue); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "SelectConfiguration failed",
				slog.Any("err", err))
			usbdStatus = usbdStatusRequestFailed
		}

		// Fill in backend-specific handles
		msConfig = ds.dev.CompleteConfig(msConfig)

		// Log the completed config for diagnostics
		h.log.LogAttrs(context.Background(), slog.LevelDebug, "SELECT_CONFIGURATION result",
			slog.String("configHandle", fmt.Sprintf("%08x", msConfig.ConfigurationHandle)),
			slog.Int("numInterfaces", int(msConfig.NumInterfaces)),
			slog.Int("bConfigValue", int(msConfig.BConfigurationValue)))
		for i, ifc := range msConfig.Interfaces {
			h.log.LogAttrs(context.Background(), slog.LevelDebug, "  interface",
				slog.Int("idx", i),
				slog.Int("ifNum", int(ifc.InterfaceNumber)),
				slog.Int("alt", int(ifc.AlternateSetting)),
				slog.String("class", fmt.Sprintf("%02x/%02x/%02x", ifc.BInterfaceClass, ifc.BInterfaceSubClass, ifc.BInterfaceProtocol)),
				slog.String("ifHandle", fmt.Sprintf("%08x", ifc.InterfaceHandle)),
				slog.Int("pipes", int(ifc.NumberOfPipes)))
			for j, p := range ifc.Pipes {
				h.log.LogAttrs(context.Background(), slog.LevelDebug, "    pipe",
					slog.Int("idx", j),
					slog.String("ep", fmt.Sprintf("%02x", p.BEndpointAddress)),
					slog.Int("type", int(p.PipeType)),
					slog.String("handle", fmt.Sprintf("%08x", p.PipeHandle)),
					slog.Int("maxPkt", int(p.MaximumPacketSize)),
					slog.Int("maxXfer", int(p.MaximumTransferSize)))
			}
		}
	}

	// Send TS_URB_SELECT_CONFIGURATION_RESULT
	h.sendSelectConfigurationResult(channelID, ds, messageID, requestID, noAck, usbdStatus, msConfig)
}

func (h *Handler) sendSelectConfigurationResult(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, usbdStatus uint32, msConfig *MSUSBConfig) {
	interfaceID := streamIDProxy | ds.reqCompletion

	var configData []byte
	if msConfig != nil {
		configData = WriteMSUSBConfig(msConfig)
	} else {
		configData = make([]byte, 8) // ConfigurationHandle(4)=0 + NumInterfaces(4)=0
	}

	cbTsUrbResult := uint16(8 + len(configData))

	// URB_COMPLETION_NO_DATA header (36 bytes) + config data
	buf := make([]byte, 12+4+4+8+4+4+len(configData))
	off := 0

	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], urbCompletionNoDataFn)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], uint32(cbTsUrbResult))
	off += 4
	// TS_URB_RESULT_HEADER
	binary.LittleEndian.PutUint16(buf[off:], cbTsUrbResult)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // Reserved
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatus)
	off += 4
	// Config data
	copy(buf[off:], configData)
	off += len(configData)
	// HResult + OutputBufferSize
	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // OutputBufferSize
	off += 4

	if !noAck {
		if err := h.dvcSend(channelID, buf[:off]); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send SELECT_CONFIGURATION result",
				slog.Any("err", err))
		}
	}
}

func (h *Handler) urbSelectInterface(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody []byte) {
	if len(urbBody) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	// ConfigurationHandle
	_ = binary.LittleEndian.Uint32(urbBody[0:4])

	iface, _ := readMSUSBInterface(urbBody[4:])
	if iface == nil {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	usbdStatus := usbdStatusSuccess
	if err := ds.dev.SelectInterface(iface.InterfaceNumber, iface.AlternateSetting); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "SelectInterface failed",
			slog.Any("err", err))
		usbdStatus = usbdStatusRequestFailed
	}

	// Fill in backend-specific pipe info
	activeCfg := ds.dev.GetActiveConfig()
	if activeCfg != nil {
		for _, ai := range activeCfg.Interfaces {
			if ai.InterfaceNumber == iface.InterfaceNumber {
				iface.InterfaceHandle = ai.InterfaceHandle
				iface.BInterfaceClass = ai.BInterfaceClass
				iface.BInterfaceSubClass = ai.BInterfaceSubClass
				iface.BInterfaceProtocol = ai.BInterfaceProtocol
				iface.Pipes = ai.Pipes
				iface.NumberOfPipes = ai.NumberOfPipes
				break
			}
		}
	}

	// Send TS_URB_SELECT_INTERFACE_RESULT
	ifaceData := WriteMSUSBInterfaceResult(iface)
	cbTsUrbResult := uint16(8 + len(ifaceData))

	interfaceID := streamIDProxy | ds.reqCompletion
	buf := make([]byte, 12+4+4+8+len(ifaceData)+8)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], urbCompletionNoDataFn)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], uint32(cbTsUrbResult))
	off += 4
	binary.LittleEndian.PutUint16(buf[off:], cbTsUrbResult)
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0)
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatus)
	off += 4
	copy(buf[off:], ifaceData)
	off += len(ifaceData)
	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // OutputBufferSize
	off += 4

	if !noAck {
		if err := h.dvcSend(channelID, buf[:off]); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send SELECT_INTERFACE result",
				slog.Any("err", err))
		}
	}
}

func (h *Handler) urbControlTransfer(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte, external bool) {
	if len(urbBody) < 16 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	pipeHandle := binary.LittleEndian.Uint32(urbBody[0:4])
	transferFlags := binary.LittleEndian.Uint32(urbBody[4:8])

	off := 8
	timeout := uint32(0)
	if external {
		if len(urbBody) < 20 {
			h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
			return
		}
		timeout = binary.LittleEndian.Uint32(urbBody[off : off+4])
		off += 4
	}

	if len(urbBody[off:]) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	bmRequestType := urbBody[off]
	bRequest := urbBody[off+1]
	wValue := binary.LittleEndian.Uint16(urbBody[off+2 : off+4])
	wIndex := binary.LittleEndian.Uint16(urbBody[off+4 : off+6])
	// wLength at [off+6:off+8]
	off += 8

	if len(urbBody[off:]) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	outputBufSize := binary.LittleEndian.Uint32(urbBody[off : off+4])

	_ = pipeHandle
	if timeout == 0 {
		timeout = 2000 // 2 second default
	}

	isIn := (transferFlags & usbdTransferDirection) != 0

	var dataBuf []byte
	if isIn {
		dataBuf = make([]byte, outputBufSize)
	} else {
		dataBuf = transferData
	}

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, bRequest, wValue, wIndex, dataBuf, timeout)
	if n < 0 {
		n = 0
	}

	if isIn && n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbBulkOrInterruptTransfer(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte) {
	if len(urbBody) < 12 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	pipeHandle := binary.LittleEndian.Uint32(urbBody[0:4])
	transferFlags := binary.LittleEndian.Uint32(urbBody[4:8])
	outputBufSize := binary.LittleEndian.Uint32(urbBody[8:12])

	endpointAddr := uint8(pipeHandle & 0xFF)
	isIn := (transferFlags & usbdTransferDirection) != 0

	timeout := uint32(10000) // 10 seconds

	var dataBuf []byte
	if isIn {
		dataBuf = make([]byte, outputBufSize)
	} else {
		// Copy transferData — the caller's buffer may be reused after return.
		dataBuf = make([]byte, len(transferData))
		copy(dataBuf, transferData)
	}

	// Run in a goroutine to avoid blocking the receive loop.
	// Mass storage SCSI commands can take hundreds of milliseconds.
	// Acquire semaphore to bound concurrent transfer goroutines.
	select {
	case h.transferSem <- struct{}{}:
	default:
		// Too many concurrent transfers — reject immediately.
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	go func() {
		defer func() { <-h.transferSem }()
		n, usbdStatus := ds.dev.BulkOrInterruptTransfer(endpointAddr, dataBuf, timeout)
		if n < 0 {
			n = 0
		}

		if isIn && n > 0 {
			h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
		} else {
			h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
		}
	}()
}

func (h *Handler) urbIsochTransfer(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte) {
	if len(urbBody) < 20 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	pipeHandle := binary.LittleEndian.Uint32(urbBody[0:4])
	transferFlags := binary.LittleEndian.Uint32(urbBody[4:8])
	startFrame := binary.LittleEndian.Uint32(urbBody[8:12])
	numPackets := binary.LittleEndian.Uint32(urbBody[12:16])
	errorCount := binary.LittleEndian.Uint32(urbBody[16:20])

	_ = errorCount

	// Read packet descriptors (12 bytes each)
	pktDescOff := 20
	if len(urbBody[pktDescOff:]) < int(numPackets)*12 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	packets := make([]IsochPacket, numPackets)
	for i := uint32(0); i < numPackets; i++ {
		off := pktDescOff + int(i)*12
		packets[i].Offset = binary.LittleEndian.Uint32(urbBody[off : off+4])
		packets[i].Length = binary.LittleEndian.Uint32(urbBody[off+4 : off+8])
		packets[i].Status = binary.LittleEndian.Uint32(urbBody[off+8 : off+12])
	}

	pktEnd := pktDescOff + int(numPackets)*12
	if len(urbBody[pktEnd:]) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}
	// outputBufSize
	_ = binary.LittleEndian.Uint32(urbBody[pktEnd : pktEnd+4])

	endpointAddr := uint8(pipeHandle & 0xFF)
	timeout := uint32(5000)

	results, outData, usbdStatus := ds.dev.IsochTransfer(endpointAddr, transferFlags, startFrame, packets, transferData, timeout)

	// Build isoch completion
	h.sendIsochCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, startFrame, results, outData)
}

func (h *Handler) urbPipeRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody []byte, op int) {
	if len(urbBody) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	pipeHandle := binary.LittleEndian.Uint32(urbBody[0:4])
	endpointAddr := uint8(pipeHandle & 0xFF)

	switch op {
	case pipeCancel:
		ds.dev.CancelTransfer(0) // cancel all on this pipe
	case pipeReset:
		if err := ds.dev.ClearHalt(endpointAddr); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelWarn, "ClearHalt failed",
				slog.Any("err", err))
		}
	}

	h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusSuccess)
}

func (h *Handler) urbGetCurrentFrameNumber(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody []byte) {
	if len(urbBody) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	frameNumber := uint32(time.Now().UnixMilli())
	interfaceID := streamIDProxy | ds.reqCompletion

	// CbTsUrbResult = 12 (header 8 + FrameNumber 4)
	buf := make([]byte, 12+4+4+12+8)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], urbCompletionNoDataFn)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 12) // CbTsUrbResult
	off += 4
	binary.LittleEndian.PutUint16(buf[off:], 12) // Size
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // Reserved
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatusSuccess)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], frameNumber)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // OutputBufferSize
	off += 4

	if !noAck {
		if err := h.dvcSend(channelID, buf[:off]); err != nil {
			h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send GET_CURRENT_FRAME_NUMBER",
				slog.Any("err", err))
		}
	}
}

func (h *Handler) urbControlDescriptorRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte, recipient uint8, urbFunc uint16) {
	if len(urbBody) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	descIndex := urbBody[0]
	descType := urbBody[1]
	langID := binary.LittleEndian.Uint16(urbBody[2:4])
	outputBufSize := binary.LittleEndian.Uint32(urbBody[4:8])

	// Determine if this is a GET or SET descriptor
	isGet := (urbFunc == tsURBGetDescriptorFromDevice || urbFunc == tsURBGetDescriptorFromEndpoint || urbFunc == tsURBGetDescriptorFromInterface)

	wValue := uint16(descType)<<8 | uint16(descIndex)
	var bmRequestType uint8
	if isGet {
		bmRequestType = 0x80 | recipient // IN
	} else {
		bmRequestType = 0x00 | recipient // OUT
	}

	var dataBuf []byte
	if isGet {
		dataBuf = make([]byte, outputBufSize)
	} else {
		dataBuf = transferData
	}

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "ControlTransfer descriptor",
		slog.String("bmRequestType", fmt.Sprintf("%02x", bmRequestType)),
		slog.String("wValue", fmt.Sprintf("%04x", wValue)),
		slog.String("wIndex", fmt.Sprintf("%04x", langID)),
		slog.Int("bufSize", len(dataBuf)),
		slog.Bool("isGet", isGet))

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, usbRequestGetDescriptor, wValue, langID, dataBuf, 2000)

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "ControlTransfer descriptor result",
		slog.Int("n", n),
		slog.String("usbdStatus", fmt.Sprintf("%08x", usbdStatus)))

	if n < 0 {
		n = 0
	}

	if isGet && n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbControlGetStatusRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte, recipient uint8) {
	if len(urbBody) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	index := binary.LittleEndian.Uint16(urbBody[0:2])
	// 2 bytes padding
	outputBufSize := binary.LittleEndian.Uint32(urbBody[4:8])

	bmRequestType := uint8(0x80) | recipient
	dataBuf := make([]byte, outputBufSize)

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, usbRequestGetStatus, 0, index, dataBuf, 2000)
	if n < 0 {
		n = 0
	}

	if n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbControlFeatureRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte, recipient uint8, isClear bool) {
	if len(urbBody) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	featureSelector := binary.LittleEndian.Uint16(urbBody[0:2])
	index := binary.LittleEndian.Uint16(urbBody[2:4])
	// outputBufSize at [4:8]

	var bRequest uint8
	if isClear {
		bRequest = usbRequestClearFeature
	} else {
		bRequest = usbRequestSetFeature
	}

	bmRequestType := recipient // OUT direction
	dataBuf := transferData

	_, usbdStatus := ds.dev.ControlTransfer(bmRequestType, bRequest, featureSelector, index, dataBuf, 2000)
	h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
}

func (h *Handler) urbControlVendorOrClassRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte, isVendor bool, recipient uint8) {
	if len(urbBody) < 16 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	transferFlags := binary.LittleEndian.Uint32(urbBody[0:4])
	_ = urbBody[4] // ReqTypeReservedBits
	bRequest := urbBody[5]
	wValue := binary.LittleEndian.Uint16(urbBody[6:8])
	wIndex := binary.LittleEndian.Uint16(urbBody[8:10])
	// 2 bytes padding at [10:12]
	outputBufSize := binary.LittleEndian.Uint32(urbBody[12:16])

	var reqType uint8
	if isVendor {
		reqType = 0x02 << 5 // Vendor
	} else {
		reqType = 0x01 << 5 // Class
	}

	bmRequestType := reqType | recipient
	isIn := (transferFlags & usbdTransferDirection) != 0
	if isIn {
		bmRequestType |= 0x80
	}

	var dataBuf []byte
	if isIn {
		dataBuf = make([]byte, outputBufSize)
	} else {
		dataBuf = transferData
	}

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, bRequest, wValue, wIndex, dataBuf, 2000)
	if n < 0 {
		n = 0
	}

	if isIn && n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbControlGetConfigurationRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte) {
	if len(urbBody) < 4 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	outputBufSize := binary.LittleEndian.Uint32(urbBody[0:4])
	dataBuf := make([]byte, outputBufSize)
	bmRequestType := uint8(0x80) // IN, Standard, Device

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, usbRequestGetConfiguration, 0, 0, dataBuf, 2000)
	if n < 0 {
		n = 0
	}

	if n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbControlGetInterfaceRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte) {
	if len(urbBody) < 8 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	interfaceNr := binary.LittleEndian.Uint16(urbBody[0:2])
	// 2 bytes padding
	outputBufSize := binary.LittleEndian.Uint32(urbBody[4:8])

	dataBuf := make([]byte, outputBufSize)
	bmRequestType := uint8(0x80 | 0x01) // IN, Standard, Interface

	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, usbRequestGetInterface, 0, interfaceNr, dataBuf, 2000)
	if n < 0 {
		n = 0
	}

	if n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

func (h *Handler) urbOSFeatureDescriptorRequest(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, urbBody, transferData []byte) {
	if len(urbBody) < 12 {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatusInvalidParameter)
		return
	}

	recipient := urbBody[0] & 0x1F
	interfaceNumber := urbBody[1]
	msPageIndex := urbBody[2]
	msFeatureDescIndex := binary.LittleEndian.Uint16(urbBody[3:5])
	// 3 bytes padding at [5:8]
	outputBufSize := binary.LittleEndian.Uint32(urbBody[8:12])

	_ = msPageIndex

	// MS OS feature descriptor uses vendor request 0xee
	bmRequestType := uint8(0x80) | (0x02 << 5) | recipient // IN, Vendor
	wValue := uint16(interfaceNumber)<<8 | uint16(msPageIndex)

	dataBuf := make([]byte, outputBufSize)
	n, usbdStatus := ds.dev.ControlTransfer(bmRequestType, 0x01, wValue, msFeatureDescIndex, dataBuf, 2000)
	if n < 0 {
		n = 0
	}

	if n > 0 {
		h.sendURBCompletion(channelID, ds, messageID, requestID, noAck, usbdStatus, dataBuf[:n])
	} else {
		h.sendURBCompletionNoData(channelID, ds, messageID, requestID, noAck, usbdStatus)
	}
}

// --- Completion message builders ---

// sendURBCompletion sends URB_COMPLETION (with output data).
func (h *Handler) sendURBCompletion(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, usbdStatus uint32, outputData []byte) {
	if noAck {
		return
	}

	interfaceID := streamIDProxy | ds.reqCompletion
	outputSize := uint32(len(outputData))

	buf := make([]byte, 36+outputSize)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], urbCompletionFn)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 8) // CbTsUrbResult
	off += 4
	// TS_URB_RESULT_HEADER
	binary.LittleEndian.PutUint16(buf[off:], 8) // Size
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // Reserved
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatus)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], outputSize)
	off += 4
	copy(buf[off:], outputData)

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "sending URB_COMPLETION",
		slog.Int("requestId", int(requestID)),
		slog.String("usbdStatus", fmt.Sprintf("%08x", usbdStatus)),
		slog.Int("outputLen", len(outputData)))

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send URB_COMPLETION",
			slog.Any("err", err))
	}
}

// sendURBCompletionNoData sends URB_COMPLETION_NO_DATA.
func (h *Handler) sendURBCompletionNoData(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, usbdStatus uint32) {
	if noAck {
		return
	}

	interfaceID := streamIDProxy | ds.reqCompletion

	buf := make([]byte, 36)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], urbCompletionNoDataFn)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 8) // CbTsUrbResult
	off += 4
	binary.LittleEndian.PutUint16(buf[off:], 8) // Size
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0) // Reserved
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatus)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], 0) // OutputBufferSize

	h.log.LogAttrs(context.Background(), slog.LevelDebug, "sending URB_COMPLETION_NO_DATA",
		slog.Int("requestId", int(requestID)),
		slog.String("usbdStatus", fmt.Sprintf("%08x", usbdStatus)))

	if err := h.dvcSend(channelID, buf); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send URB_COMPLETION_NO_DATA",
			slog.Any("err", err))
	}
}

// sendIsochCompletion sends an isochronous transfer completion.
func (h *Handler) sendIsochCompletion(channelID uint32, ds *deviceState, messageID, requestID uint32, noAck bool, usbdStatus, startFrame uint32, results []IsochPacketResult, outputData []byte) {
	if noAck {
		return
	}

	interfaceID := streamIDProxy | ds.reqCompletion
	numPackets := uint32(len(results))
	errCount := uint32(0)
	for _, r := range results {
		if r.Status != 0 {
			errCount++
		}
	}

	var pktDataSize uint32
	if usbdStatus == usbdStatusSuccess {
		pktDataSize = numPackets * 12
	}
	cbTsUrbResult := uint32(20 + pktDataSize)
	outputSize := uint32(len(outputData))

	funcID := urbCompletionFn
	if outputSize == 0 {
		funcID = urbCompletionNoDataFn
	}

	buf := make([]byte, 12+4+4+cbTsUrbResult+8+outputSize)
	off := 0
	binary.LittleEndian.PutUint32(buf[off:], interfaceID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], messageID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], funcID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], requestID)
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], cbTsUrbResult)
	off += 4
	// TS_URB_RESULT_HEADER
	binary.LittleEndian.PutUint16(buf[off:], uint16(cbTsUrbResult))
	off += 2
	binary.LittleEndian.PutUint16(buf[off:], 0)
	off += 2
	binary.LittleEndian.PutUint32(buf[off:], usbdStatus)
	off += 4
	// Isoch-specific fields
	binary.LittleEndian.PutUint32(buf[off:], startFrame)
	off += 4
	if usbdStatus == usbdStatusSuccess {
		binary.LittleEndian.PutUint32(buf[off:], numPackets)
		off += 4
	}
	binary.LittleEndian.PutUint32(buf[off:], errCount)
	off += 4

	if usbdStatus == usbdStatusSuccess {
		for _, r := range results {
			binary.LittleEndian.PutUint32(buf[off:], r.Status)
			off += 4
			binary.LittleEndian.PutUint32(buf[off:], r.Length)
			off += 4
			binary.LittleEndian.PutUint32(buf[off:], 0) // Reserved
			off += 4
		}
	}

	binary.LittleEndian.PutUint32(buf[off:], 0) // HResult
	off += 4
	binary.LittleEndian.PutUint32(buf[off:], outputSize)
	off += 4
	if outputSize > 0 {
		copy(buf[off:], outputData)
		off += int(outputSize)
	}

	if err := h.dvcSend(channelID, buf[:off]); err != nil {
		h.log.LogAttrs(context.Background(), slog.LevelError, "failed to send ISOCH_COMPLETION",
			slog.Any("err", err))
	}
}
