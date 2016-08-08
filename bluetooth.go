package gobt

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

type _Socklen uint32
type RawSockaddrL2 struct {
	Family uint16
	Psm    uint16
	Bdaddr [6]uint8
}
type SockaddrL2 struct {
	PSM    uint16
	Bdaddr [6]uint8
	raw    RawSockaddrL2
}

func (sa *SockaddrL2) sockaddr() (unsafe.Pointer, _Socklen, error) {
	sa.raw.Family = unix.AF_BLUETOOTH
	sa.raw.Psm = uint16(sa.PSM)
	sa.raw.Bdaddr = sa.Bdaddr

	return unsafe.Pointer(&sa.raw), _Socklen(unsafe.Sizeof(RawSockaddrL2{})), nil
}

func (sa *SockaddrL2) String() string {
	return fmt.Sprintf("[PSM: %d, Bdaddr: %v]", sa.PSM, sa.Bdaddr)
}

const (
	PSMCTRL = 0x11
	PSMINTR = 0x13
	BUFSIZE = 1024

	FDBITS = 32
)

var mu sync.Mutex

type FdSet struct {
	Bits [32]int32
}

func setFd(fd int, fdset *FdSet) {
	mask := uint(1) << (uint(fd) % uint(FDBITS))
	fdset.Bits[fd/FDBITS] |= int32(mask)
}

func isSetFd(fd int, fdset *FdSet) bool {
	mask := uint(1) << (uint(fd) % uint(FDBITS))
	return ((fdset.Bits[fd/FDBITS] & int32(mask)) != 0)
}

func internalSelect(fd int, r, w, e *FdSet, to time.Duration) (int, error) {
	t := unix.NsecToTimeval(to.Nanoseconds())
	rFd := uintptr(unsafe.Pointer(r))
	wFd := uintptr(unsafe.Pointer(w))
	eFd := uintptr(unsafe.Pointer(e))

	// n, err := unix.Select(fd, rFd, wFd, eFd, &t)
	n, _, err := unix.Syscall6(unix.SYS_SELECT, uintptr(fd), rFd, wFd, eFd, uintptr(unsafe.Pointer(&t)), 0)
	if err != 0 {
		log.Println("Select Error: ", err)
		return -1, err
	}
	return int(n), nil
}

type Bluetooth struct {
	fd     int
	family int
	proto  int
	typ    int
	saddr  SockaddrL2

	block bool
	mu    sync.Mutex
}

func (bt *Bluetooth) SetBlocking(block bool) error {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	dFlgPtr, _, err := unix.Syscall(unix.SYS_FCNTL, uintptr(bt.fd), unix.F_GETFL, 0)

	if err != 0 {
		log.Println("SetBlocking fetch state: ", err)
		return err
	}

	var delayFlag uint
	if block {
		delayFlag = uint(dFlgPtr) & (^(uint)(unix.O_NONBLOCK))
	} else {
		delayFlag = uint(dFlgPtr) | ((uint)(unix.O_NONBLOCK))
	}

	_, _, err = unix.Syscall(unix.SYS_FCNTL, uintptr(bt.fd), unix.F_SETFL, uintptr(delayFlag))
	if err != 0 {
		log.Println("SetBlocking set state: ", err)
		return err
	}

	return nil
}

func NewBluetoothSocket(fd int) (*Bluetooth, error) {
	bt := &Bluetooth{
		fd:     fd,
		family: unix.AF_BLUETOOTH,
		typ:    unix.SOCK_SEQPACKET,
		proto:  unix.BTPROTO_L2CAP,
		block:  false,
	}

	var rsa RawSockaddrL2
	var addrlen _Socklen = _Socklen(unsafe.Sizeof(RawSockaddrL2{}))
	_, _, err := unix.RawSyscall(unix.SYS_GETSOCKNAME, uintptr(fd), uintptr(unsafe.Pointer(&rsa)), uintptr(unsafe.Pointer(&addrlen)))
	if int(err) != 0 {
		log.Println("Failure on getsockname: ", err)
		unix.Close(fd)
		return nil, err
	}

	bt.saddr = SockaddrL2{
		PSM:    rsa.Psm,
		Bdaddr: rsa.Bdaddr,
	}

	log.Println("Resolved sockname: ", bt.saddr)
	log.Println("New Socket is created")

	return bt, nil
}

