package fins

import (
	"context"
	"errors"
	"testing"
)

func TestExclusiveOwnershipPreservesSIDAndRetiresOldHandle(t *testing.T) {
	var sid byte
	tr := &writeTransport{exchange: func(req []byte) ([]byte, error) {
		sid++
		if req[9] != sid {
			t.Fatal("SID reset on ownership transfer")
		}
		return reply(req, nil), nil
	}}
	old := New(tr, Route{Source: Address{Node: 1}, Destination: Address{Node: 2}})
	if _, err := old.Call(context.Background(), 0x0501, nil); err != nil {
		t.Fatal(err)
	}
	owned, err := old.TakeExclusive()
	if err != nil {
		t.Fatal(err)
	}
	if !errors.Is(old.Close(), ErrClientTransferred) || tr.closes != 0 {
		t.Fatal("old close affected owner")
	}
	if _, err = old.Call(context.Background(), 0x0501, nil); !errors.Is(err, ErrClientTransferred) {
		t.Fatal(err)
	}
	if _, err = old.TakeExclusive(); !errors.Is(err, ErrClientTransferred) {
		t.Fatal(err)
	}
	if _, err = old.WriteProgramBlock(context.Background(), writePayload(0, 4, true)); !errors.Is(err, ErrClientTransferred) {
		t.Fatal(err)
	}
	if old.ProgramWriteBlockLimit() != 0 || tr.calls != 1 {
		t.Fatal("retired handle performed I/O")
	}
	if _, err = owned.Call(context.Background(), 0x0501, nil); err != nil {
		t.Fatal(err)
	}
	if err = owned.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err = owned.TakeExclusive(); err == nil {
		t.Fatal("closed client transferred")
	}
	if _, err = owned.Call(context.Background(), 0x0501, nil); err == nil || tr.calls != 2 {
		t.Fatal("closed client exchanged")
	}
}
