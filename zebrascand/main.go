package main

import (
	"encoding/json"
	"github.com/elemecca/go-zebra-scanner/snapi"
	log "github.com/sirupsen/logrus"
	"os"
)

func isAsciiPrintable(s []byte) bool {
	for _, c := range s {
		if c < 32 || c > 126 {
			return false
		}
	}
	return true
}

func main() {
	if os.Getenv("ZSDEBUG") != "" {
		log.SetLevel(log.DebugLevel)
		log.SetFormatter(&debugTextFormatter{&log.TextFormatter{}})
	}

	devs := snapi.Enumerate()
	if len(devs) < 1 {
		log.Error("no devices found")
		return
	}

	dev, err := devs[0].Open()
	if err != nil {
		log.Error("device open failed:", err)
		return
	}

	log.Info("device opened, running")
	server := Server{
		ListenAddress: "localhost:4141",
	}
	go server.Serve()
	for {
		switch event := (<-dev.EventChan).(type) {
		case snapi.ScanEvent:
			msg := map[string]interface{}{
				"event": "scan",
				"type":  event.PrimaryType,
			}

			if isAsciiPrintable(event.PrimaryData) {
				msg["text"] = string(event.PrimaryData)
			} else {
				msg["data"] = event.PrimaryData
			}

			if event.SupplementalType != "" {
				sup := map[string]interface{}{
					"type": event.SupplementalType,
				}

				if isAsciiPrintable(event.SupplementalData) {
					sup["text"] = string(event.SupplementalData)
				} else {
					sup["data"] = event.SupplementalData
				}

				msg["supplemental"] = sup
			}

			code, _ := json.Marshal(msg)
			server.Broadcast(code)
		}
	}
}
