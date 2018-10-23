module github.com/elemecca/go-zebra-scanner/devices

require (
	github.com/elemecca/go-zebra-scanner/snapi v0.0.0
	github.com/google/gousb v0.0.0
	github.com/sirupsen/logrus v1.1.1
)

replace github.com/elemecca/go-zebra-scanner/snapi => ../snapi

replace github.com/google/gousb => ../vendor/gousb
