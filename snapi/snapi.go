package snapi

import (
	"encoding/binary"
	"encoding/hex"
	//"errors"
	"github.com/karalabe/hid"
	"log"
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
	CodeType uint16
	Data     []byte
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
		hid:         hidDev,
		EventChan:   make(chan interface{}, 20),
		ackInChan:   make(chan ackMsg),
		ackOutChan:  make(chan []byte),
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
	log.Printf(
		"snapi: debug: received scan packet %d/%d, codeType %04x, size %d\n%s",
		packet.packetIndex + 1, packet.packetCount, packet.codeType,
		len(packet.data), hex.Dump(packet.data),
	)

	if dev.scan.data == nil {
		log.Println("snapi: debug: starting new scan")
		dev.scan.packetCount = packet.packetCount
		dev.scan.packetIndex = 0
		dev.scan.codeType = packet.codeType
		dev.scan.data = make([]byte, 0, int(packet.packetCount)*len(packet.data))
	} else {
		if packet.packetCount != dev.scan.packetCount {
			// TODO: error packet packetCount mismatch
			log.Printf(
				"snapi: error: received scan packet %d/%d, expected %d/%d",
				packet.packetIndex + 1, packet.packetCount,
				dev.scan.packetIndex + 1, dev.scan.packetCount,
			)
			dev.clearScan()
			return
		}

		if packet.codeType != dev.scan.codeType {
			// TODO: error packet codeType mismatch
			log.Printf(
				"snapi: error: received scan packet %d/%d with codeType %04x, expected %04x",
				packet.packetIndex + 1, packet.packetCount,
				packet.codeType, dev.scan.codeType,
			)
			dev.clearScan()
			return
		}
	}

	if packet.packetIndex != dev.scan.packetIndex {
		// TODO: error packet packetIndex mismatch
		log.Printf(
			"snapi: error: received scan packet %d/%d, expected %d/%d",
			packet.packetIndex + 1, packet.packetCount,
			dev.scan.packetIndex + 1, dev.scan.packetCount,
		)
		dev.clearScan()
		return
	}

	dev.scan.packetIndex++
	dev.scan.data = append(dev.scan.data, packet.data...)

	if dev.scan.packetIndex >= dev.scan.packetCount {
		log.Printf(
			"snapi: debug: scan complete: code type %04x, length %d\n%s",
			dev.scan.codeType, len(dev.scan.data), hex.Dump(dev.scan.data),
		)

		dev.EventChan <- ScanEvent{
			CodeType: dev.scan.codeType,
			Data:     dev.scan.data,
		}

		dev.clearScan()
	}
}

func (dev *Device) readLoop() {
	for {
		report := make([]byte, maxReportSize)
		_, err := dev.hid.Read(report)
		if err != nil {
			log.Println("snapi: error: HID read failed:", err.Error())
			log.Println("snapi: read loop shutting down")
			break
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
			log.Println("snapi: warning: received unknown report", cmdId)
		}
	}
}

func (dev *Device) writeLoop() {
	for {
		var msg []byte
		select {
		case msg = <- dev.ackOutChan:
		}

		log.Printf(
			"snapi: debug: sending command: length %d\n%s",
			len(msg), hex.Dump(msg),
		)

		count, err := dev.hid.Write(msg)
		if err != nil {
			log.Println("snapi: error: write failed:", err.Error())
		} else if count != len(msg) {
			log.Printf(
				"snapi: error: write length mismatch: expected %d, wrote %d",
				len(msg), count,
			)
		}
	}
}
