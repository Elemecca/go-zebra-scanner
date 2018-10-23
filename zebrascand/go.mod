module github.com/elemecca/go-zebra-scanner/zebrascand

require (
	github.com/elemecca/go-zebra-scanner/devices v0.0.0
	github.com/elemecca/go-zebra-scanner/snapi v0.0.0
	github.com/gorilla/websocket v1.4.0
	github.com/sirupsen/logrus v1.1.1
)

replace github.com/elemecca/go-zebra-scanner/snapi => ../snapi

replace github.com/elemecca/go-zebra-scanner/devices => ../devices

replace github.com/google/gousb => ../vendor/github.com/google/gousb
