package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/godbus/dbus"
	"github.com/potch8228/gobt"
	"github.com/potch8228/gobt/bluetooth"
	"github.com/satori/go.uuid"
)

func main() {
	connIntr, err := bluetooth.Listen(bluetooth.PSMINTR, 1, false)
	if err != nil {
		log.Println("Listen failed: ", err, bluetooth.PSMINTR)
		return
	}

	hidp := gobt.NewHidProfile("/red/potch/profile", connIntr)

	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatalln("Failed to connect to system bus: ", err)
	}

	if err := conn.Export(hidp, hidp.Path(), "org.bluez.Profile1"); err != nil {
		log.Fatalln(err)
	}
	log.Println("org.bluez.Profile1 exported")

	s, err := os.Open("./sdp_record.xml")
	if err != nil {
		log.Fatalln(err)
	}
	sdp, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatalln(err)
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
	log.Println(regObjCall)
	var r interface{}
	if regObjCall.Err != nil {
		log.Println(regObjCall.Store(&r))
		log.Println(r)
		log.Fatalln(regObjCall.Err)
	}
	log.Println("HID Profile registered")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	evloop := true
	for evloop {
		select {
		case dObjCall := <-dObjCh:
			if dObjCall.Err != nil {
				log.Println(dObjCall.Err)
				evloop = false
			}
		case <-sig:
			log.Println("Will Quit Program")
			evloop = false
		default:
		}
	}

	// Probably no need of closing profile
	log.Println("Trying to Close Profile")
	unregObjCall := dObj.Call("org.bluez.ProfileManager1.UnregisterProfile", 0, hidp.Path())
	log.Println(unregObjCall)
	if unregObjCall.Err != nil {
		log.Println(unregObjCall.Store(&r))
		log.Println(r)
		log.Println(regObjCall.Err)
	}
	log.Println("HID Profile unregistered")

	log.Println("Trying to Destroy Profile Obj")
	hidp.Close()

	close(dObjCh)
	conn.Close()
}
