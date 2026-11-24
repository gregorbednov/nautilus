//go:build linux

package fins

import (
	"context"
	"fmt"
	"io"
	"os"
	"syscall"
	"testing"
	"time"
	"unsafe"
)

func TestHostLinkThroughLinuxTTY(t *testing.T) {
	fd, err := syscall.Open("/dev/ptmx", syscall.O_RDWR|syscall.O_NOCTTY|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if os.IsNotExist(err) {
		t.Skip("Linux PTY device unavailable")
	}
	if err != nil {
		t.Fatal(err)
	}
	master := os.NewFile(uintptr(fd), "ptmx")
	defer master.Close()
	var unlock int32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x40045431, uintptr(unsafe.Pointer(&unlock))); e != 0 {
		t.Fatal(e)
	}
	var number uint32
	if _, _, e := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), 0x80045430, uintptr(unsafe.Pointer(&number))); e != 0 {
		t.Fatal(e)
	}
	slave, err := OpenSerial(fmt.Sprintf("/dev/pts/%d", number), SerialConfig{Baud: 9600, DataBits: 8, Parity: "none", StopBits: 1})
	if err != nil {
		t.Fatal(err)
	}
	c, err := NewHostLink(slave, 0, 0, time.Second)
	if err != nil {
		slave.Close()
		t.Fatal(err)
	}
	defer c.Close()
	_ = master.SetDeadline(time.Now().Add(2 * time.Second))
	done := make(chan error, 1)
	go func() {
		var req []byte
		one := []byte{0}
		for {
			if _, err := io.ReadFull(master, one); err != nil {
				done <- err
				return
			}
			req = append(req, one[0])
			if one[0] == '\r' {
				break
			}
			if len(req) > 1114 {
				done <- fmt.Errorf("oversized serial frame")
				return
			}
		}
		if string(req) != string(hostFrame("@00FA000000001010182000A000001")) {
			done <- fmt.Errorf("wrong serial command: %q", req)
			return
		}
		_, err := master.Write(hostFrame("@00FA40000001010100001234"))
		done <- err
	}()
	m, _ := ParseMemory("D10")
	b, err := c.ReadMemory(context.Background(), m, 1)
	if err != nil || len(b) != 2 || b[0] != 0x12 || b[1] != 0x34 {
		t.Fatalf("serial response %x %v", b, err)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