func Listen(psm uint, bklen int, block bool) (*Bluetooth, error) {
	mu.Lock()
	defer mu.Unlock()
	bt := &Bluetooth{
		family: unix.AF_BLUETOOTH,
		typ:    unix.SOCK_SEQPACKET, // RFCOMM = SOCK_STREAM, L2CAP = SOCK_SEQPACKET, HCI = SOCK_RAW
		proto:  unix.BTPROTO_L2CAP,
		block:  block,
	}

	fd, err := unix.Socket(bt.family, bt.typ, bt.proto)
	if err != nil {
		log.Println(err)
		return nil, err
	}
	log.Println("Socket is created")

	bt.fd = fd
	unix.CloseOnExec(bt.fd)

	if err := bt.SetBlocking(block); err != nil {
		_err := bt.Close()
		log.Println(_err)
		log.Println("SetBlocking: ", err)
		return nil, err
	}
	log.Println("Socket is set blocking mode")

	// because L2CAP socket address struct does not exist in golang's standard libs
	// must be binded by using very low-level operations
	addr := SockaddrL2{
		PSM:    uint16(psm),
		Bdaddr: [6]uint8{0},
	}
	bt.saddr = addr
	saddr, saddrlen, err := addr.sockaddr()

	if _, _, err := unix.Syscall(unix.SYS_BIND, uintptr(bt.fd), uintptr(saddr), uintptr(saddrlen)); int(err) != 0 {
		switch int(err) {
		case 0:
		default:
			_err := bt.Close()
			log.Println(_err)
			log.Println(err)
			log.Println("Failure on Binding Socket")
			return nil, err
		}
	}
	log.Println("Socket is binded")

	if err := unix.Listen(bt.fd, bklen); err != nil {
		_err := bt.Close()
		log.Println(_err)
		return nil, err
	}
	log.Println("Socket is listening")

	return bt, nil
}

func (bt *Bluetooth) Accept() (*Bluetooth, error) {
	mu.Lock()
	defer mu.Unlock()

	var nFd int
	var rAddr *SockaddrL2

	t := 5 * time.Second
	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)
	for {
		if !bt.block {
			if to, err := internalSelect(bt.fd+1, fds, nil, nil, t); err != nil && to < 1 {
				log.Println("Select Syscall Failure: ", err)
				return nil, err
			}
		}

		var raddr RawSockaddrL2
		var addrlen _Socklen = _Socklen(unsafe.Sizeof(RawSockaddrL2{}))
		rFd, _, err := unix.Syscall(unix.SYS_ACCEPT, uintptr(bt.fd), uintptr(unsafe.Pointer(&raddr)), uintptr(unsafe.Pointer(&addrlen)))
		if err != 0 {
			switch err {
			case syscall.EAGAIN:
				time.Sleep(10 * time.Millisecond)
				continue
			case syscall.ECONNABORTED:
				continue
			}
			log.Println("Accept: Socket err: ", err)
			unix.Close(int(rFd))
			return nil, err
		}

		nFd = int(rFd)
		rAddr = &SockaddrL2{
			PSM:    raddr.Psm,
			Bdaddr: raddr.Bdaddr,
		}
		break
	}

	log.Println("Remote Address Info: ", rAddr)

	rbt := &Bluetooth{
		family: bt.family,
		typ:    bt.typ,
		proto:  bt.proto,
		block:  bt.block,
		fd:     nFd,
		saddr:  *rAddr,
	}

	unix.CloseOnExec(nFd)
	log.Println("Accept closeonexec")

	if err := rbt.SetBlocking(false); err != nil {
		_err := bt.Close()
		_err = rbt.Close()
		log.Println(_err)
		log.Println("SetBlocking: ", err)
		return nil, err
	}
	log.Println("Accepted Socket is set blocking mode")

	return rbt, nil
}

func (bt *Bluetooth) Read(b []byte) (int, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	var bp unsafe.Pointer
	var _zero uintptr
	if len(b) > 0 {
		bp = unsafe.Pointer(&b[0])
	} else {
		bp = unsafe.Pointer(&_zero)
	}
	t := 5 * time.Second
	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)

	var r int
	for {
		if !bt.block {
			if to, err := internalSelect(bt.fd+1, fds, nil, nil, t); err != nil && to < 1 {
				log.Println("Select Syscall Failure: ", err)
				return -1, err
			}
		}

		_r, _, err := unix.Syscall(unix.SYS_READ, uintptr(bt.fd), uintptr(bp), uintptr(len(b)))

		if err != 0 {
			switch err {
			case syscall.EAGAIN:
				time.Sleep(10 * time.Millisecond)
				continue
			}
			log.Println("Bluetooth Read Error: ", err)
			return -1, err
		}

		r = int(_r)
		break
	}

	return r, nil
}

func (bt *Bluetooth) Write(d []byte) (int, error) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	var dp unsafe.Pointer
	var _zero uintptr
	if len(d) > 0 {
		dp = unsafe.Pointer(&d[0])
	} else {
		dp = unsafe.Pointer(&_zero)
	}
	t := 5 * time.Second
	fds := &FdSet{Bits: [32]int32{0}}
	setFd(bt.fd, fds)

	var r int
	for {
		if bt.block {
			if to, err := internalSelect(bt.fd+1, nil, fds, nil, t); err != nil && to < 1 {
				log.Println("Select Syscall Failure: ", err)
				return -1, err
			}
		}

		_r, _, err := unix.Syscall(unix.SYS_WRITE, uintptr(bt.fd), uintptr(dp), uintptr(len(d)))

		if err != 0 {
			log.Println("Bluetooth Write Error: ", err)
			return -1, err
		}

		r = int(_r)
		break
	}

	return r, nil
}

func (bt *Bluetooth) Close() error {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	if bt.fd <= 0 {
		return unix.EINVAL
	}

	if err := unix.Close(bt.fd); err != nil {
		log.Println("Bluetooth Close fd: ", err)
		return err
	}

	return nil
}
