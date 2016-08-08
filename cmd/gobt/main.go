package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"os"
	"os/signal"

	"github.com/godbus/dbus"
	"github.com/potch8228/gobt"
	"github.com/satori/go.uuid"
)

func main() {
	conn, err := dbus.SystemBus()
	if err != nil {
		log.Fatalln("Failed to connect to system bus: ", err)
	}

	dObj := conn.Object("org.bluez", "/org/bluez")

	connIntr, err := gobt.Listen(gobt.PSMINTR, 1, false)
	if err != nil {
		log.Println("Listen failed: ", err, gobt.PSMINTR)
		return
	}

	var r interface{}
	hidp := gobt.NewHidProfile("/red/potch/profile", connIntr)

	if err := conn.Export(hidp, hidp.Path(), "org.bluez.Profile1"); err != nil {
		log.Fatalln(err)
	}

	uid := uuid.NewV4()

	s, err := os.Open("./sdp_record.xml")
	if err != nil {
		log.Fatalln(err)
	}
	sdp, err := ioutil.ReadAll(s)
	if err != nil {
		log.Fatalln(err)
	}

	opts := map[string]dbus.Variant{
		"PSM": dbus.MakeVariant(uint16(gobt.PSMCTRL)),
		"RequireAuthentication": dbus.MakeVariant(true),
		"RequireAuthorization":  dbus.MakeVariant(true),
		"ServiceRecord":         dbus.MakeVariant(bytes.NewBuffer(sdp).String()),
	}
	dObjCh := make(chan *dbus.Call, 1)
	regObjCall := dObj.Go("org.bluez.ProfileManager1.RegisterProfile", 0, dObjCh, hidp.Path(), uid.String(), opts)
	log.Println(regObjCall)
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
