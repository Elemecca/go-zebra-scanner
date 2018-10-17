package main

import (
	"encoding/hex"
	"github.com/elemecca/go-zebra-scanner/snapi"
	"log"
)

func main() {
	devs := snapi.Enumerate()
	if len(devs) < 1 {
		log.Println("main: error: no devices found")
		return
	}

	dev, err := devs[0].Open()
	if err != nil {
		log.Println("main: error: device open failed:", err.Error())
		return
	}

	log.Println("main: info: device opened, running")
	for {
		event := <- dev.EventChan
		switch event := event.(type) {
		case snapi.ScanEvent:
			log.Printf(
				"main: scan complete: code type %04x, length %d\n%s",
				event.CodeType, len(event.Data), hex.Dump(event.Data),
			)
		}
	}
}
