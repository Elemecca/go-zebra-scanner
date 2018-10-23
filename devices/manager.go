package devices

import (
	"fmt"
	"github.com/elemecca/go-zebra-scanner/snapi"
	"github.com/google/gousb"
	log "github.com/sirupsen/logrus"
)

// ManagedDevice wraps an underlying device with common metadata.
type ManagedDevice struct {
	usbDesc *gousb.DeviceDesc
	usbDev  *gousb.Device
	SNAPI   *snapi.Device
}

// DeviceAttachedEvent indicates a device was connected and opened.
type DeviceAttachedEvent struct {
	device *ManagedDevice
}

// DeviceDetachedEvent indicates an open device was disconnected.
type DeviceDetachedEvent struct {
	device *ManagedDevice
}

type deviceArriveEvt struct {
	desc   *gousb.DeviceDesc
	device *gousb.Device
}

type deviceLeaveEvt struct {
	desc *gousb.DeviceDesc
}

// DeviceManager tracks the set of connected, open devices.
// It watches USB hotplug events and opens any new supported device.
type DeviceManager struct {
	usb        *gousb.Context
	deviceChan chan interface{}
	eventChan  chan<- interface{}
	devEvtChan chan interface{}
	devices    map[string]*ManagedDevice
	deviceKeys map[*snapi.Device]string
}

// NewDeviceManager starts up a new DeviceManager.
func NewDeviceManager(eventChan chan<- interface{}) (*DeviceManager, error) {
	dm := &DeviceManager{
		usb:        gousb.NewContext(),
		deviceChan: make(chan interface{}, 10),
		eventChan:  eventChan,
		devEvtChan: make(chan interface{}, 10),
		devices:    make(map[string]*ManagedDevice),
		deviceKeys: make(map[*snapi.Device]string),
	}

	_, err := dm.usb.RegisterHotplug(dm.handleHotplug)
	if err != nil {
		dm.Close()
		return nil, err
	}

	go dm.deviceLoop()
	go dm.eventLoop()

	return dm, nil
}

func (dm *DeviceManager) handleHotplug(evt gousb.HotplugEvent) {
	arrive := evt.Type() == gousb.HotplugEventDeviceArrived
	desc, err := evt.DeviceDesc()
	if err != nil {
		log.WithError(err).Warn("USB hotplug failed to get descriptor")
		return
	}

	log := log.WithFields(log.Fields{
		"vendor":  desc.Vendor,
		"product": desc.Product,
		"bus":     desc.Bus,
		"address": desc.Address,
	})

	if arrive {
		log.Debug("USB device arrived")
	} else {
		log.Debug("USB device left")
	}

	// for now, just ignore non-SNAPI devices
	if desc.Vendor != snapi.UsbVid || desc.Product != snapi.UsbPid {
		return
	}

	if arrive {
		device, err := evt.Open()
		if err != nil {
			log.WithError(err).Warn("failed to open USB device: ", err)
			return
		}

		dm.deviceChan <- deviceArriveEvt{
			desc:   desc,
			device: device,
		}
	} else {
		dm.deviceChan <- deviceLeaveEvt{
			desc: desc,
		}
	}
}

func formatDeviceKey(desc *gousb.DeviceDesc) string {
	return fmt.Sprintf("%03d:%03d", desc.Bus, desc.Address)
}

// deviceLoop manages the device list
func (dm *DeviceManager) deviceLoop() {
	for {
		switch evt := (<-dm.deviceChan).(type) {
		case deviceArriveEvt:
			log := log.WithFields(log.Fields{
				"bus":     evt.desc.Bus,
				"address": evt.desc.Address,
			})

			snapiDev, err := snapi.OpenUsbDevice(evt.device, dm.devEvtChan)
			if err != nil {
				log.Warn("failed to open SNAPI device: ", err)
				continue
			}

			log.Info("SNAPI device connected")

			device := &ManagedDevice{
				usbDesc: evt.desc,
				usbDev:  evt.device,
				SNAPI:   snapiDev,
			}

			key := formatDeviceKey(evt.desc)
			dm.deviceKeys[snapiDev] = key
			dm.devices[key] = device

			dm.eventChan <- DeviceAttachedEvent{device}

		case deviceLeaveEvt:
			key := formatDeviceKey(evt.desc)
			device, present := dm.devices[key]
			if present {
				device.SNAPI.Close()
			}

		case snapi.DeviceClosedEvent:
			key, present := dm.deviceKeys[evt.Device]
			if present {
				device := dm.devices[key]
				log := log.WithFields(log.Fields{
					"bus":     device.usbDesc.Bus,
					"address": device.usbDesc.Address,
				})

				log.Info("SNAPI device disconnected")

				err := device.usbDev.Close()
				if err != nil {
					log.WithError(err).Warn("closing USB device failed")
				}

				delete(dm.devices, key)
				delete(dm.deviceKeys, evt.Device)
				dm.eventChan <- DeviceDetachedEvent{device}
			}
		}
	}
}

// eventLoop filters events from managed devices
func (dm *DeviceManager) eventLoop() {
	for {
		switch evt := (<-dm.devEvtChan).(type) {
		case snapi.DeviceClosedEvent:
			dm.deviceChan <- evt

		default:
			dm.eventChan <- evt
		}
	}
}

// Close shuts down the device manager.
func (dm *DeviceManager) Close() {
	dm.usb.Close()
}
