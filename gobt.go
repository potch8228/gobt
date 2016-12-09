package gobt

import (
	"path/filepath"
	"time"

	"github.com/potch8228/gobt/bluetooth"
	"github.com/potch8228/gobt/hid"
	btlog "github.com/potch8228/gobt/log"
)

type GoBtPollState byte

const (
	STOP GoBtPollState = iota
)

const (
	HIDPHEADERTRANSMASK = 0xf0

	HIDPTRANSHANDSHAKE   = 0x00
	HIDPTRANSSETPROTOCOL = 0x60
	HIDPTRANSDATA        = 0xa0

	HIDPHSHKSUCCESSFUL = 0x00
	HIDPHSHKERRUNKNOWN = 0x0e
)

type GoBt struct {
	kbds  []*hid.Keyboard
	mses  []*hid.Mouse
	sintr *bluetooth.Bluetooth
	sctrl *bluetooth.Bluetooth

	cctl chan GoBtPollState
}

func NewGoBt(sintr, sctrl *bluetooth.Bluetooth) *GoBt {
	gobt := GoBt{
		sintr: sintr,
		sctrl: sctrl,
		cctl:  make(chan GoBtPollState, 2),
	}

	kbdPs, _ := filepath.Glob("/dev/input/by-path/*event-kbd")
	gobt.registerKeyboardPaths(kbdPs)

	msePs, _ := filepath.Glob("/dev/input/by-path/*event-mouse")
	gobt.registerMousePaths(msePs)

	btlog.Debug("Sending hello on ctrl channel")
	if _, err := gobt.sctrl.Write([]byte{0xa1, 0x13, 0x03}); err != nil {
		btlog.Debug("Failure on Sending Hello on Ctrl 1", err)
		return nil
	}
	if _, err := gobt.sctrl.Write([]byte{0xa1, 0x13, 0x02}); err != nil {
		btlog.Debug("Failure on Sending Hello on Ctrl 2", err)
		return nil
	}
	time.Sleep(1 * time.Second)

	go gobt.startProcessCtrlEvent()
	return &gobt
}

func (gb *GoBt) startProcessCtrlEvent() {
	for {
		select {
		case <-gb.cctl:
			btlog.Debug("Will Quit GoBt Process loop")
			return
		default:
			r := make([]byte, bluetooth.BUFSIZE)
			d, err := gb.sctrl.Read(r)
			if err != nil || d < 1 {
				btlog.Debug("GoBt.procesCtrlEvent: no data received - quitting event loop")
				gb.Close()
				return
			}

			hsk := []byte{HIDPTRANSHANDSHAKE}
			msgTyp := r[0] & HIDPHEADERTRANSMASK

			switch {
			case (msgTyp & HIDPTRANSSETPROTOCOL) != 0:
				btlog.Debug("GoBt.procesCtrlEvent: handshake set protocol")
				hsk[0] |= HIDPHSHKSUCCESSFUL
				if _, err := gb.sctrl.Write(hsk); err != nil {
					btlog.Debug("GoBt.procesCtrlEvent: handshake set protocol: failure on reply")
				}
			case (msgTyp & HIDPTRANSDATA) != 0:
				btlog.Debug("GoBt.procesCtrlEvent: handshake data")
			default:
				btlog.Debug("GoBt.procesCtrlEvent: unknown handshake message")
				hsk[0] |= HIDPHSHKERRUNKNOWN
				gb.sctrl.Write(hsk)
			}
		}
	}
}

func (gb *GoBt) registerKeyboardPaths(ps []string) {
	kbds := make([]*hid.Keyboard, len(ps))
	var err error
	for i, p := range ps {
		kbds[i], err = hid.NewKeyboard(p, gb.sintr)
		if err != nil {
			btlog.Debug("New Keyboard Initialization failed", err, i)
		}
	}
	gb.kbds = kbds
}

func (gb *GoBt) registerMousePaths(ps []string) {
	mses := make([]*hid.Mouse, len(ps))
	var err error
	for i, p := range ps {
		mses[i], err = hid.NewMouse(p, gb.sintr)
		if err != nil {
			btlog.Debug("New Mouse Initialization failed", err, i)
		}
	}
	gb.mses = mses
}

func (gb *GoBt) Close() {
	for _, kbd := range gb.kbds {
		kbd.StopProcess()
	}

	for _, mse := range gb.mses {
		mse.StopProcess()
	}

	btlog.Debug("Stopped HIDevices")

	btlog.Debug("Trying to Stop GoBt evevnt loop")
	gb.cctl <- STOP

	btlog.Debug("Trying to Destory Objects")
	gb.kbds = nil
	gb.mses = nil
	gb.sintr = nil
	gb.sctrl = nil
}
