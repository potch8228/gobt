package gobt

import (
	"fmt"
	"log"

	"golang.org/x/sys/unix"

	"github.com/godbus/dbus"
	"github.com/potch8228/gobt/bluetooth"
)

type HidProfile struct {
	path dbus.ObjectPath

	gb map[dbus.ObjectPath]*GoBt

	connIntr *bluetooth.Bluetooth

	sintr *bluetooth.Bluetooth
	sctrl *bluetooth.Bluetooth
}

func NewHidProfile(path string, connIntr *bluetooth.Bluetooth) *HidProfile {
	return &HidProfile{
		path:     (dbus.ObjectPath)(path),
		gb:       make(map[dbus.ObjectPath]*GoBt),
		connIntr: connIntr,
	}
}

func (p *HidProfile) Path() dbus.ObjectPath {
	return p.path
}

func (p *HidProfile) Release() *dbus.Error {
	log.Println("Release")
	return nil
}

func (p *HidProfile) NewConnection(dev dbus.ObjectPath, fd dbus.UnixFD, fdProps map[string]dbus.Variant) *dbus.Error {
	log.Println("NewConnection", dev, fd, fdProps)

	var err error
	p.sintr, err = p.connIntr.Accept()
	if err != nil {
		p.connIntr.Close()
		log.Println("Accept failed: ", err, bluetooth.PSMINTR)
		return dbus.NewError(fmt.Sprintf("Accept failed: %v", bluetooth.PSMINTR), []interface{}{err})
	}
	log.Println("Connection Accepted : ", bluetooth.PSMINTR)

	p.sctrl, err = bluetooth.NewBluetoothSocket(int(fd))
	if err != nil {
		_err := unix.Close(int(fd))

		if _err != nil {
			log.Println("NewBluetoothSocket closing fd failed: ", _err)
			return dbus.NewError("NewBluetoothSocket closing fd failed: ", []interface{}{_err})
		}
		log.Println("NewBluetoothSocket failed: ", err, fd, fdProps)
		return dbus.NewError(fmt.Sprintf("NewBluetoothSocket failed: %v, %v, %v", err, fd, fdProps), []interface{}{err})
	}
	log.Println("Created New Ctrl Socket")

	p.gb[dev] = NewGoBt(p.sintr, p.sctrl)
	return nil
}

func (p *HidProfile) RequestDisconnection(dev dbus.ObjectPath) *dbus.Error {
	log.Println("RequestDisconnection: ", dev)
	p.gb[dev].Close()
	return nil
}

func (p *HidProfile) Close() {
	log.Println("Hid Profile will close")
	for k := range p.gb {
		p.gb[k].Close()
		p.gb[k] = nil
	}

	if p.sintr != nil {
		p.sintr.Close()
	}

	if p.sctrl != nil {
		p.sctrl.Close()
	}
}
