package main

import (
	//"github.com/elemecca/go-zebra-scanner/snapi"
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type Server struct {
	ListenAddress string
	broadcastChan chan interface{}
	subscribeChan chan chan interface{}
}

func (self *Server) socketEventLoop(conn *websocket.Conn) {
	eventChan := make(chan interface{})
	self.subscribeChan <- eventChan

	log.WithFields(log.Fields{
		"remoteAddr": conn.RemoteAddr(),
	}).Debug("start WS write loop")

	for {
		event := <-eventChan

		log.WithFields(log.Fields{
			"event":      event,
			"remoteAddr": conn.RemoteAddr(),
		}).Debug("WS write loop received event")

		conn.WriteJSON(event)
	}
}

func (self *Server) Serve() {
	self.broadcastChan = make(chan interface{})
	self.subscribeChan = make(chan chan interface{})

	upgrader := websocket.Upgrader{}

	route := http.ServeMux{}

	route.HandleFunc("/scan", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.WithFields(log.Fields{
				"remoteAddr": conn.RemoteAddr(),
			}).Error("failed to upgrade to WS: ", err)
			return
		}

		log.WithFields(log.Fields{
			"remoteAddr": conn.RemoteAddr(),
		}).Info("accepted WS connection")

		go self.socketEventLoop(conn)
	})

	server := http.Server{
		Addr:    self.ListenAddress,
		Handler: &route,
	}

	go self.broadcastLoop()
	server.ListenAndServe()
}

func (self *Server) broadcastLoop() {
	subs := make(map[chan interface{}]bool)
	for {
		log.Debug("enter broadcastLoop")
		select {
		case sub := <-self.subscribeChan:
			log.Debug("register subscriber ", sub)
			subs[sub] = true

		case msg := <-self.broadcastChan:
			for sub, _ := range subs {
				log.Debug("broadcast subscriber ", sub)
				sub <- msg
			}
		}
	}
}

func (self *Server) Broadcast(msg interface{}) {
	self.broadcastChan <- msg
}
