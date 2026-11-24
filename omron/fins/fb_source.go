package fins

import (
	"context"
	"encoding/binary"
	"errors"
)

const (
	fbSourceName     = "SRCPRGRM.FBL"
	fbSourceDisk     = 0xc001
	fbSourceMaxBytes = 0x7fff

	// FBSourceChangeRefused is an explicit refusal to change the file.
	FBSourceChangeRefused = 0x2108
)

// FBSourceCatalog is the complete CPU-internal root listing returned by 2201.
// ArchiveBytes is zero when SRCPRGRM.FBL is absent.
type FBSourceCatalog struct {
	Raw          []byte
	ArchiveBytes uint32
	Files        uint16
}

func (c *Client) ReadFBSourceCatalog(ctx context.Context) (FBSourceCatalog, error) {
	var result FBSourceCatalog
	_, err := c.callValidated(ctx, 0x2201, []byte{0xc0, 0, 0, 0, 0, 0, 0, 0}, func(data []byte) error {
		const header, entry = 28, 22
		if len(data) < header {
			return errors.New("short CPU file catalog")
		}
		total, listed := binary.BigEndian.Uint16(data[24:]), binary.BigEndian.Uint16(data[26:])
		if listed != total || len(data) != header+entry*int(listed) {
			return errors.New("incomplete CPU file catalog")
		}
		for offset := header; offset < len(data); offset += entry {
			record := data[offset : offset+entry]
			if string(record[:12]) != fbSourceName {
				continue
			}
			size := binary.BigEndian.Uint32(record[16:20])
			if size == 0 || result.ArchiveBytes != 0 {
				return errors.New("invalid SRCPRGRM.FBL catalog entry")
			}
			result.ArchiveBytes = size
		}
		result.Raw = data
		result.Files = listed
		return nil
	})
	return result, err
}

// ReadFBSource reads the exact size advertised by the CPU catalog. Each 2202
// reply must agree on total size, offset, block length and the final-block bit.
func (c *Client) ReadFBSource(ctx context.Context, size uint32) ([]byte, error) {
	if c.Binding().Transport != "tcp" || size == 0 || size > fbSourceMaxBytes {
		return nil, errors.New("invalid SRCPRGRM.FBL transport or size")
	}
	request := make([]byte, 22)
	binary.BigEndian.PutUint16(request, fbSourceDisk)
	copy(request[2:14], fbSourceName)
	archive := make([]byte, 0, size)
	for offset := uint32(0); offset < size; {
		count := min(uint32(1024), size-offset)
		binary.BigEndian.PutUint32(request[14:18], offset)
		binary.BigEndian.PutUint16(request[18:20], uint16(count))
		flags := uint16(count)
		if offset+count == size {
			flags |= 0x8000
		}
		reply, err := c.callValidated(ctx, 0x2202, request, func(data []byte) error {
			if len(data) != 10+int(count) || binary.BigEndian.Uint32(data) != size ||
				binary.BigEndian.Uint32(data[4:]) != offset || binary.BigEndian.Uint16(data[8:]) != flags {
				return errors.New("inconsistent SRCPRGRM.FBL read response")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		archive = append(archive, reply[10:]...)
		offset += count
	}
	return archive, nil
}

// FBSourceWritePayload frames the single final block on the CPU-internal
// file service. The address and name are fixed to the FB source archive.
func FBSourceWritePayload(archive []byte) ([]byte, error) {
	if len(archive) == 0 || len(archive) > fbSourceMaxBytes {
		return nil, errors.New("SRCPRGRM.FBL must fit one FINS file block")
	}
	payload := make([]byte, 24+len(archive))
	binary.BigEndian.PutUint16(payload, fbSourceDisk)
	binary.BigEndian.PutUint16(payload[2:], 1)
	copy(payload[4:16], fbSourceName)
	binary.BigEndian.PutUint16(payload[20:], 0x8000|uint16(len(archive)))
	copy(payload[22:], archive)
	return payload, nil
}

// WriteFBSource sends one 2203 request and validates its empty ACK. A correlated
// 2108 refusal leaves this client open for read-only reconciliation by the caller.
func (c *Client) WriteFBSource(ctx context.Context, archive []byte) error {
	if c.Binding().Transport != "tcp" {
		return errors.New("SRCPRGRM.FBL transfer requires TCP")
	}
	payload, err := FBSourceWritePayload(archive)
	if err != nil {
		return err
	}
	_, err = c.callWithValidation(ctx, 0x2203, payload, emptyControlACK, FBSourceChangeRefused)
	return err
}
