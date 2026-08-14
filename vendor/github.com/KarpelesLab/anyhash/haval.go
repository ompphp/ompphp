package anyhash

// HAVAL hash algorithm implementation.
// Reference: Y. Zheng, J. Pieprzyk and J. Seberry, "HAVAL --- a one-way
// hashing algorithm with variable length of output", AUSCRYPT'92.

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

func init() {
	for _, passes := range []int{3, 4, 5} {
		for _, size := range []int{128, 160, 192, 224, 256} {
			p, s := passes, size
			name := fmt.Sprintf("haval%d,%d", s, p)
			registerHash(name, func() Hash { return newHaval(p, s) })
		}
	}
}

const havalBlockSize = 128 // 1024 bits
const havalVersion = 1

type havalDigest struct {
	s      [8]uint32 // hash state (fingerprint)
	buf    [128]byte // partial block buffer
	len    uint64    // total bytes written
	passes int       // 3, 4, or 5
	size   int       // digest size in bits (128, 160, 192, 224, 256)
}

func newHaval(passes, sizeBits int) *havalDigest {
	d := &havalDigest{
		passes: passes,
		size:   sizeBits,
	}
	d.Reset()
	return d
}

func (d *havalDigest) Size() int      { return d.size / 8 }
func (d *havalDigest) BlockSize() int { return havalBlockSize }

func (d *havalDigest) Reset() {
	d.s[0] = 0x243F6A88
	d.s[1] = 0x85A308D3
	d.s[2] = 0x13198A2E
	d.s[3] = 0x03707344
	d.s[4] = 0xA4093822
	d.s[5] = 0x299F31D0
	d.s[6] = 0x082EFA98
	d.s[7] = 0xEC4E6C89
	d.len = 0
}

func (d *havalDigest) Clone() Hash {
	c := *d
	return &c
}

// PHP format: [h0..h7, bitCountLo, bitCountHi] + buffer(128)
func (d *havalDigest) PHPAlgo() string {
	return fmt.Sprintf("haval%d,%d", d.size, d.passes)
}
func (d *havalDigest) MarshalPHP() []any {
	state := make([]any, 0, 11)
	for i := 0; i < 8; i++ {
		state = append(state, int32(d.s[i]))
	}
	bitCount := d.len * 8
	lo, hi := u64toi32pair(bitCount)
	state = append(state, lo, hi)
	buf := make([]byte, 128)
	bufLen := int(d.len % havalBlockSize)
	copy(buf, d.buf[:bufLen])
	state = append(state, buf)
	return state
}
func (d *havalDigest) UnmarshalPHP(state []any) error {
	if len(state) < 11 {
		return fmt.Errorf("anyhash: haval PHP state needs 11 elements, got %d", len(state))
	}
	for i := 0; i < 8; i++ {
		d.s[i] = uint32(phpInt(state, i))
	}
	bitCount := i32pairtou64(phpInt(state, 8), phpInt(state, 9))
	d.len = bitCount / 8
	copy(d.buf[:], phpBuf(state, 10))
	return nil
}

func (d *havalDigest) Write(p []byte) (int, error) {
	n := len(p)
	d.len += uint64(n)

	bufLen := int((d.len - uint64(n)) % havalBlockSize)

	if bufLen > 0 {
		fill := copy(d.buf[bufLen:], p)
		bufLen += fill
		p = p[fill:]
		if bufLen == havalBlockSize {
			havalBlock(&d.s, d.buf[:], d.passes)
		}
	}

	for len(p) >= havalBlockSize {
		havalBlock(&d.s, p[:havalBlockSize], d.passes)
		p = p[havalBlockSize:]
	}

	if len(p) > 0 {
		copy(d.buf[:], p)
	}

	return n, nil
}

func (d *havalDigest) Sum(in []byte) []byte {
	// Work on a copy so caller can continue writing.
	d0 := *d

	// Build padding.
	var tmp [138]byte // max: 128 + 10
	tmp[0] = 0x01     // HAVAL pads with a 1 bit in LSB position

	// Pad to 118 mod 128.
	bufLen := d0.len % havalBlockSize
	var padLen uint64
	if bufLen < 118 {
		padLen = 118 - bufLen
	} else {
		padLen = 246 - bufLen
	}

	// Append the 10-byte tail: 2 bytes version/passes/fptlen + 8 bytes bit count.
	tail := tmp[padLen:]
	tail[0] = byte(((d0.size & 0x3) << 6) | ((d0.passes & 0x7) << 3) | (havalVersion & 0x7))
	tail[1] = byte((d0.size >> 2) & 0xFF)
	binary.LittleEndian.PutUint64(tail[2:], d0.len*8)

	d0.Write(tmp[:padLen+10])

	// Tailor (fold) the output.
	havalTailor(&d0.s, d0.size)

	var digest [32]byte
	outWords := d0.size / 32
	for i := 0; i < outWords; i++ {
		binary.LittleEndian.PutUint32(digest[i*4:], d0.s[i])
	}
	return append(in, digest[:d0.size/8]...)
}

