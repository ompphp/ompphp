package gotz

import (
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrBadData = errors.New("gotz: malformed timezone data")

// parseData parses TZif-format binary data into a Zone.
func parseData(name string, data []byte) (*Zone, error) {
	if len(data) < 44 {
		return nil, fmt.Errorf("%w: file too short", ErrBadData)
	}

	// Validate magic number.
	if string(data[:4]) != "TZif" {
		return nil, fmt.Errorf("%w: invalid magic number", ErrBadData)
	}

	// Read version.
	var version int
	switch data[4] {
	case 0:
		version = 1
	case '2':
		version = 2
	case '3':
		version = 3
	case '4':
		version = 4
	default:
		return nil, fmt.Errorf("%w: unknown version byte %#x", ErrBadData, data[4])
	}

	// Parse header counts (6 big-endian uint32 at offset 20).
	d := data[20:]
	if len(d) < 24 {
		return nil, fmt.Errorf("%w: header truncated", ErrBadData)
	}

	isutcnt := int(binary.BigEndian.Uint32(d[0:4]))
	isstdcnt := int(binary.BigEndian.Uint32(d[4:8]))
	leapcnt := int(binary.BigEndian.Uint32(d[8:12]))
	timecnt := int(binary.BigEndian.Uint32(d[12:16]))
	typecnt := int(binary.BigEndian.Uint32(d[16:20]))
	charcnt := int(binary.BigEndian.Uint32(d[20:24]))

	if typecnt == 0 {
		return nil, fmt.Errorf("%w: no time types", ErrBadData)
	}

	// Calculate v1 data block size to skip it for v2+ files.
	v1dataSize := timecnt*4 + // transition times (int32)
		timecnt*1 + // transition type indices
		typecnt*6 + // ttinfo records
		charcnt + // abbreviation chars
		leapcnt*8 + // leap second records (v1: 4+4)
		isstdcnt + // std/wall indicators
		isutcnt // UT/local indicators

	d = data[44:] // skip header
	if len(d) < v1dataSize {
		return nil, fmt.Errorf("%w: v1 data block truncated", ErrBadData)
	}

	if version >= 2 {
		// Skip v1 data block and read v2+ header.
		d = d[v1dataSize:]
		if len(d) < 44 {
			return nil, fmt.Errorf("%w: v2 header truncated", ErrBadData)
		}
		if string(d[:4]) != "TZif" {
			return nil, fmt.Errorf("%w: v2 magic mismatch", ErrBadData)
		}

		// Re-read counts from v2 header.
		d = d[20:]
		isutcnt = int(binary.BigEndian.Uint32(d[0:4]))
		isstdcnt = int(binary.BigEndian.Uint32(d[4:8]))
		leapcnt = int(binary.BigEndian.Uint32(d[8:12]))
		timecnt = int(binary.BigEndian.Uint32(d[12:16]))
		typecnt = int(binary.BigEndian.Uint32(d[16:20]))
		charcnt = int(binary.BigEndian.Uint32(d[20:24]))

		if typecnt == 0 {
			return nil, fmt.Errorf("%w: no time types in v2 block", ErrBadData)
		}

		d = d[24:] // skip v2 counts
	} else {
		d = data[44:] // start of v1 data
	}

	// Now parse the data block.
	// For v1: transition times are 4 bytes (int32).
	// For v2+: transition times are 8 bytes (int64).
	timeSize := 4
	leapSize := 8 // v1: 4 (time) + 4 (correction)
	if version >= 2 {
		timeSize = 8
		leapSize = 12 // v2+: 8 (time) + 4 (correction)
	}

	totalNeeded := timecnt*timeSize + // transition times
		timecnt*1 + // transition type indices
		typecnt*6 + // ttinfo records
		charcnt + // abbreviation chars
		leapcnt*leapSize + // leap second records
		isstdcnt + // std/wall indicators
		isutcnt // UT/local indicators

	if len(d) < totalNeeded {
		return nil, fmt.Errorf("%w: data block truncated", ErrBadData)
	}

	// Read transition times.
	transitions := make([]Transition, timecnt)
	for i := 0; i < timecnt; i++ {
		if version >= 2 {
			transitions[i].When = int64(binary.BigEndian.Uint64(d[:8]))
			d = d[8:]
		} else {
			transitions[i].When = int64(int32(binary.BigEndian.Uint32(d[:4])))
			d = d[4:]
		}
	}

	// Read transition type indices.
	for i := 0; i < timecnt; i++ {
		idx := int(d[0])
		if idx >= typecnt {
			return nil, fmt.Errorf("%w: transition type index %d out of range (typecnt=%d)", ErrBadData, idx, typecnt)
		}
		transitions[i].Type = idx
		d = d[1:]
	}

	// Read ttinfo records (6 bytes each).
	types := make([]ZoneType, typecnt)
	abbrIndices := make([]int, typecnt) // save for abbreviation lookup
	for i := 0; i < typecnt; i++ {
		offset := int32(binary.BigEndian.Uint32(d[0:4]))
		isDST := d[4] != 0
		abbrIdx := int(d[5])
		types[i] = ZoneType{
			Offset: int(offset),
			IsDST:  isDST,
		}
		abbrIndices[i] = abbrIdx
		d = d[6:]
	}

	// Read abbreviation string block.
	if charcnt > 0 {
		abbrevBlock := d[:charcnt]
		for i, idx := range abbrIndices {
			if idx >= charcnt {
				return nil, fmt.Errorf("%w: abbreviation index %d out of range", ErrBadData, idx)
			}
			types[i].Abbrev = byteString(abbrevBlock[idx:])
		}
		d = d[charcnt:]
	}

	// Read leap second records.
	leapSeconds := make([]LeapSecond, leapcnt)
	for i := 0; i < leapcnt; i++ {
		if version >= 2 {
			leapSeconds[i].When = int64(binary.BigEndian.Uint64(d[:8]))
			d = d[8:]
		} else {
			leapSeconds[i].When = int64(int32(binary.BigEndian.Uint32(d[:4])))
			d = d[4:]
		}
		leapSeconds[i].Correction = int32(binary.BigEndian.Uint32(d[:4]))
		d = d[4:]
	}

	// Read std/wall indicators.
	for i := 0; i < isstdcnt && i < timecnt; i++ {
		transitions[i].IsStd = d[0] != 0
		d = d[1:]
	}
	if isstdcnt > timecnt {
		d = d[isstdcnt-timecnt:]
	}

	// Read UT/local indicators.
	for i := 0; i < isutcnt && i < timecnt; i++ {
		transitions[i].IsUT = d[0] != 0
		d = d[1:]
	}
	if isutcnt > timecnt {
		d = d[isutcnt-timecnt:]
	}

	// Read POSIX TZ footer string (v2+ only).
	var extendRaw string
	var extend *PosixTZ
	if version >= 2 && len(d) > 1 && d[0] == '\n' {
		d = d[1:]
		if idx := indexByte(d, '\n'); idx >= 0 {
			extendRaw = string(d[:idx])
			if extendRaw != "" {
				p, err := ParsePosixTZ(extendRaw)
				if err == nil {
					extend = p
				}
			}
		}
	}

	z := &Zone{
		name:        name,
		version:     version,
		types:       types,
		transitions: transitions,
		leapSeconds: leapSeconds,
		extend:      extend,
		extendRaw:   extendRaw,
		rawData:     append([]byte(nil), data...), // defensive copy
	}

	return z, nil
}

// byteString extracts a NUL-terminated string from a byte slice.
func byteString(p []byte) string {
	for i, b := range p {
		if b == 0 {
			return string(p[:i])
		}
	}
	return string(p)
}

// indexByte returns the index of the first occurrence of c in s, or -1.
func indexByte(s []byte, c byte) int {
	for i, b := range s {
		if b == c {
			return i
		}
	}
	return -1
}
