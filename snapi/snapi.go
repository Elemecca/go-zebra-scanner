package snapi

import (
	"encoding/binary"
	"github.com/karalabe/hid"
	log "github.com/sirupsen/logrus"
)

const maxReportSize = 64

const (
	inMsgStatus    = 0x21
	inMsgScan      = 0x22
	inMsgScanLarge = 0x26
	inMsgNotify    = 0x24
)

const (
	outMsgStatus = 0x01
	outMsgAim    = 0x02
	outMsgLights = 0x10
	outMsgMode   = 0x03
	outMsgBeep   = 0x04
)

const (
	statusSuccess     = 0x1
	statusError       = 0x2
	statusUnsupported = 0x3
	statusBadState    = 0x4
	statusTimeout     = 0xD
)

type DeviceInfo struct {
	hid hid.DeviceInfo
}

func Enumerate() []DeviceInfo {
	devs := hid.Enumerate(0x05e0, 0x1900)
	out := make([]DeviceInfo, 0, len(devs))
	for _, info := range devs {
		out = append(out, DeviceInfo{hid: info})
	}
	return out
}

type ackMsg struct {
	cmdId  byte
	status byte
	param  byte
}

type ScanEvent struct {
	PrimaryType      string
	PrimaryData      []byte
	SupplementalType string
	SupplementalData []byte
}

type scanPacket struct {
	packetCount uint16
	packetIndex uint16
	codeType    uint16
	data        []byte
}

type Device struct {
	hid        *hid.Device
	EventChan  chan interface{}
	ackInChan  chan ackMsg
	ackOutChan chan []byte
	scan       scanPacket
}

func (info DeviceInfo) Open() (*Device, error) {
	hidDev, err := info.hid.Open()
	if err != nil {
		return nil, err
	}

	dev := &Device{
		hid:        hidDev,
		EventChan:  make(chan interface{}, 20),
		ackInChan:  make(chan ackMsg),
		ackOutChan: make(chan []byte),
	}

	go dev.readLoop()
	go dev.writeLoop()

	return dev, nil
}

func (dev *Device) writeStatus(cmdId byte, status byte) {
	dev.ackOutChan <- []byte{0, outMsgStatus, cmdId, status & 0x0f, 0}
}

func (dev *Device) writeAck(cmdId byte) {
	dev.writeStatus(cmdId, statusSuccess)
}

func (dev *Device) clearScan() {
	dev.scan.packetCount = 0
	dev.scan.packetIndex = 0
	dev.scan.codeType = 0
	dev.scan.data = nil
}

func (dev *Device) handleScan(packet scanPacket) {
	if log.IsLevelEnabled(log.DebugLevel) {
		log.WithFields(log.Fields{
			"packetCount": packet.packetCount,
			"packetIndex": packet.packetIndex,
			"codeType":    packet.codeType,
			"data":        packet.data,
		}).Debug("received scan packet")
	}

	if dev.scan.data == nil {
		log.Debug("starting new scan")
		dev.scan.packetCount = packet.packetCount
		dev.scan.packetIndex = 0
		dev.scan.codeType = packet.codeType
		dev.scan.data = make([]byte, 0, int(packet.packetCount)*len(packet.data))
	}

	// grumble. can't wrap this to my satisfaction in the if itself in Go.
	bad := packet.packetCount != dev.scan.packetCount ||
		packet.packetIndex != dev.scan.packetIndex ||
		packet.codeType != dev.scan.codeType
	if bad {
		log.WithFields(log.Fields{
			"expectPacketCount": dev.scan.packetCount,
			"expectPacketIndex": dev.scan.packetIndex,
			"actualPacketCount": packet.packetCount,
			"actualPacketIndex": packet.packetIndex,
			"expectCodeType":    dev.scan.codeType,
			"actualCodeType":    packet.codeType,
		}).Warn("received unexpected scan packet, resetting scan")
		dev.clearScan()
		return
	}

	dev.scan.packetIndex++
	dev.scan.data = append(dev.scan.data, packet.data...)

	if dev.scan.packetIndex >= dev.scan.packetCount {
		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(log.Fields{
				"codeType": dev.scan.codeType,
				"data":     dev.scan.data,
			}).Debug("scan complete")
		}

		codeType, hit := CodeTypeTable[dev.scan.codeType]
		if !hit {
			codeType = CodeType{"unknown", ""}
			log.WithFields(log.Fields{
				"codeType": dev.scan.codeType,
			}).Warn("received unknown codeType")
		}

		primaryData := dev.scan.data
		var supplementalData []byte

		primaryLen, hit := PrimaryLengthTable[codeType.primary]
		if hit {
			supplementalData = primaryData[primaryLen:]
			primaryData = primaryData[:primaryLen]
		}

		dev.EventChan <- ScanEvent{
			PrimaryType:      codeType.primary,
			PrimaryData:      primaryData,
			SupplementalType: codeType.supplemental,
			SupplementalData: supplementalData,
		}

		dev.clearScan()
	}
}

func (dev *Device) readLoop() {
	for {
		report := make([]byte, maxReportSize)
		_, err := dev.hid.Read(report)
		if err != nil {
			// FIXME: signal error and close device
			log.Error("HID read failed:", err)
			continue
		}

		cmdId := report[0]

		switch cmdId {
		case inMsgStatus:
			dev.ackInChan <- ackMsg{
				cmdId:  report[1],
				status: report[2] & 0x0f,
				param:  report[3] & 0x0f,
			}

		case inMsgScan:
			dev.handleScan(scanPacket{
				packetCount: uint16(report[1]),
				packetIndex: uint16(report[2]),
				codeType:    binary.LittleEndian.Uint16(report[4:]),
				data:        report[6 : 6+report[3]],
			})
			dev.writeAck(cmdId)

		case inMsgScanLarge:
			dev.handleScan(scanPacket{
				packetCount: binary.BigEndian.Uint16(report[1:]),
				packetIndex: binary.BigEndian.Uint16(report[3:]),
				codeType:    binary.LittleEndian.Uint16(report[6:]),
				data:        report[8 : 8+report[5]],
			})
			dev.writeAck(cmdId)

		default:
			log.WithFields(log.Fields{
				"commandId": cmdId,
			}).Warn("received unrecognized report")
		}
	}
}

func (dev *Device) writeLoop() {
	for {
		var msg []byte
		select {
		case msg = <-dev.ackOutChan:
		}

		if log.IsLevelEnabled(log.DebugLevel) {
			log.WithFields(log.Fields{
				"length": len(msg),
				"data":   msg,
			}).Debug("sending command")
		}

		count, err := dev.hid.Write(msg)
		if err != nil {
			// FIXME: signal error and close device
			log.Error("HID write failed:", err)
		} else if count != len(msg) {
			// FIXME: signal error and close device
			log.WithFields(log.Fields{
				"expectLength": len(msg),
				"actualLength": count,
			}).Error("HID write length mismatch")
		}
	}
}