// HAVAL boolean functions.

func havalF1(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return x1&(x0^x4) ^ x2&x5 ^ x3&x6 ^ x0
}

func havalF2(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return x2&(x1&^x3^x4&x5^x6^x0) ^ x4&(x1^x5) ^ x3&x5 ^ x0
}

func havalF3(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return x3&(x1&x2^x6^x0) ^ x1&x4 ^ x2&x5 ^ x0
}

func havalF4(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return x4&(x5&^x2^x3&^x6^x1^x6^x0) ^ x3&(x1&x2^x5^x6) ^ x2&x6 ^ x0
}

func havalF5(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return x0&(x1&x2&x3^^x5) ^ x1&x4 ^ x2&x5 ^ x3&x6
}

// Fphi functions: apply the permutation for the given pass and number of total passes,
// then call the corresponding boolean function.

func havalFphi1(passes int, x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	switch passes {
	case 3:
		return havalF1(x1, x0, x3, x5, x6, x2, x4)
	case 4:
		return havalF1(x2, x6, x1, x4, x5, x3, x0)
	default: // 5
		return havalF1(x3, x4, x1, x0, x5, x2, x6)
	}
}

func havalFphi2(passes int, x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	switch passes {
	case 3:
		return havalF2(x4, x2, x1, x0, x5, x3, x6)
	case 4:
		return havalF2(x3, x5, x2, x0, x1, x6, x4)
	default: // 5
		return havalF2(x6, x2, x1, x0, x3, x4, x5)
	}
}

func havalFphi3(passes int, x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	switch passes {
	case 3:
		return havalF3(x6, x1, x2, x3, x4, x5, x0)
	case 4:
		return havalF3(x1, x4, x3, x6, x0, x2, x5)
	default: // 5
		return havalF3(x2, x6, x0, x4, x3, x1, x5)
	}
}

func havalFphi4(passes int, x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	switch passes {
	case 4:
		return havalF4(x6, x4, x0, x5, x2, x1, x3)
	default: // 5
		return havalF4(x1, x5, x3, x2, x0, x4, x6)
	}
}

func havalFphi5(x6, x5, x4, x3, x2, x1, x0 uint32) uint32 {
	return havalF5(x2, x5, x0, x6, x4, x3, x1)
}

// Word order for each pass.
var havalWordOrder = [5][32]int{
	// Pass 1
	{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31},
	// Pass 2
	{5, 14, 26, 18, 11, 28, 7, 16, 0, 23, 20, 22, 1, 10, 4, 8, 30, 3, 21, 9, 17, 24, 29, 6, 19, 12, 15, 13, 2, 25, 31, 27},
	// Pass 3
	{19, 9, 4, 20, 28, 17, 8, 22, 29, 14, 25, 12, 24, 30, 16, 26, 31, 15, 7, 3, 1, 0, 18, 27, 13, 6, 21, 10, 23, 11, 5, 2},
	// Pass 4
	{24, 4, 0, 14, 2, 7, 28, 23, 26, 6, 30, 20, 18, 25, 19, 3, 22, 11, 31, 21, 8, 27, 12, 9, 1, 29, 5, 15, 17, 10, 16, 13},
	// Pass 5
	{27, 3, 21, 26, 17, 11, 20, 29, 19, 0, 12, 7, 13, 8, 31, 10, 5, 9, 14, 30, 18, 6, 28, 24, 2, 23, 16, 22, 4, 1, 25, 15},
}

