package nbpanel

import "fmt"

// artworkPool stores groups using HMI-relative pointers. A nil pool keeps
// the bytes inline in the component record.
type artworkPool struct {
	base, limit int
	data        []byte
}

func (pool *artworkPool) offset(inline int) int {
	if pool == nil {
		return inline
	}
	return pool.base + len(pool.data)
}

func (pool *artworkPool) store(record []byte, parts ...[]byte) ([]byte, error) {
	if pool == nil {
		for _, part := range parts {
			record = append(record, part...)
		}
		return record, nil
	}
	remaining := pool.limit - len(pool.data)
	for _, part := range parts {
		if len(part) > remaining {
			return nil, fmt.Errorf("artwork exceeds the panel package size limit")
		}
		remaining -= len(part)
	}
	for _, part := range parts {
		pool.data = append(pool.data, part...)
	}
	return record, nil
}
