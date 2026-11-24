package transfer

import (
	"bytes"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestProgramWriteACKSyntheticLayout(t *testing.T) {
	// Synthetic retained bodies, using the independently documented/native
	// layout. These are not claimed to have been sent to or captured from a PLC.
	request, _ := hex.DecodeString("ffff000000648004deadbeef")
	response, _ := hex.DecodeString("03070000ffff000000640004")
	beforeRequest, beforeResponse := bytes.Clone(request), bytes.Clone(response)
	ack, err := ValidateProgramWriteResponse(request, response)
	if err != nil || ack != (ProgramWriteACK{100, 4, 104, true}) {
		t.Fatalf("ACK = %+v, %v", ack, err)
	}
	if !bytes.Equal(request, beforeRequest) || !bytes.Equal(response, beforeResponse) {
		t.Fatal("retained bytes changed")
	}
	request[6] = 0
	ack, err = ValidateProgramWriteResponse(request, response)
	if err != nil || ack.FinalRequested || ack.NextOffsetBytes != 104 {
		t.Fatalf("non-final ACK = %+v, %v", ack, err)
	}
}

func TestProgramWriteACKRejectsUncorrelatedOrPartialProgress(t *testing.T) {
	request, _ := hex.DecodeString("ffff000000648004deadbeef")
	for _, tc := range []struct{ name, body string }{
		{"truncated", "03070000ffff0000006400"},
		{"extra bytes", "03070000ffff00000064000400"},
		{"other command", "03060000ffff000000640004"},
		{"extended command is distinct", "03170000ffff000000640004"},
		{"other program", "030700000000000000640004"},
		{"other offset", "03070000ffff000000680004"},
		{"zero count", "03070000ffff000000640000"},
		{"partial count", "03070000ffff000000640002"},
		{"excess count", "03070000ffff000000640008"},
		{"echoed final bit", "03070000ffff000000648004"},
		{"PLC error", "03071104"},
		{"PLC error with data", "03071104ffff000000640004"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body, _ := hex.DecodeString(tc.body)
			ack, err := ValidateProgramWriteResponse(request, body)
			if !errors.Is(err, ErrProgramWriteResponse) || ack != (ProgramWriteACK{}) {
				t.Fatalf("must acknowledge no progress: %+v, %v", ack, err)
			}
			if tc.name == "PLC error" && !strings.Contains(err.Error(), "1104") {
				t.Fatal("PLC end code lost")
			}
		})
	}
}

func TestProgramWriteACKRejectsInvalidRequests(t *testing.T) {
	response, _ := hex.DecodeString("03070000ffff000000640004")
	for _, body := range []string{
		"", "ffff000000648004", "0000000000648004deadbeef",
		"ffff000000658004deadbeef", "ffff000000648000deadbeef",
		"ffff000000648003deadbe", "ffff000000648008deadbeef",
		"fffffffffffc8004deadbeef",
	} {
		request, _ := hex.DecodeString(body)
		if ack, err := ValidateProgramWriteResponse(request, response); !errors.Is(err, ErrProgramWriteResponse) || ack != (ProgramWriteACK{}) {
			t.Fatalf("request %s: %+v, %v", body, ack, err)
		}
	}
	if _, err := ValidateProgramWriteResponse(make([]byte, 8+MaxProgramWriteBytes+4), response); !errors.Is(err, ErrProgramWriteResponse) {
		t.Fatal("accepted oversized request")
	}
}
