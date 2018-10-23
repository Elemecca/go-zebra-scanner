package snapi

import (
	log "github.com/sirupsen/logrus"
)

// ScanEvent encodes the results of a successful scan.
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

func (dev *Device) clearScan() {
	dev.scan.packetCount = 0
	dev.scan.packetIndex = 0
	dev.scan.codeType = 0
	dev.scan.data = nil
}

func (dev *Device) handleScan(packet scanPacket) {
	if log.IsLevelEnabled(log.DebugLevel) {
		dev.log.WithFields(log.Fields{
			"packetCount": packet.packetCount,
			"packetIndex": packet.packetIndex,
			"codeType":    packet.codeType,
			"data":        packet.data,
		}).Debug("snapi: received scan packet")
	}

	if dev.scan.data == nil {
		dev.log.Debug("snapi: starting new scan")
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
		dev.log.WithFields(log.Fields{
			"expectPacketCount": dev.scan.packetCount,
			"expectPacketIndex": dev.scan.packetIndex,
			"actualPacketCount": packet.packetCount,
			"actualPacketIndex": packet.packetIndex,
			"expectCodeType":    dev.scan.codeType,
			"actualCodeType":    packet.codeType,
		}).Warn("snapi: received unexpected scan packet, resetting scan")
		dev.clearScan()
		return
	}

	dev.scan.packetIndex++
	dev.scan.data = append(dev.scan.data, packet.data...)

	if dev.scan.packetIndex >= dev.scan.packetCount {
		if log.IsLevelEnabled(log.DebugLevel) {
			dev.log.WithFields(log.Fields{
				"codeType": dev.scan.codeType,
				"data":     dev.scan.data,
			}).Debug("snapi: scan complete")
		}

		codeType, hit := CodeTypeTable[dev.scan.codeType]
		if !hit {
			codeType = CodeType{"unknown", ""}
			dev.log.WithFields(log.Fields{
				"codeType": dev.scan.codeType,
			}).Warn("snapi: received unknown codeType")
		}

		primaryData := dev.scan.data
		var supplementalData []byte

		primaryLen, hit := primaryLengthTable[codeType.primary]
		if hit {
			supplementalData = primaryData[primaryLen:]
			primaryData = primaryData[:primaryLen]
		}

		dev.eventChan <- ScanEvent{
			PrimaryType:      codeType.primary,
			PrimaryData:      primaryData,
			SupplementalType: codeType.supplemental,
			SupplementalData: supplementalData,
		}

		dev.clearScan()
	}
}
