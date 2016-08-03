GoBT
====

What's this?
----
This program will convert your locally connected keyboard/mouse input to Bluetooth HID input device.

Requirements
----
- Linux
    - Tested with Raspbian Jessie
    - (Raspberry Pi 2 for transmitter; Raspberry Pi 3 for receiver)
- BlueZ 5.23+
    - DBus
    - Systemd

Required Modules
----
 - Go 1.6.2+
    - [godbus/dbus](https://github.com/godbus/dbus)
    - [gvalkov/golang-evdev](https://github.com/gvalkov/golang-evdev)
    - [satori/go.uuid](https://github.com/atori/go.uuid)

Preparation
----
`--noplugin=input` option for the bluetoothd is required in order to make this program work.

For example:

On Raspbian Jessie:

Target file: `/lib/systemd/system/bluetooth.service`

```
[Unit]
Description=Bluetooth service
Documentation=man:bluetoothd(8)

[Service]
Type=dbus
BusName=org.bluez
########
## Original Entry
# ExecStart=/usr/lib/bluetooth/bluetoothd
########
## Modified Entry
ExecStart=/usr/lib/bluetooth/bluetoothd -C --noplugin=input
########
NotifyAccess=main
#WatchdogSec=10
#Restart=on-failure
CapabilityBoundingSet=CAP_NET_ADMIN CAP_NET_BIND_SERVICE
LimitNPROC=1

[Install]
WantedBy=bluetooth.target
Alias=dbus-org.bluez.service
```

(`-C` is for some 'older' tools; like sdptools)

Pairing
----
Use bluetoothctl to accept connection and authorize pairing.

```
$ sudo bluetoothctl
[bluetooth]# discoverable on
[bluetooth]# pairable on
[bluetooth]# agent on
[bluetooth]# default-agent
```

Usage
----
`sudo ./gobt`

or

`sudo go run`

(need to execute with root/admin privilege)

After running gobt on transmission side, let the receiver to pair.

In order to stop program, send an interrupt signal from remote or secondary shell.

Credits
----
 - [Emulate a Bluetooth keyboard with the Raspberry Pi - Liam Fraser](http://www.linuxuser.co.uk/tutorials/emulate-a-bluetooth-keyboard-with-the-raspberry-pi)
 - [lvht/btk](https://github.com/lvht/btk)
 - [ii/iikeyboard wiki](https://github.com/ii/iikeyboard/wiki)

License
----
MIT License; See LICENSE for detail

Author
----
Tatsuya Kobayashi <pikopiko28@gmail.com>
