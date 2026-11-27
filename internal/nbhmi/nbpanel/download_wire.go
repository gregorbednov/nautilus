package nbpanel

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"time"
)

const (
	DownloadPort    = 21845
	nbVersion       = 0x03
	nbWriteProgress = 0x1f
	nbFrameOverhead = 6
	nbChunkSize     = 4096
)

var big = binary.BigEndian

// PanelVersion describes the response to the NB Ethernet version query.
type PanelVersion struct {
	ModelID         uint16 `json:"model_id"`
	Mode            byte   `json:"mode"`
	HardwareVersion uint16 `json:"hardware_version"`
	GUIVersion      uint16 `json:"gui_version"`
	ComVersion      uint16 `json:"comserver_version"`
	ServoVersion    uint16 `json:"servo_version"`
	KernelVersion   uint32 `json:"kernel_version"`
	RootFSVersion   uint32 `json:"rootfs_version"`
}

// ProbeVersion queries a single panel over IPv4 without opening a download session.
func ProbeVersion(ctx context.Context, endpoint string) (PanelVersion, error) {
	client, err := dialNBDownload(ctx, endpoint)
	if err != nil {
		return PanelVersion{}, err
	}
	defer client.close()
	return client.version()
}

type nbDownloadConn struct {
	net.Conn
	ctx  context.Context
	stop func() bool
	buf  [nbChunkSize + 256]byte
}

func dialNBDownload(ctx context.Context, endpoint string) (*nbDownloadConn, error) {
	address, err := netip.ParseAddrPort(endpoint)
	if err != nil {
		ip, ipErr := netip.ParseAddr(endpoint)
		if ipErr != nil {
			return nil, fmt.Errorf("expected an IPv4 HMI address with an optional UDP port")
		}
		address = netip.AddrPortFrom(ip, DownloadPort)
	}
	ip := address.Addr()
	if !ip.Is4() || (!ip.IsGlobalUnicast() && !ip.IsLoopback()) || address.Port() == 0 {
		return nil, fmt.Errorf("expected a unicast IPv4 HMI address and a nonzero port")
	}
	var dialer net.Dialer
	conn, err := dialer.DialContext(ctx, "udp4", net.JoinHostPort(ip.String(), strconv.Itoa(int(address.Port()))))
	if err != nil {
		return nil, err
	}
	stop := context.AfterFunc(ctx, func() { _ = conn.Close() })
	return &nbDownloadConn{Conn: conn, ctx: ctx, stop: stop}, nil
}

func (client *nbDownloadConn) close() {
	client.stop()
	_ = client.Close()
}

func encodeNBFrame(command byte, body []byte) []byte {
	frame := make([]byte, nbFrameOverhead+len(body))
	frame[0], frame[1] = 0x5a, 0xa5
	big.PutUint16(frame[2:], uint16(len(frame)))
	frame[4] = command
	copy(frame[5:], body)
	frame[len(frame)-1] = byte(nbSum(frame))
	return frame
}

func decodeNBFrame(frame []byte) (command byte, body []byte, ok bool) {
	if len(frame) < nbFrameOverhead || frame[0] != 0x5a || frame[1] != 0xa5 {
		return 0, nil, false
	}
	size := int(big.Uint16(frame[2:]))
	if size < nbFrameOverhead || size > len(frame) || byte(nbSum(frame[:size-1])) != frame[size-1] {
		return 0, nil, false
	}
	return frame[4], frame[5 : size-1], true
}

func nbSum(data []byte) uint32 {
	var sum uint32
	for _, value := range data {
		sum += uint32(value)
	}
	return sum
}

// receive ignores delayed replies from earlier requests and invalid checksums.
// A negative sequence means the command has no sequence field in its reply.
func (client *nbDownloadConn) receive(command byte, sequence int, deadline time.Time, progress bool) (byte, []byte, error) {
	if err := client.SetReadDeadline(deadline); err != nil {
		return 0, nil, err
	}
	for {
		n, err := client.Read(client.buf[:])
		if err != nil {
			if client.ctx.Err() != nil {
				return 0, nil, client.ctx.Err()
			}
			return 0, nil, err
		}
		reply, body, valid := decodeNBFrame(client.buf[:n])
		if !valid {
			continue
		}
		if progress && reply == nbWriteProgress && len(body) >= 4 {
			return reply, body, nil
		}
		if reply == command && (sequence < 0 || len(body) >= 2 && int(big.Uint16(body)) == sequence) {
			return reply, body, nil
		}
	}
}

func (client *nbDownloadConn) exchange(frame []byte, sequence, attempts int, timeout time.Duration) ([]byte, error) {
	for attempt := 1; ; attempt++ {
		deadline := time.Now().Add(timeout)
		if err := client.send(frame, deadline); err != nil {
			return nil, err
		}
		_, body, err := client.receive(frame[4], sequence, deadline, false)
		var timeoutError net.Error
		if err == nil || attempt == attempts || !errors.As(err, &timeoutError) || !timeoutError.Timeout() {
			return body, err
		}
	}
}

func (client *nbDownloadConn) send(frame []byte, deadline time.Time) error {
	if err := client.SetWriteDeadline(deadline); err != nil {
		return err
	}
	_, err := client.Write(frame)
	return err
}

func (client *nbDownloadConn) version() (PanelVersion, error) {
	body, err := client.exchange(encodeNBFrame(nbVersion, nil), -1, 3, time.Second)
	if err != nil {
		return PanelVersion{}, fmt.Errorf("query panel version: %w", err)
	}
	if err := nbStatus(body, 0, 1); err != nil {
		return PanelVersion{}, fmt.Errorf("query panel version: %w", err)
	}
	if len(body) < 22 {
		return PanelVersion{}, fmt.Errorf("incomplete panel version reply")
	}
	return PanelVersion{
		Mode: body[1], ModelID: little.Uint16(body[2:]),
		HardwareVersion: little.Uint16(body[4:]),
		GUIVersion:      little.Uint16(body[6:]), ComVersion: little.Uint16(body[8:]),
		ServoVersion: little.Uint16(body[10:]), KernelVersion: little.Uint32(body[14:]),
		RootFSVersion: little.Uint32(body[18:]),
	}, nil
}

func nbStatus(body []byte, offset int, success byte) error {
	if len(body) <= offset {
		return fmt.Errorf("missing reply status")
	}
	if body[offset] != success {
		return fmt.Errorf("panel returned status 0x%02x", body[offset])
	}
	return nil
}
