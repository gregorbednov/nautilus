package nbpanel

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestNBDownloadProjectOverUDP(t *testing.T) {
	data := panelArtworkFixture()
	for i, version := range []uint32{20325, 1182, 30162} {
		little.PutUint32(data[34+16*i:], version)
	}
	projectAt := u32(data, 8)
	driversAt := u32(data, projectAt+112)
	little.PutUint32(data[driversAt+4:], 0x000201c0)
	little.PutUint32(data[projectAt+124:], 0x00640001)
	pkg, err := ParsePackage(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	type packet struct {
		request []byte
		replies [][]byte
	}
	var script []packet
	add := func(cmd byte, body, reply []byte) {
		script = append(script, packet{encodeNBFrame(cmd, body), [][]byte{encodeNBFrame(cmd, reply)}})
	}
	versionQuery, _ := hex.DecodeString("5aa500060308")
	versionReply, _ := hex.DecodeString("5aa5001e0301027a006e00654f9e04d2758000e4060000e6070000010101")
	damagedReply := bytes.Clone(versionReply)
	damagedReply[7] ^= 1
	script = append(script, packet{versionQuery, [][]byte{damagedReply, versionReply}})
	add(0x01, []byte("\x01eView"), []byte{2})
	initialize, _ := hex.DecodeString("5aa50007410047566965770d")
	script = append(script, packet{initialize, [][]byte{encodeNBFrame(0x41, []byte{1})}})
	add(0x05, nil, []byte{1, 1, 0xc0, 0, 2})
	add(0x08, nil, []byte{1, 0, 1, 0, 100})
	add(0x2a, []byte("\x02eView"), []byte{1})
	add(0x1a, []byte("libfreetype.so"), []byte{0, 0, 0, 4})
	add(0x2f, []byte("\x02eView"), []byte{1})

	fontAt := u32(data, u32(data, projectAt+148)+8)
	archiveAt := u32(data, projectAt+140)
	files := []nbResource{
		{"font_1.ttf", data[fontAt : fontAt+4]},
		{"uncompile", data[archiveAt : archiveAt+8]},
		{"HMI0.hmi", data[pkg.hmi:pkg.hmiEnd]},
	}
	for _, file := range files {
		blocks := (len(file.data) + 4095) / 4096
		start := big.AppendUint32([]byte{0, 0}, uint32(len(file.data)))
		start = big.AppendUint16(start, uint16(blocks+2))
		add(0x10, append(start, file.name...), []byte{1})
		for i := range blocks {
			seq := big.AppendUint16(nil, uint16(i+1))
			chunk := file.data[i*4096 : min((i+1)*4096, len(file.data))]
			add(0x11, append(bytes.Clone(seq), chunk...), append(seq, 1))
		}
		var sum uint32
		for _, b := range file.data {
			sum += uint32(b)
		}
		seq := big.AppendUint16(nil, uint16(blocks+1))
		frame := encodeNBFrame(0x12, big.AppendUint32(bytes.Clone(seq), sum))
		script = append(script, packet{frame, [][]byte{
			encodeNBFrame(0x1f, []byte{0, 1, 0, 1}),
			encodeNBFrame(0x12, append(seq, 2)),
		}})
	}
	add(0x22, data[projectAt+28:projectAt+92], []byte{1})
	add(0x0f, nil, []byte{1})

	server, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	deadline, _ := ctx.Deadline()
	if err := server.SetDeadline(deadline); err != nil {
		t.Fatal(err)
	}
	result := make(chan error, 1)
	go func() {
		buffer := make([]byte, 8192)
		for i, step := range script {
			n, from, err := server.ReadFromUDP(buffer)
			if err != nil {
				result <- err
				return
			}
			if !bytes.Equal(buffer[:n], step.request) {
				result <- fmt.Errorf("step %d: got %d bytes beginning %x, want %d bytes beginning %x",
					i+1, n, buffer[:min(n, 16)], len(step.request), step.request[:min(len(step.request), 16)])
				return
			}
			for _, reply := range step.replies {
				if _, err := server.WriteToUDP(reply, from); err != nil {
					result <- err
					return
				}
			}
		}
		result <- nil
	}()

	var transferred []string
	err = pkg.Download(ctx, server.LocalAddr().String(), func(name string, _ int) {
		transferred = append(transferred, name)
	})
	if err != nil {
		_ = server.Close()
	}
	if receiverErr := <-result; receiverErr != nil {
		t.Fatalf("receiver: %v (download: %v)", receiverErr, err)
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(transferred) != len(files) {
		t.Fatalf("transferred %v", transferred)
	}
	for i, file := range files {
		if transferred[i] != file.name {
			t.Fatalf("resource %d: transferred %s, want %s", i+1, transferred[i], file.name)
		}
	}
}
