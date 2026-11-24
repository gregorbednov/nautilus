package fins

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"testing"
)

func rangeReply(req []byte, code uint16) []byte {
	b := reply(req, nil)
	binary.BigEndian.PutUint16(b[12:14], code)
	return b
}

func TestProgramReadShrinksAtRangeBoundary(t *testing.T) {
	for _, size := range []int{4, 196, 1200, 2048, 2880, 12288, 12380} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			want := bytes.Repeat([]byte{1, 2, 3, 4}, size/4)
			offset, calls, rejections := 0, 0, 0
			client := New(exchangeFunc(func(req []byte) ([]byte, error) {
				calls++
				if calls > 64 {
					t.Fatal("unbounded range probing")
				}
				if len(req) != 20 || binary.BigEndian.Uint16(req[10:12]) != 0x0306 || binary.BigEndian.Uint16(req[12:14]) != 0xffff {
					t.Fatal("unexpected command")
				}
				start := int(binary.BigEndian.Uint32(req[14:18]))
				n := int(binary.BigEndian.Uint16(req[18:20]))
				if start != offset || start%4 != 0 || n < 4 || n%4 != 0 || n > 992 {
					t.Fatal("invalid read range", start, n, offset)
				}
				if start+n > len(want) {
					rejections++
					return rangeReply(req, 0x1104), nil
				}
				body := bytes.Clone(req[12:])
				flags := uint16(n)
				offset += n
				if offset == len(want) {
					flags |= 0x8000
				}
				binary.BigEndian.PutUint16(body[6:8], flags)
				return reply(req, append(body, want[start:offset]...)), nil
			}), Route{})
			got, err := client.ReadProgram(context.Background(), size+4096)
			if err != nil || !bytes.Equal(got, want) || rejections == 0 || offset != size {
				t.Fatal("boundary backup failed", len(got), err, calls, rejections)
			}
		})
	}
}

func TestProgramRangeErrorWithoutFinalFlagIsNotEOF(t *testing.T) {
	calls := 0
	client := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		start := binary.BigEndian.Uint32(req[14:18])
		n := binary.BigEndian.Uint16(req[18:20])
		if start == 0 && n == 4 {
			return reply(req, append(bytes.Clone(req[12:]), 1, 2, 3, 4)), nil
		}
		return rangeReply(req, 0x1104), nil
	}), Route{})
	got, err := client.ReadProgram(context.Background(), 1024)
	var end *EndError
	if got != nil || !errors.As(err, &end) || end.Code != 0x1104 || calls > 10 {
		t.Fatal("range error promoted to complete backup", got, err, calls)
	}
}

func TestProgramReadDoesNotRetryOtherFailures(t *testing.T) {
	for _, code := range []uint16{0x1103, 0x2102, 0x2201} {
		calls := 0
		client := New(exchangeFunc(func(req []byte) ([]byte, error) { calls++; return rangeReply(req, code), nil }), Route{})
		got, err := client.ReadProgram(context.Background(), 4096)
		var end *EndError
		if got != nil || !errors.As(err, &end) || end.Code != code || calls != 1 {
			t.Fatal("unexpected retry", code, calls, err)
		}
	}
	calls := 0
	client := New(exchangeFunc(func([]byte) ([]byte, error) { calls++; return nil, context.DeadlineExceeded }), Route{})
	if got, err := client.ReadProgram(context.Background(), 4096); got != nil || !errors.Is(err, context.DeadlineExceeded) || calls != 1 {
		t.Fatal("retried timeout", err, calls)
	}
}
