//go:build linux

package fins

import (
	"fmt"
	"os"
	"strings"
	"syscall"
	"unsafe"
)

// SerialConfig is explicit because the installed port may be configured for
// Host Link, Toolbus, or another protocol with different framing settings.
type SerialConfig struct {
	Baud     int
	DataBits int
	Parity   string
	StopBits int
}

func OpenSerial(path string, c SerialConfig) (*os.File, error) {
	speeds := map[int]uint32{1200: syscall.B1200, 2400: syscall.B2400, 4800: syscall.B4800, 9600: syscall.B9600, 19200: syscall.B19200, 38400: syscall.B38400, 57600: syscall.B57600, 115200: syscall.B115200}
	speed, ok := speeds[c.Baud]
	if !ok {
		return nil, fmt.Errorf("unsupported serial baud %d", c.Baud)
	}
	flags := uint32(syscall.CLOCAL|syscall.CREAD) | speed
	switch c.DataBits {
	case 7:
		flags |= syscall.CS7
	case 8:
		flags |= syscall.CS8
	default:
		return nil, fmt.Errorf("serial data bits must be 7 or 8")
	}
	switch strings.ToLower(c.Parity) {
	case "none":
	case "even":
		flags |= syscall.PARENB
	case "odd":
		flags |= syscall.PARENB | syscall.PARODD
	default:
		return nil, fmt.Errorf("serial parity must be none, even or odd")
	}
	switch c.StopBits {
	case 1:
	case 2:
		flags |= syscall.CSTOPB
	default:
		return nil, fmt.Errorf("serial stop bits must be 1 or 2")
	}
	fd, err := syscall.Open(path, syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), path)
	fail := func(e error) (*os.File, error) { _ = file.Close(); return nil, e }
	var term syscall.Termios
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TCGETS, uintptr(unsafe.Pointer(&term))); e != 0 {
		return fail(e)
	}
	term.Iflag, term.Oflag, term.Lflag = 0, 0, 0
	term.Cflag = flags
	term.Ispeed, term.Ospeed = speed, speed
	term.Cc[syscall.VMIN] = 1
	term.Cc[syscall.VTIME] = 0
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), syscall.TCSETS, uintptr(unsafe.Pointer(&term))); e != 0 {
		return fail(e)
	}
	return file, nil
}
