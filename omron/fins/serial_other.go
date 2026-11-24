//go:build !linux

package fins

import (
	"errors"
	"os"
)

type SerialConfig struct {
	Baud     int
	DataBits int
	Parity   string
	StopBits int
}

func OpenSerial(string, SerialConfig) (*os.File, error) {
	return nil, errors.New("built-in Host Link serial device support currently requires Linux")
}
