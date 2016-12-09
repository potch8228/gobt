package main

import (
	"bytes"
	"io/ioutil"
	"os"
	"os/signal"

	"github.com/godbus/dbus"
	"github.com/potch8228/gobt"
	"github.com/potch8228/gobt/bluetooth"
	btlog "github.com/potch8228/gobt/log"
	"github.com/satori/go.uuid"
)

func main() {
	connIntr, err := bluetooth.Listen(bluetooth.PSMINTR, 1, false)
	if err != nil {
		btlog.Fatal("Listen failed", err, bluetooth.PSMINTR)
	}

	hidp := gobt.NewHidProfile("/red/potch/profile", connIntr)

	conn, err := dbus.SystemBus()
	if err != nil {
		btlog.Fatal("Failed to connect to system bus", err)
	}

	if err := conn.Export(hidp, hidp.Path(), "org.bluez.Profile1"); err != nil {
		btlog.Fatal(err)
	}
	btlog.Debug("org.bluez.Profile1 exported")

	s, err := os.Open("./sdp_record.xml")
	if err != nil {
		btlog.Fatal(err)
	}
	sdp, err := ioutil.ReadAll(s)
	if err != nil {
		btlog.Fatal(err)
	}

	opts := map[string]dbus.Variant{
		"PSM": dbus.MakeVariant(uint16(bluetooth.PSMCTRL)),
		"RequireAuthentication": dbus.MakeVariant(true),
		"RequireAuthorization":  dbus.MakeVariant(true),
		"ServiceRecord":         dbus.MakeVariant(bytes.NewBuffer(sdp).String()),
	}
	uid := uuid.NewV4()

	dObjCh := make(chan *dbus.Call, 1)
	dObj := conn.Object("org.bluez", "/org/bluez")
	regObjCall := dObj.Go("org.bluez.ProfileManager1.RegisterProfile", 0, dObjCh, hidp.Path(), uid.String(), opts)
	btlog.Debug(regObjCall)
	var r interface{}
	if regObjCall.Err != nil {
		btlog.Fatal(regObjCall.Store(&r), r, regObjCall.Err)
	}
	btlog.Debug("HID Profile registered")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	evloop := true
	for evloop {
		select {
		case dObjCall := <-dObjCh:
			if dObjCall.Err != nil {
				btlog.Debug(dObjCall.Err)
				evloop = false
			}
		case <-sig:
			btlog.Debug("Will Quit Program")
			evloop = false
		default:
		}
	}

	// Probably no need of closing profile
	btlog.Debug("Trying to Close Profile")
	unregObjCall := dObj.Call("org.bluez.ProfileManager1.UnregisterProfile", 0, hidp.Path())
	btlog.Debug(unregObjCall)
	if unregObjCall.Err != nil {
		btlog.Debug(unregObjCall.Store(&r), r, regObjCall.Err)
	}
	btlog.Debug("HID Profile unregistered", "Trying to Destroy Profile Obj")
	hidp.Close()

	close(dObjCh)
	conn.Close()
}
