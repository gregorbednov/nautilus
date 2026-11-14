package fins

import (
	"context"
	"errors"
	"testing"
)

type payloadTransport struct {
	data          []byte
	calls, closes int
}

func (t *payloadTransport) Exchange(_ context.Context, req []byte) ([]byte, error) {
	t.calls++
	return reply(req, t.data), nil
}
func (t *payloadTransport) Close() error { t.closes++; return nil }

func TestMalformedPayloadPoisonsSession(t *testing.T) {
	status := make([]byte, 26)
	status[1] = 3
	tests := []struct {
		name string
		data []byte
		read func(*Client) error
	}{
		{"identity", nil, func(c *Client) error { _, err := c.Identity(context.Background()); return err }},
		{"status length", nil, func(c *Client) error { _, err := c.Status(context.Background()); return err }},
		{"status mode", status, func(c *Client) error { _, err := c.Status(context.Background()); return err }},
		{"memory length", []byte{1}, func(c *Client) error { _, err := c.ReadMemory(context.Background(), Memory{Area: 0x82}, 1); return err }},
		{"invalid bit", []byte{2}, func(c *Client) error {
			_, err := c.ReadMemory(context.Background(), Memory{Area: 0x31, BitAccess: true}, 1)
			return err
		}},
		{"program echo", make([]byte, 8), func(c *Client) error { _, err := c.ReadProgram(context.Background(), 32); return err }},
		{"program length", []byte{255, 255, 0, 0, 0, 0, 0x80, 4}, func(c *Client) error { _, err := c.ReadProgram(context.Background(), 32); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			transport := &payloadTransport{data: test.data}
			client := New(transport, Route{})
			if err := test.read(client); err == nil {
				t.Fatal("accepted malformed payload")
			}
			if _, err := client.Call(context.Background(), 0x0501, nil); err == nil {
				t.Fatal("reused malformed session")
			}
			if transport.calls != 1 || transport.closes != 1 {
				t.Fatalf("calls=%d closes=%d", transport.calls, transport.closes)
			}
		})
	}
}

func TestInvalidReadRequestDoesNotPoisonSession(t *testing.T) {
	transport := &payloadTransport{data: make([]byte, 40)}
	client := New(transport, Route{})
	if _, err := client.ReadMemory(context.Background(), Memory{Area: 0x82}, 0); err == nil {
		t.Fatal("accepted zero count")
	}
	if _, err := client.Identity(context.Background()); err != nil {
		t.Fatal(err)
	}
	if transport.calls != 1 || transport.closes != 0 {
		t.Fatal("local validation damaged session")
	}
}

func TestPLCEndErrorDoesNotPoisonSession(t *testing.T) {
	calls := 0
	client := New(exchangeFunc(func(req []byte) ([]byte, error) {
		calls++
		if calls == 1 {
			frame := reply(req, nil)
			frame[12], frame[13] = 0x21, 0x02
			return frame, nil
		}
		return reply(req, make([]byte, 40)), nil
	}), Route{})
	_, err := client.Identity(context.Background())
	var end *EndError
	if !errors.As(err, &end) || end.Code != 0x2102 {
		t.Fatalf("lost PLC end code: %v", err)
	}
	if _, err := client.Identity(context.Background()); err != nil {
		t.Fatal("poisoned valid end-code response", err)
	}
	if calls != 2 {
		t.Fatal("unexpected exchanges")
	}
}
