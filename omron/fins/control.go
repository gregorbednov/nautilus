package fins

import (
	"context"
	"errors"
	"fmt"
)

// CPUMode is the explicit requested CPU execution mode, not an acknowledgement
// or proof that the mode was reached. Both MONITOR and RUN execute the program.
type CPUMode byte

const (
	CPUModeProgram CPUMode = 0
	CPUModeMonitor CPUMode = 2
	CPUModeRun     CPUMode = 4
)

// AccessRightBusyError retains a correlated 0C01/3001 owner address. It is a
// device report, not authenticated identity or a lease/ownership token.
// errors.As also exposes the underlying *EndError with the original code.
type AccessRightBusyError struct {
	Owner Address
	end   *EndError
}

func (e *AccessRightBusyError) Error() string {
	return fmt.Sprintf("%s; access held by network=%02X node=%02X unit=%02X", e.end, e.Owner.Network, e.Owner.Node, e.Owner.Unit)
}
func (e *AccessRightBusyError) Unwrap() error { return e.end }

func accessRightBusyError(end *EndError, data []byte) error {
	if len(data) != 3 {
		return fmt.Errorf("access owner response requires exactly 3 bytes (got %d): %w", len(data), end)
	}
	return &AccessRightBusyError{Owner: Address{Network: data[0], Node: data[1], Unit: data[2]}, end: end}
}

// All post-exchange errors on these commands are terminal, even PLC refusals.
// The raw Call path follows the same rule; no forced-acquire helper is exposed.
func terminalCommand(command uint16) bool {
	switch command {
	case 0x0307, 0x0401, 0x0402, 0x0c01, 0x0c02, 0x0c03, 0x2203:
		return true
	}
	return false
}

func emptyControlACK(data []byte) error {
	if len(data) != 0 {
		return errors.New("control ACK must contain only command and end code")
	}
	return nil
}

// RequestCPUMode sends exactly one mutating request. PROGRAM uses STOP0402;
// MONITOR/RUN use0401 with an explicit mode. Program number is always FFFF;
// omitted/default MONITOR forms are deliberately not used. W342 5-3-13/14.
// Success means an empty successful ACK only: the caller must read Status and
// verify the requested mode. No access acquisition, backup, automatic status
// polling, restoration of the prior mode, retry or reconnect is performed.
func (c *Client) RequestCPUMode(ctx context.Context, mode CPUMode) error {
	command := uint16(0x0402)
	data := []byte{0xff, 0xff}
	switch mode {
	case CPUModeProgram:
	case CPUModeMonitor, CPUModeRun:
		command = 0x0401
		data = append(data, byte(mode))
	default:
		return fmt.Errorf("unsupported CPU mode %02X", byte(mode))
	}
	_, err := c.callValidated(ctx, command, data, emptyControlACK)
	return err
}

// AcquireProgramAccess sends non-forced0C01 with FFFF (W342 5-3-24).
// Success is an ACK, not a persistent ownership proof: other devices can
// force-acquire or globally release access. Busy3001 preserves the reported
// owner; all post-exchange errors close/fail this client. No0C02 fallback.
// Closing a connection neither sends RELEASE nor proves the right was freed.
func (c *Client) AcquireProgramAccess(ctx context.Context) error {
	_, err := c.callValidated(ctx, 0x0c01, []byte{0xff, 0xff}, emptyControlACK)
	return err
}

// ReleaseProgramAccess explicitly sends0C03 with FFFF (W342 5-3-26). This
// command releases access REGARDLESS OF THE OWNER, including another device.
// It is not conditional lease cleanup and is never called by Close/defer or
// error handling. Caller must establish the intended target and operation.
// Success does not prove previous ownership or that access remains free.
func (c *Client) ReleaseProgramAccess(ctx context.Context) error {
	_, err := c.callValidated(ctx, 0x0c03, []byte{0xff, 0xff}, emptyControlACK)
	return err
}