// Constants for passes 2-5 (pass 1 has no constants).
var havalK = [4][32]uint32{
	// Pass 2
	{
		0x452821E6, 0x38D01377, 0xBE5466CF, 0x34E90C6C,
		0xC0AC29B7, 0xC97C50DD, 0x3F84D5B5, 0xB5470917,
		0x9216D5D9, 0x8979FB1B, 0xD1310BA6, 0x98DFB5AC,
		0x2FFD72DB, 0xD01ADFB7, 0xB8E1AFED, 0x6A267E96,
		0xBA7C9045, 0xF12C7F99, 0x24A19947, 0xB3916CF7,
		0x0801F2E2, 0x858EFC16, 0x636920D8, 0x71574E69,
		0xA458FEA3, 0xF4933D7E, 0x0D95748F, 0x728EB658,
		0x718BCD58, 0x82154AEE, 0x7B54A41D, 0xC25A59B5,
	},
	// Pass 3
	{
		0x9C30D539, 0x2AF26013, 0xC5D1B023, 0x286085F0,
		0xCA417918, 0xB8DB38EF, 0x8E79DCB0, 0x603A180E,
		0x6C9E0E8B, 0xB01E8A3E, 0xD71577C1, 0xBD314B27,
		0x78AF2FDA, 0x55605C60, 0xE65525F3, 0xAA55AB94,
		0x57489862, 0x63E81440, 0x55CA396A, 0x2AAB10B6,
		0xB4CC5C34, 0x1141E8CE, 0xA15486AF, 0x7C72E993,
		0xB3EE1411, 0x636FBC2A, 0x2BA9C55D, 0x741831F6,
		0xCE5C3E16, 0x9B87931E, 0xAFD6BA33, 0x6C24CF5C,
	},
	// Pass 4
	{
		0x7A325381, 0x28958677, 0x3B8F4898, 0x6B4BB9AF,
		0xC4BFE81B, 0x66282193, 0x61D809CC, 0xFB21A991,
		0x487CAC60, 0x5DEC8032, 0xEF845D5D, 0xE98575B1,
		0xDC262302, 0xEB651B88, 0x23893E81, 0xD396ACC5,
		0x0F6D6FF3, 0x83F44239, 0x2E0B4482, 0xA4842004,
		0x69C8F04A, 0x9E1F9B5E, 0x21C66842, 0xF6E96C9A,
		0x670C9C61, 0xABD388F0, 0x6A51A0D2, 0xD8542F68,
		0x960FA728, 0xAB5133A3, 0x6EEF0B6C, 0x137A3BE4,
	},
	// Pass 5
	{
		0xBA3BF050, 0x7EFB2A98, 0xA1F1651D, 0x39AF0176,
		0x66CA593E, 0x82430E88, 0x8CEE8619, 0x456F9FB4,
		0x7D84A5C3, 0x3B8B5EBE, 0xE06F75D8, 0x85C12073,
		0x401A449F, 0x56C16AA6, 0x4ED3AA62, 0x363F7706,
		0x1BFEDF72, 0x429B023D, 0x37D0D724, 0xD00A1248,
		0xDB0FEAD3, 0x49F1C09B, 0x075372C9, 0x80991B7B,
		0x25D479D8, 0xF6E8DEF7, 0xE3FE501A, 0xB6794C3B,
		0x976CE0BD, 0x04C006BA, 0xC1A94FB6, 0x409F60C4,
	},
}

