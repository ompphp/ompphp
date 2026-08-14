package anyhash

// MD2 implementation per RFC 1319.

import "fmt"

const md2BlockSize = 16
const md2Size = 16

// S-box from RFC 1319, derived from the digits of pi.
var md2S = [256]byte{
	41, 46, 67, 201, 162, 216, 124, 1, 61, 54, 84, 161, 236, 240, 6, 19,
	98, 167, 5, 243, 192, 199, 115, 140, 152, 147, 43, 217, 188, 76, 130, 202,
	30, 155, 87, 60, 253, 212, 224, 22, 103, 66, 111, 24, 138, 23, 229, 18,
	190, 78, 196, 214, 218, 158, 222, 73, 160, 251, 245, 142, 187, 47, 238, 122,
	169, 104, 121, 145, 21, 178, 7, 63, 148, 194, 16, 137, 11, 34, 95, 33,
	128, 127, 93, 154, 90, 144, 50, 39, 53, 62, 204, 231, 191, 247, 151, 3,
	255, 25, 48, 179, 72, 165, 181, 209, 215, 94, 146, 42, 172, 86, 170, 198,
	79, 184, 56, 210, 150, 164, 125, 182, 118, 252, 107, 226, 156, 116, 4, 241,
	69, 157, 112, 89, 100, 113, 135, 32, 134, 91, 207, 101, 230, 45, 168, 2,
	27, 96, 37, 173, 174, 176, 185, 246, 28, 70, 97, 105, 52, 64, 126, 15,
	85, 71, 163, 35, 221, 81, 175, 58, 195, 92, 249, 206, 186, 197, 234, 38,
	44, 83, 13, 110, 133, 40, 132, 9, 211, 223, 205, 244, 65, 129, 77, 82,
	106, 220, 55, 200, 108, 193, 171, 250, 36, 225, 123, 8, 12, 189, 177, 74,
	120, 136, 149, 139, 227, 99, 232, 109, 233, 203, 213, 254, 59, 0, 29, 57,
	242, 239, 183, 14, 102, 88, 208, 228, 166, 119, 114, 248, 235, 117, 75, 10,
	49, 68, 80, 180, 143, 237, 31, 26, 219, 153, 141, 51, 159, 17, 131, 20,
}

type md2digest struct {
	state    [48]byte // working state (X in the RFC)
	checksum [16]byte // running checksum (C in the RFC)
	buf      [16]byte // partial block buffer
	bufLen   int      // bytes in buf
	len      uint64   // total bytes written
}

func newMD2() *md2digest {
	return &md2digest{}
}

func (d *md2digest) Size() int      { return md2Size }
func (d *md2digest) BlockSize() int { return md2BlockSize }

func (d *md2digest) Reset() {
	*d = md2digest{}
}

func (d *md2digest) Clone() Hash {
	c := *d
	return &c
}

// PHP format: MD2 is special - state is stored as byte strings, not int32 values.
// ints=[bufLen], buf=state(48)+checksum(16)+buf(16)=80 bytes
func (d *md2digest) PHPAlgo() string { return "md2" }
func (d *md2digest) MarshalPHP() []any {
	stateBytes := make([]byte, 48)
	copy(stateBytes, d.state[:])
	checksumBytes := make([]byte, 16)
	copy(checksumBytes, d.checksum[:])
	bufBytes := make([]byte, 16)
	copy(bufBytes, d.buf[:])
	return []any{stateBytes, checksumBytes, bufBytes, int32(d.bufLen)}
}
func (d *md2digest) UnmarshalPHP(state []any) error {
	if len(state) < 4 {
		return fmt.Errorf("anyhash: md2 PHP state needs 4 elements, got %d", len(state))
	}
	sb := phpBuf(state, 0)
	if len(sb) < 48 {
		return fmt.Errorf("anyhash: md2 PHP state[0] needs 48 bytes, got %d", len(sb))
	}
	copy(d.state[:], sb[:48])
	cb := phpBuf(state, 1)
	if len(cb) < 16 {
		return fmt.Errorf("anyhash: md2 PHP state[1] needs 16 bytes, got %d", len(cb))
	}
	copy(d.checksum[:], cb[:16])
	bb := phpBuf(state, 2)
	if len(bb) < 16 {
		return fmt.Errorf("anyhash: md2 PHP state[2] needs 16 bytes, got %d", len(bb))
	}
	copy(d.buf[:], bb[:16])
	d.bufLen = int(phpInt(state, 3))
	return nil
}

func (d *md2digest) Write(p []byte) (int, error) {
	n := len(p)
	d.len += uint64(n)

	// Fill partial block.
	if d.bufLen > 0 {
		fill := copy(d.buf[d.bufLen:], p)
		d.bufLen += fill
		p = p[fill:]
		if d.bufLen == md2BlockSize {
			d.processBlock(d.buf[:])
			d.bufLen = 0
		}
	}

	// Process full blocks.
	for len(p) >= md2BlockSize {
		d.processBlock(p[:md2BlockSize])
		p = p[md2BlockSize:]
	}

	// Save remainder.
	if len(p) > 0 {
		d.bufLen = copy(d.buf[:], p)
	}

	return n, nil
}

func (d *md2digest) processBlock(block []byte) {
	// Update checksum.
	var l byte = d.checksum[15]
	for j := 0; j < 16; j++ {
		l = d.checksum[j] ^ md2S[block[j]^l]
		d.checksum[j] = l
	}

	// Update state.
	for j := 0; j < 16; j++ {
		d.state[16+j] = block[j]
		d.state[32+j] = block[j] ^ d.state[j]
	}

	var t byte
	for j := 0; j < 18; j++ {
		for k := 0; k < 48; k++ {
			t = d.state[k] ^ md2S[t]
			d.state[k] = t
		}
		t += byte(j)
	}
}

func (d *md2digest) Sum(in []byte) []byte {
	// Work on a copy so the caller can continue writing.
	d0 := *d

	// Padding: pad to a full block with the padding byte equal to the
	// number of padding bytes needed.
	padLen := md2BlockSize - d0.bufLen
	var pad [md2BlockSize]byte
	for i := range pad {
		pad[i] = byte(padLen)
	}
	d0.Write(pad[:padLen])

	// Append checksum block. Copy first since processBlock modifies
	// d.checksum in place, which would alias the block data.
	var checksum [16]byte
	copy(checksum[:], d0.checksum[:])
	d0.processBlock(checksum[:])

	return append(in, d0.state[:16]...)
}
