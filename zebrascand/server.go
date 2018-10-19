package main

import (
	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
)

// Server encapsulates the Websocket server component.
type Server struct {
	ListenAddress string
	broadcastChan chan []byte
	subscribeChan chan chan []byte
}

func (s *Server) socketEventLoop(conn *websocket.Conn) {
	eventChan := make(chan []byte)
	s.subscribeChan <- eventChan

	log.WithFields(log.Fields{
		"remoteAddr": conn.RemoteAddr(),
	}).Debug("start WS write loop")

	for {
		event := <-eventChan

		log.WithFields(log.Fields{
			"event":      event,
			"remoteAddr": conn.RemoteAddr(),
		}).Debug("WS write loop received event")

		conn.WriteMessage(websocket.TextMessage, event)
	}
}

// Serve starts up the server and blocks indefinitely to serve requests.
// This should generally be called as a goroutine.
func (s *Server) Serve() {
	s.broadcastChan = make(chan []byte)
	s.subscribeChan = make(chan chan []byte)

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

		go s.socketEventLoop(conn)
	})

	server := http.Server{
		Addr:    s.ListenAddress,
		Handler: &route,
	}

	go s.broadcastLoop()
	server.ListenAndServe()
}

func (s *Server) broadcastLoop() {
	subs := make(map[chan []byte]bool)
	for {
		log.Debug("enter broadcastLoop")
		select {
		case sub := <-s.subscribeChan:
			log.Debug("register subscriber ", sub)
			subs[sub] = true

		case msg := <-s.broadcastChan:
			for sub := range subs {
				log.Debug("broadcast subscriber ", sub)
				sub <- msg
			}
		}
	}
}

// Broadcast sends a message to all connected clients.
func (s *Server) Broadcast(msg []byte) {
	s.broadcastChan <- msg
}
