package main

import (
	"fmt"
	"log"

	"github.com/gvalkov/golang-evdev"
)

type DeviceEventCtrl byte

const (
	DEVEVCBQUIT DeviceEventCtrl = iota
)

type DeviceError struct {
	msg    string
	method string
}

func (de *DeviceError) Error() string {
	return fmt.Sprintf("DeviceError: '%s' by method: %s", de.msg, de.method)
}

/*
Keyboard HID Report structure
[
	0xA1, # This is an input report at first byte field
	0x02, # Usage report(exclusive for bt) = Keyboard
	# Bit array for Modifier keys (D7 being the first element, D0 being last)
	[
		0,   # Right GUI - (usually the Windows key)
		0,   # Right ALT
		0,   # Right Shift
		0,   # Right Control
		0,   # Left GUI - (again, usually the Windows key)
		0,   # Left ALT
		0,   # Left Shift
		0    # Left Control
	],
	0x00, # Vendor reserved
	0x00, # Rest is space for 6 keys
	0x00,
	0x00,
	0x00,
	0x00,
	0x00
]
*/
type Keyboard struct {
	dev   *evdev.InputDevice
	state []byte
	evlp  bool
	ctl   chan DeviceEventCtrl
	intr  chan *evdev.InputEvent
	sintr *Bluetooth
}

func NewKeyboard(path string, sintr *Bluetooth) (*Keyboard, error) {
	k := new(Keyboard)

	k.evlp = false
	k.state = make([]byte, 10)
	for i, _ := range k.state {
		k.state[i] = 0x00
	}
	k.state[0] = 0xA1
	k.state[1] = 0x02

	var err error
	k.dev, err = evdev.Open(path)
	if err != nil {
		log.Println("Failure on Opening Keyboard: ", path)
		return nil, err
	}
	k.sintr = sintr

	k.ctl = make(chan DeviceEventCtrl, 1)
	k.intr = make(chan *evdev.InputEvent, 10)

	k.evlp = true
	go k.startProcess()

	return k, nil
}

func (k *Keyboard) startProcess() {
	go k.pollEvent()

	for k.evlp {
		select {
		case ev := <-k.intr:
			log.Println("Keyboard Event detected", ev)
			if err := k.changeState(ev); err != nil {
				log.Println("Failure on keyboard changeState", err)
				k.StopProcess()
				break
			}
			k.send()
		}
	}

	k.evlp = false
	close(k.ctl)
	close(k.intr)
	log.Println("Stopping Keyboard Event loop")
}

func (k *Keyboard) StopProcess() {
	k.ctl <- DEVEVCBQUIT
	k.evlp = false
}

func (k *Keyboard) pollEvent() {
	for {
		select {
		case <-k.ctl:
			log.Println("Quitting Keyboard Poller")
			return
		default:
			input, err := k.dev.ReadOne()
			if err != nil {
				log.Println("Error on reading keyboard event: ", err)
				k.StopProcess()
				return
			}

			if input.Type == evdev.EV_KEY && evdev.KeyEventState(input.Value) <= evdev.KeyDown {
				k.intr <- input
			}
		}
	}
}

func (k *Keyboard) send() {
	log.Printf("Current Keyboard State: %v", k.state)
	if _, err := k.sintr.Write(k.state); err != nil {
		log.Println("Failure on Sending Keyboard State")
		return
	}

	log.Println("Sending Keyboard State Done")
}

func (k *Keyboard) changeState(ev *evdev.InputEvent) error {
	kev := evdev.NewKeyEvent(ev)
	raw := evdev.KEY[int(kev.Scancode)]

	key, mkey := Convert(raw)
	kev.Keycode = uint16(key)

	var err error = nil
	switch mkey {
	case MOD:
		err = k.updateModifiers(kev)
	case FUNC:
		k.updateStates(kev)
	}

	return err
}

func (k *Keyboard) updateModifiers(kev *evdev.KeyEvent) error {
	if kev.Keycode > 8 { // length of 8 bits
		return &DeviceError{msg: "bitpos(kev.Keycode) > 8", method: "updateModifiers()"}
	}

	switch kev.State {
	case evdev.KeyDown:
		k.state[2] |= byte(1 << kev.Keycode)
	case evdev.KeyUp:
		k.state[2] &= byte(^(1 << kev.Keycode))
	}

	return nil
}

