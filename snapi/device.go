package snapi

import (
	"encoding/binary"
	"errors"
	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
)

// UsbVid is the USB idVendor value for SNAPI devices
const UsbVid = 0x05e0

// UsbPid is the USB idProduct value for SNAPI devices
const UsbPid = 0x1900

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

type ackMsg struct {
	cmdID  byte
	status byte
	param  byte
}

// DeviceClosedEvent indicates that the device has been closed.
// This can happen as a result of calling Close or if an I/O error
// causes the device to be closed automatically.
type DeviceClosedEvent struct {
	Device *Device
}

// Device represents a device that has been opened.
type Device struct {
	usbDev    *gousb.Device
	usbConfig *gousb.Config
	hidIface  *gousb.Interface
	hidIn     *gousb.InEndpoint

	eventChan chan<- interface{}

	scan       scanPacket
	ackInChan  chan ackMsg
	ackOutChan chan []byte

	closeChan chan bool
	closing   bool

	log *log.Entry
}

// OpenUsbDevice connects to a gousb Device
func OpenUsbDevice(
	usbDev *gousb.Device,
	eventChan chan<- interface{},
) (*Device, error) {
	if usbDev.Desc.Vendor != UsbVid || usbDev.Desc.Product != UsbPid {
		return nil, errors.New("given device is not a SNAPI device")
	}

	dev := &Device{
		usbDev:     usbDev,
		eventChan:  eventChan,
		closeChan:  make(chan bool, 5),
		ackInChan:  make(chan ackMsg),
		ackOutChan: make(chan []byte),
	}

	dev.log = log.WithFields(log.Fields{
		"bus":     usbDev.Desc.Bus,
		"address": usbDev.Desc.Address,
	})

	err := usbDev.SetAutoDetach(true)
	if err != nil {
		return nil, err
	}

	configID, err := usbDev.ActiveConfigNum()
	if err != nil {
		return nil, err
	}

	dev.usbConfig, err = usbDev.Config(configID)
	if err != nil {
		return nil, err
	}

	// find and claim the HID interface
	for _, iface := range dev.usbConfig.Desc.Interfaces {
		for _, alt := range iface.AltSettings {
			if alt.Class == 3 /* HID */ {
				dev.hidIface, err = dev.usbConfig.Interface(iface.Number, alt.Number)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}
	if dev.hidIface == nil {
		return nil, errors.New("HID interface not found")
	}

	// find and claim the HID interrupt in endpoint
	for _, ep := range dev.hidIface.Setting.Endpoints {
		if ep.TransferType == gousb.TransferTypeInterrupt {
			if ep.Direction == gousb.EndpointDirectionIn {
				dev.hidIn, err = dev.hidIface.InEndpoint(ep.Number)
				if err != nil {
					return nil, err
				}
				break
			}
		}
	}
	if dev.hidIn == nil {
		return nil, errors.New("HID interrupt IN endpoint not found")
	}

	go dev.readLoop()
	go dev.writeLoop()

	dev.log.Debug("snapi: device opened")

	return dev, nil
}

func (dev *Device) writeStatus(cmdID byte, status byte) {
	dev.ackOutChan <- []byte{outMsgStatus, cmdID, status & 0x0f, 0}
}

func (dev *Device) writeAck(cmdID byte) {
	dev.writeStatus(cmdID, statusSuccess)
}

func (dev *Device) readLoop() {
	for {
		report := make([]byte, maxReportSize)
		size, err := dev.hidIn.Read(report)
		if err != nil {
			if !dev.closing {
				dev.log.WithError(err).Warn("snapi: HID read failed")
				dev.closeChan <- true
			}
			return
		}

		// the device sends empty reports sometimes, ignore them
		if size < 1 {
			dev.log.Debug("snapi: received empty HID report")
			continue
		} else {
			dev.log.WithFields(log.Fields{
				"data": report[:size],
			}).Debug("snapi: received HID report")
		}

		cmdID := report[0]

		switch cmdID {
		case inMsgStatus:
			dev.ackInChan <- ackMsg{
				cmdID:  report[1],
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
			dev.writeAck(cmdID)

		case inMsgScanLarge:
			dev.handleScan(scanPacket{
				packetCount: binary.BigEndian.Uint16(report[1:]),
				packetIndex: binary.BigEndian.Uint16(report[3:]),
				codeType:    binary.LittleEndian.Uint16(report[6:]),
				data:        report[8 : 8+report[5]],
			})
			dev.writeAck(cmdID)

		default:
			dev.log.WithFields(log.Fields{
				"commandId": cmdID,
			}).Warn("snapi: received unrecognized report")
		}
	}
}

func (dev *Device) writeLoop() {
	for {
		var msg []byte
		select {
		case <-dev.closeChan:
			dev.closeInternal()
			return

		case msg = <-dev.ackOutChan:
		}

		if log.IsLevelEnabled(log.DebugLevel) {
			dev.log.WithFields(log.Fields{
				"length": len(msg),
				"data":   msg,
			}).Debug("snapi: sending command")
		}

		count, err := dev.usbDev.Control(
			0x21,                                // bmRequestType
			0x09,                                // bRequest = SET_REPORT
			0x0200,                              // wValue = Output Report
			uint16(dev.hidIface.Setting.Number), // wIndex
			msg,                                 // Data
		)
		if err != nil {
			dev.log.WithError(err).Error("snapi: HID write failed")
		} else if count != len(msg) {
			dev.log.WithFields(log.Fields{
				"expectLength": len(msg),
				"actualLength": count,
			}).Error("snapi: HID write length mismatch")
		}
	}
}

func (dev *Device) closeInternal() {
	dev.log.Debug("snapi: closing device")
	dev.closing = true
	dev.hidIface.Close()

	err := dev.usbConfig.Close()
	if err != nil {
		dev.log.WithError(err).Warn("snapi: closing USB config failed")
	}

	dev.eventChan <- DeviceClosedEvent{dev}
}

// Close requests that the device resources be released.
// The request is queued and will not necessary be handled immediately.
// A DeviceClosedEvent will be sent to the event channel when the device
// has been closed.
func (dev *Device) Close() {
	dev.closeChan <- true
}