// havalBlock processes a single 128-byte block.
func havalBlock(s *[8]uint32, block []byte, passes int) {
	var w [32]uint32
	for i := 0; i < 32; i++ {
		w[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	t0, t1, t2, t3 := s[0], s[1], s[2], s[3]
	t4, t5, t6, t7 := s[4], s[5], s[6], s[7]

	// Pass 1: no constants.
	for i := 0; i < 32; i++ {
		temp := havalFphi1(passes, t6, t5, t4, t3, t2, t1, t0)
		t7 = bits.RotateLeft32(temp, -7) + bits.RotateLeft32(t7, -11) + w[havalWordOrder[0][i]]
		// Rotate registers: t7 becomes the new value, shift all down.
		t0, t1, t2, t3, t4, t5, t6, t7 = t7, t0, t1, t2, t3, t4, t5, t6
	}

	// Pass 2: with constants.
	for i := 0; i < 32; i++ {
		temp := havalFphi2(passes, t6, t5, t4, t3, t2, t1, t0)
		t7 = bits.RotateLeft32(temp, -7) + bits.RotateLeft32(t7, -11) + w[havalWordOrder[1][i]] + havalK[0][i]
		t0, t1, t2, t3, t4, t5, t6, t7 = t7, t0, t1, t2, t3, t4, t5, t6
	}

	// Pass 3: with constants.
	for i := 0; i < 32; i++ {
		temp := havalFphi3(passes, t6, t5, t4, t3, t2, t1, t0)
		t7 = bits.RotateLeft32(temp, -7) + bits.RotateLeft32(t7, -11) + w[havalWordOrder[2][i]] + havalK[1][i]
		t0, t1, t2, t3, t4, t5, t6, t7 = t7, t0, t1, t2, t3, t4, t5, t6
	}

	if passes >= 4 {
		// Pass 4: with constants.
		for i := 0; i < 32; i++ {
			temp := havalFphi4(passes, t6, t5, t4, t3, t2, t1, t0)
			t7 = bits.RotateLeft32(temp, -7) + bits.RotateLeft32(t7, -11) + w[havalWordOrder[3][i]] + havalK[2][i]
			t0, t1, t2, t3, t4, t5, t6, t7 = t7, t0, t1, t2, t3, t4, t5, t6
		}
	}

	if passes >= 5 {
		// Pass 5: with constants.
		for i := 0; i < 32; i++ {
			temp := havalFphi5(t6, t5, t4, t3, t2, t1, t0)
			t7 = bits.RotateLeft32(temp, -7) + bits.RotateLeft32(t7, -11) + w[havalWordOrder[4][i]] + havalK[3][i]
			t0, t1, t2, t3, t4, t5, t6, t7 = t7, t0, t1, t2, t3, t4, t5, t6
		}
	}

	s[0] += t0
	s[1] += t1
	s[2] += t2
	s[3] += t3
	s[4] += t4
	s[5] += t5
	s[6] += t6
	s[7] += t7
}

// havalTailor folds the 256-bit internal state to the desired output size.
func havalTailor(s *[8]uint32, sizeBits int) {
	switch sizeBits {
	case 128:
		temp := (s[7] & 0x000000FF) |
			(s[6] & 0xFF000000) |
			(s[5] & 0x00FF0000) |
			(s[4] & 0x0000FF00)
		s[0] += bits.RotateLeft32(temp, -8)

		temp = (s[7] & 0x0000FF00) |
			(s[6] & 0x000000FF) |
			(s[5] & 0xFF000000) |
			(s[4] & 0x00FF0000)
		s[1] += bits.RotateLeft32(temp, -16)

		temp = (s[7] & 0x00FF0000) |
			(s[6] & 0x0000FF00) |
			(s[5] & 0x000000FF) |
			(s[4] & 0xFF000000)
		s[2] += bits.RotateLeft32(temp, -24)

		temp = (s[7] & 0xFF000000) |
			(s[6] & 0x00FF0000) |
			(s[5] & 0x0000FF00) |
			(s[4] & 0x000000FF)
		s[3] += temp

	case 160:
		temp := (s[7] & 0x3F) |
			(s[6] & (0x7F << 25)) |
			(s[5] & (0x3F << 19))
		s[0] += bits.RotateLeft32(temp, -19)

		temp = (s[7] & (0x3F << 6)) |
			(s[6] & 0x3F) |
			(s[5] & (0x7F << 25))
		s[1] += bits.RotateLeft32(temp, -25)

		temp = (s[7] & (0x7F << 12)) |
			(s[6] & (0x3F << 6)) |
			(s[5] & 0x3F)
		s[2] += temp

		temp = (s[7] & (0x3F << 19)) |
			(s[6] & (0x7F << 12)) |
			(s[5] & (0x3F << 6))
		s[3] += temp >> 6

		temp = (s[7] & (0x7F << 25)) |
			(s[6] & (0x3F << 19)) |
			(s[5] & (0x7F << 12))
		s[4] += temp >> 12

	case 192:
		temp := (s[7] & 0x1F) |
			(s[6] & (0x3F << 26))
		s[0] += bits.RotateLeft32(temp, -26)

		temp = (s[7] & (0x1F << 5)) |
			(s[6] & 0x1F)
		s[1] += temp

		temp = (s[7] & (0x3F << 10)) |
			(s[6] & (0x1F << 5))
		s[2] += temp >> 5

		temp = (s[7] & (0x1F << 16)) |
			(s[6] & (0x3F << 10))
		s[3] += temp >> 10

		temp = (s[7] & (0x1F << 21)) |
			(s[6] & (0x1F << 16))
		s[4] += temp >> 16

		temp = (s[7] & (0x3F << 26)) |
			(s[6] & (0x1F << 21))
		s[5] += temp >> 21

	case 224:
		s[0] += (s[7] >> 27) & 0x1F
		s[1] += (s[7] >> 22) & 0x1F
		s[2] += (s[7] >> 18) & 0x0F
		s[3] += (s[7] >> 13) & 0x1F
		s[4] += (s[7] >> 9) & 0x0F
		s[5] += (s[7] >> 4) & 0x1F
		s[6] += s[7] & 0x0F

	case 256:
		// No folding needed.
	}
}
