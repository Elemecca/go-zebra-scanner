package snapi

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
	dev.log.Debug(
		"snapi: received scan packet",
		"packetCount", packet.packetCount,
		"packetIndex", packet.packetIndex,
		"codeType", packet.codeType,
		"data", packet.data,
	)

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
		dev.log.Warn(
			"snapi: received unexpected scan packet, resetting scan",
			"expectPacketCount", dev.scan.packetCount,
			"expectPacketIndex", dev.scan.packetIndex,
			"actualPacketCount", packet.packetCount,
			"actualPacketIndex", packet.packetIndex,
			"expectCodeType", dev.scan.codeType,
			"actualCodeType", packet.codeType,
		)
		dev.clearScan()
		return
	}

	dev.scan.packetIndex++
	dev.scan.data = append(dev.scan.data, packet.data...)

	if dev.scan.packetIndex >= dev.scan.packetCount {
		dev.log.Debug(
			"snapi: scan complete",
			"codeType", dev.scan.codeType,
			"data", dev.scan.data,
		)

		codeType, hit := CodeTypeTable[dev.scan.codeType]
		if !hit {
			codeType = CodeType{"unknown", ""}
			dev.log.Warn(
				"snapi: received unknown codeType",
				"codeType", dev.scan.codeType,
			)
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
