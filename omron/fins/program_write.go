package fins

import (
	"bytes"
	"context"
	"errors"

	"github.com/joyautomation/nautilus/omron/transfer"
)

// ProgramWriteBlockLimit reports a request-capacity bound, not hardware or
// image qualification. Host Link framing needs smaller blocks than Ethernet.
func (c *Client) ProgramWriteBlockLimit() int {
	if c == nil || c.transport == nil || c.transferred.Load() || c.closed.Load() {
		return 0
	}
	limit := 8 + transfer.MaxProgramWriteBytes
	if t, ok := c.transport.(interface{ MaxRequestData() int }); ok {
		limit = min(limit, t.MaxRequestData())
	}
	if limit < 12 {
		return 0
	}
	return (limit - 8) / 4 * 4
}

// WriteProgramBlock sends exactly one explicitly supplied 0307 payload. This
// low-level mutating API is not a full download/recovery procedure or preflight
// approval. The caller must establish target, PROGRAM mode, access, backup and
// authorization. It does not change mode, acquire access, stamp an image or retry.
// ACK validation runs while the client's route/SID exchange lock is held.
// Any exchange/response/end-code error closes and permanently fails the session;
// even a PLC refusal cannot prove that no program bytes were modified.
func (c *Client) WriteProgramBlock(ctx context.Context, payload []byte) (transfer.ProgramWriteACK, error) {
	if c == nil || c.transport == nil {
		return transfer.ProgramWriteACK{}, errors.New("uninitialized program-write client")
	}
	if c.transferred.Load() {
		return transfer.ProgramWriteACK{}, ErrClientTransferred
	}
	if len(payload) < 12 || len(payload) > 8+transfer.MaxProgramWriteBytes {
		_, err := transfer.ValidateProgramWriteRequest(payload)
		return transfer.ProgramWriteACK{}, err
	}
	owned := bytes.Clone(payload)
	request, err := transfer.ValidateProgramWriteRequest(owned)
	if err != nil {
		return transfer.ProgramWriteACK{}, err
	}
	if request.DataBytes > c.ProgramWriteBlockLimit() {
		return transfer.ProgramWriteACK{}, errors.New("program-write block exceeds transport request capacity")
	}
	var ack transfer.ProgramWriteACK
	_, err = c.callValidated(ctx, 0x0307, owned, func(data []byte) error {
		// response() has already correlated route/SID/command and verified end=0.
		// Reconstruct only that verified prefix for the existing strict validator.
		body := make([]byte, 4+len(data))
		body[0], body[1] = 0x03, 0x07
		copy(body[4:], data)
		var validationError error
		ack, validationError = transfer.ValidateProgramWriteResponse(owned, body)
		return validationError
	})
	if err != nil {
		return transfer.ProgramWriteACK{}, err
	}
	return ack, nil
}
