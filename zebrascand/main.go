package main

import (
	"github.com/elemecca/go-zebra-scanner/snapi"
	log "github.com/sirupsen/logrus"
	"os"
)

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
		server.Broadcast(<-dev.EventChan)
	}
}