func (k *Keyboard) updateStates(kev *evdev.KeyEvent) {
	for i := 4; i < len(k.state); i++ {
		switch {
		case kev.State == evdev.KeyUp && byte(kev.Keycode) == k.state[i]:
			k.state[i] = 0x00
		case kev.State == evdev.KeyDown && k.state[i] == 0x00:
			k.state[i] = byte(kev.Keycode)
			return
		}
	}
}

/*
Mouse HID Report structure
[
	0xA1, # This is an input report at first byte field
	0x01, # Usage report(exclusive for bt) = Mouse
	# Bit array for Modifier keys (D7 being the first element, D0 being last)
	[
		0,   // Not Used
		0,   // Not Used
		0,   // Not Used
		0,   // Not Used
		0,   // Not Used
		0,   // Button Left
		0,   // Button Middle
		0    // Button Right
	],
	0x00, // X relative
	0x00, // Y relative
	0x00  // Wheel relative
]
*/
type Mouse struct {
	dev   *evdev.InputDevice
	state []byte
	evlp  bool
	ctl   chan DeviceEventCtrl
	intr  chan *evdev.InputEvent
	sintr *Bluetooth
}

func NewMouse(path string, sintr *Bluetooth) (*Mouse, error) {
	m := new(Mouse)

	m.evlp = false
	m.state = make([]byte, 6)
	for i, _ := range m.state {
		m.state[i] = 0x00
	}
	m.state[0] = 0xA1
	m.state[1] = 0x01

	var err error
	m.dev, err = evdev.Open(path)
	if err != nil {
		return nil, err
	}
	m.sintr = sintr

	m.ctl = make(chan DeviceEventCtrl, 1)
	m.intr = make(chan *evdev.InputEvent, 10)

	m.evlp = true
	go m.startProcess()

	return m, nil
}

func (m *Mouse) startProcess() {
	go m.pollEvent()

	for m.evlp {
		select {
		case ev := <-m.intr:
			m.changeState(ev)
			m.send()
		}
	}

	close(m.ctl)
	close(m.intr)
	log.Println("Stopping Mouse Event Loop")
}

func (m *Mouse) StopProcess() {
	m.ctl <- DEVEVCBQUIT
	m.evlp = false
}

func (m *Mouse) pollEvent() {
	for {
		select {
		case <-m.ctl:
			log.Println("Quitting Mouse Poller")
			return
		default:
			input, err := m.dev.ReadOne()
			if err != nil {
				log.Println("Error on reading mouse event: ", err)
				m.StopProcess()
				return
			}

			switch input.Type {
			case evdev.EV_ABS:
				fallthrough
			case evdev.EV_REL:
				fallthrough
			case evdev.EV_KEY:
				m.intr <- input
			}
		}
	}
}

func (m *Mouse) send() {
	log.Printf("Current Mouse State: %v", m.state)
	if _, err := m.sintr.Write(m.state); err != nil {
		log.Println("Failure on Sending Mouse State")
	}
	log.Println("Sending Mouse State Done")
}

func (m *Mouse) changeState(ev *evdev.InputEvent) {
	m.state[3] = 0x00
	m.state[4] = 0x00
	m.state[5] = 0x00

	if ev.Type == evdev.EV_KEY {
		m.updateButton(ev)
		return
	}

	switch ev.Code {
	case evdev.REL_X:
		m.state[3] = byte(downCaseLongToShort(ev.Value))
	case evdev.REL_Y:
		m.state[4] = byte(downCaseLongToShort(ev.Value))
	case evdev.REL_WHEEL:
		m.state[5] = byte(downCaseLongToShort(ev.Value))
	}
}

func (m *Mouse) updateButton(ev *evdev.InputEvent) {
	var st byte
	switch ev.Code {
	case evdev.BTN_LEFT:
		st = byte(0x01)
	case evdev.BTN_MIDDLE:
		st = byte(0x01 << 1)
	case evdev.BTN_RIGHT:
		st = byte(0x01 << 2)
	}

	switch evdev.KeyEventState(ev.Value) {
	case evdev.KeyUp:
		m.state[2] &= ^st
	case evdev.KeyDown:
		m.state[2] |= st
	}
}

const (
	Int8Min  = -128
	Int8Max  = 127
	Int32Min = -2147483648
	Int32Max = 2147483647
)

func downCaseLongToShort(v int32) (r int8) {
	switch {
	case v > Int8Max:
		r = Int8Max
	case v < Int8Min:
		r = Int8Min
	default:
		r = int8(v)
	}
	return
}
