package transfer

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrProgramWriteResponse = errors.New("invalid program-write response")

// ProgramWriteACK describes one fully acknowledged candidate block, not a
// verified download. FinalRequested reports the request's bit, not PLC state.
type ProgramWriteACK struct {
	OffsetBytes     uint32 `json:"offsetBytes"`
	DataBytes       int    `json:"dataBytes"`
	NextOffsetBytes uint32 `json:"nextOffsetBytes"`
	FinalRequested  bool   `json:"finalRequested"`
}

// ValidateProgramWriteResponse checks a retained 0307 request payload against a
// retained response body. The request excludes the FINS header and command;
// the response excludes the FINS header but includes command and end code.
// It requires an exact, full acknowledgement. It does not correlate routes or
// SIDs, send, retry, qualify image mapping, or establish completion on a PLC.
func ValidateProgramWriteResponse(requestPayload, responseBody []byte) (ProgramWriteACK, error) {
	bad := func(reason string) (ProgramWriteACK, error) {
		return ProgramWriteACK{}, fmt.Errorf("%w: %s", ErrProgramWriteResponse, reason)
	}
	request, err := ValidateProgramWriteRequest(requestPayload)
	if err != nil {
		return ProgramWriteACK{}, fmt.Errorf("%w: %w", ErrProgramWriteResponse, err)
	}
	offset, n := request.OffsetBytes, request.DataBytes
	if len(responseBody) < 4 {
		return bad("response lacks command/end code")
	}
	if binary.BigEndian.Uint16(responseBody) != 0x0307 {
		return bad("response command")
	}
	if code := binary.BigEndian.Uint16(responseBody[2:]); code != 0 {
		return bad(fmt.Sprintf("PLC end code %04x; no progress acknowledged", code))
	}
	if len(responseBody) != 12 {
		return bad("response size")
	}
	if binary.BigEndian.Uint16(responseBody[4:]) != 0xffff {
		return bad("response program number")
	}
	if binary.BigEndian.Uint32(responseBody[6:]) != offset {
		return bad("response offset")
	}
	if binary.BigEndian.Uint16(responseBody[10:]) != uint16(n) {
		return bad("response does not acknowledge the exact requested byte count")
	}
	return ProgramWriteACK{offset, n, offset + uint32(n), request.Final}, nil
}

var ErrProgramWriteRequest = errors.New("invalid program-write request")

// ProgramWriteRequest is a validated request description, never an ACK.
type ProgramWriteRequest struct {
	OffsetBytes uint32
	DataBytes   int
	Final       bool
}

// ValidateProgramWriteRequest checks one full-program 0307 payload only.
// It does not decide ordering, target identity, mode, access or authorization.
func ValidateProgramWriteRequest(requestPayload []byte) (ProgramWriteRequest, error) {
	bad := func(reason string) (ProgramWriteRequest, error) {
		return ProgramWriteRequest{}, fmt.Errorf("%w: %s", ErrProgramWriteRequest, reason)
	}
	if len(requestPayload) < 12 || len(requestPayload) > 8+MaxProgramWriteBytes {
		return bad("request size")
	}
	if binary.BigEndian.Uint16(requestPayload) != 0xffff {
		return bad("request program number")
	}
	offset := binary.BigEndian.Uint32(requestPayload[2:])
	rawCount := binary.BigEndian.Uint16(requestPayload[6:])
	n := int(rawCount & 0x7fff)
	if n == 0 || n%4 != 0 || offset%4 != 0 || n != len(requestPayload)-8 {
		return bad("request length or alignment")
	}
	if uint64(offset)+uint64(n) > uint64(^uint32(0)) {
		return bad("next offset overflows")
	}

	return ProgramWriteRequest{OffsetBytes: offset, DataBytes: n, Final: rawCount&0x8000 != 0}, nil
}
