package anyhash

// MurmurHash3 implementation.
// murmur3a = 32-bit (x86), murmur3c = 128-bit x86, murmur3f = 128-bit x64.
// All use seed 0 to match PHP's hash() behavior.

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

func init() {
	registerHash("murmur3a", func() Hash { return newMurmur3a() })
	registerHash("murmur3c", func() Hash { return newMurmur3c() })
	registerHash("murmur3f", func() Hash { return newMurmur3f() })
}

// --- murmur3a: 32-bit x86 ---

type murmur3aDigest struct {
	h1   uint32
	seed uint32
	buf  [4]byte
	n    int    // bytes in buf
	len  uint32 // total bytes
}

func newMurmur3a() *murmur3aDigest {
	return &murmur3aDigest{}
}

func (d *murmur3aDigest) SetSeed(seed uint64) {
	d.seed = uint32(seed)
	d.h1 = d.seed
}

func (d *murmur3aDigest) Size() int      { return 4 }
func (d *murmur3aDigest) BlockSize() int { return 4 }
func (d *murmur3aDigest) Reset()         { s := d.seed; *d = murmur3aDigest{seed: s, h1: s} }
func (d *murmur3aDigest) Clone() Hash    { c := *d; return &c }

// PHP format: [h1, carry, len] — 3 ints, no buffer.
// carry = n | buf[0]<<((4-n)*8) | buf[1]<<((5-n)*8) | buf[2]<<((6-n)*8)
// Bytes pack toward MSB, count at LSB.
func (d *murmur3aDigest) PHPAlgo() string { return "murmur3a" }
func (d *murmur3aDigest) MarshalPHP() []any {
	carry := uint32(d.n)
	for i := 0; i < d.n; i++ {
		carry |= uint32(d.buf[i]) << ((4 - d.n + i) * 8)
	}
	return []any{int32(d.h1), int32(carry), int32(d.len)}
}
func (d *murmur3aDigest) UnmarshalPHP(state []any) error {
	if len(state) < 3 {
		return fmt.Errorf("anyhash: murmur3a PHP state needs 3 elements, got %d", len(state))
	}
	d.h1 = uint32(phpInt(state, 0))
	carry := uint32(phpInt(state, 1))
	d.n = int(carry & 0xFF)
	for i := 0; i < d.n; i++ {
		d.buf[i] = byte(carry >> ((4 - d.n + i) * 8))
	}
	d.len = uint32(phpInt(state, 2))
	return nil
}

func (d *murmur3aDigest) Write(p []byte) (int, error) {
	total := len(p)
	d.len += uint32(total)

	if d.n > 0 {
		fill := copy(d.buf[d.n:], p)
		d.n += fill
		p = p[fill:]
		if d.n == 4 {
			d.h1 = murmur3aBlock(d.h1, binary.LittleEndian.Uint32(d.buf[:]))
			d.n = 0
		}
	}

	for len(p) >= 4 {
		d.h1 = murmur3aBlock(d.h1, binary.LittleEndian.Uint32(p))
		p = p[4:]
	}

	if len(p) > 0 {
		d.n = copy(d.buf[:], p)
	}
	return total, nil
}

func murmur3aBlock(h1, k1 uint32) uint32 {
	k1 *= 0xcc9e2d51
	k1 = bits.RotateLeft32(k1, 15)
	k1 *= 0x1b873593
	h1 ^= k1
	h1 = bits.RotateLeft32(h1, 13)
	h1 = h1*5 + 0xe6546b64
	return h1
}

func (d *murmur3aDigest) Sum(in []byte) []byte {
	h1 := d.h1
	var k1 uint32
	switch d.n {
	case 3:
		k1 ^= uint32(d.buf[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(d.buf[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(d.buf[0])
		k1 *= 0xcc9e2d51
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= 0x1b873593
		h1 ^= k1
	}
	h1 ^= d.len
	h1 = fmix32(h1)
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], h1)
	return append(in, buf[:]...)
}

func fmix32(h uint32) uint32 {
	h ^= h >> 16
	h *= 0x85ebca6b
	h ^= h >> 13
	h *= 0xc2b2ae35
	h ^= h >> 16
	return h
}

// --- murmur3c: 128-bit x86 ---

type murmur3cDigest struct {
	h1, h2, h3, h4 uint32
	seed           uint32
	buf            [16]byte
	n              int
	len            uint32
}

func newMurmur3c() *murmur3cDigest {
	return &murmur3cDigest{}
}

func (d *murmur3cDigest) SetSeed(seed uint64) {
	d.seed = uint32(seed)
	d.h1 = d.seed
	d.h2 = d.seed
	d.h3 = d.seed
	d.h4 = d.seed
}

func (d *murmur3cDigest) Size() int      { return 16 }
func (d *murmur3cDigest) BlockSize() int { return 16 }
func (d *murmur3cDigest) Reset() {
	s := d.seed
	*d = murmur3cDigest{seed: s, h1: s, h2: s, h3: s, h4: s}
}
func (d *murmur3cDigest) Clone() Hash    { c := *d; return &c }

// PHP format: [h0..h3, carry[0..3], len] — 9 int32s (PHP_MURMUR3C_SPEC "lllllllll").
// carry[0..3] is a 16-byte buffer where full uint32s are raw little-endian and a
// partial uint32 packs bytes toward the high end (byte i at position (4-n_partial+i)).
// The low byte of carry[3] holds the carry length (0..15), per PHP's
// pcarry[3] = (k4 & ~0xff) | n.
func (d *murmur3cDigest) PHPAlgo() string { return "murmur3c" }
func (d *murmur3cDigest) MarshalPHP() []any {
	var carry [4]uint32
	nFull := d.n / 4
	partial := d.n % 4
	for i := 0; i < nFull; i++ {
		carry[i] = binary.LittleEndian.Uint32(d.buf[i*4:])
	}
	if partial > 0 {
		var pk uint32
		for i := 0; i < partial; i++ {
			pk |= uint32(d.buf[nFull*4+i]) << ((4 - partial + i) * 8)
		}
		carry[nFull] = pk
	}
	carry[3] = (carry[3] &^ 0xff) | uint32(d.n)
	return []any{
		int32(d.h1), int32(d.h2), int32(d.h3), int32(d.h4),
		int32(carry[0]), int32(carry[1]), int32(carry[2]), int32(carry[3]),
		int32(d.len),
	}
}
func (d *murmur3cDigest) UnmarshalPHP(state []any) error {
	if len(state) < 9 {
		return fmt.Errorf("anyhash: murmur3c PHP state needs 9 elements, got %d", len(state))
	}
	d.h1 = uint32(phpInt(state, 0))
	d.h2 = uint32(phpInt(state, 1))
	d.h3 = uint32(phpInt(state, 2))
	d.h4 = uint32(phpInt(state, 3))
	var carry [4]uint32
	carry[0] = uint32(phpInt(state, 4))
	carry[1] = uint32(phpInt(state, 5))
	carry[2] = uint32(phpInt(state, 6))
	carry[3] = uint32(phpInt(state, 7))
	d.len = uint32(phpInt(state, 8))
	d.n = int(carry[3] & 0xf)
	d.buf = [16]byte{}
	nFull := d.n / 4
	partial := d.n % 4
	for i := 0; i < nFull; i++ {
		binary.LittleEndian.PutUint32(d.buf[i*4:], carry[i])
	}
	if partial > 0 {
		pk := carry[nFull]
		for i := 0; i < partial; i++ {
			d.buf[nFull*4+i] = byte(pk >> ((4 - partial + i) * 8))
		}
	}
	return nil
}

func (d *murmur3cDigest) Write(p []byte) (int, error) {
	total := len(p)
	d.len += uint32(total)

	if d.n > 0 {
		fill := copy(d.buf[d.n:], p)
		d.n += fill
		p = p[fill:]
		if d.n == 16 {
			d.processBlock3c(d.buf[:])
			d.n = 0
		}
	}

	for len(p) >= 16 {
		d.processBlock3c(p[:16])
		p = p[16:]
	}

	if len(p) > 0 {
		d.n = copy(d.buf[:], p)
	}
	return total, nil
}

func (d *murmur3cDigest) processBlock3c(b []byte) {
	k1 := binary.LittleEndian.Uint32(b[0:])
	k2 := binary.LittleEndian.Uint32(b[4:])
	k3 := binary.LittleEndian.Uint32(b[8:])
	k4 := binary.LittleEndian.Uint32(b[12:])

	const c1, c2, c3, c4 = 0x239b961b, 0xab0e9789, 0x38b34ae5, 0xa1e38b93

	k1 *= c1
	k1 = bits.RotateLeft32(k1, 15)
	k1 *= c2
	d.h1 ^= k1
	d.h1 = bits.RotateLeft32(d.h1, 19)
	d.h1 += d.h2
	d.h1 = d.h1*5 + 0x561ccd1b

	k2 *= c2
	k2 = bits.RotateLeft32(k2, 16)
	k2 *= c3
	d.h2 ^= k2
	d.h2 = bits.RotateLeft32(d.h2, 17)
	d.h2 += d.h3
	d.h2 = d.h2*5 + 0x0bcaa747

	k3 *= c3
	k3 = bits.RotateLeft32(k3, 17)
	k3 *= c4
	d.h3 ^= k3
	d.h3 = bits.RotateLeft32(d.h3, 15)
	d.h3 += d.h4
	d.h3 = d.h3*5 + 0x96cd1c35

	k4 *= c4
	k4 = bits.RotateLeft32(k4, 18)
	k4 *= c1
	d.h4 ^= k4
	d.h4 = bits.RotateLeft32(d.h4, 13)
	d.h4 += d.h1
	d.h4 = d.h4*5 + 0x32ac3b17
}

func (d *murmur3cDigest) Sum(in []byte) []byte {
	h1, h2, h3, h4 := d.h1, d.h2, d.h3, d.h4
	const c1, c2, c3, c4 = 0x239b961b, 0xab0e9789, 0x38b34ae5, 0xa1e38b93

	var k1, k2, k3, k4 uint32
	tail := d.buf[:d.n]

	switch d.n {
	case 15:
		k4 ^= uint32(tail[14]) << 16
		fallthrough
	case 14:
		k4 ^= uint32(tail[13]) << 8
		fallthrough
	case 13:
		k4 ^= uint32(tail[12])
		k4 *= c4
		k4 = bits.RotateLeft32(k4, 18)
		k4 *= c1
		h4 ^= k4
		fallthrough
	case 12:
		k3 ^= uint32(tail[11]) << 24
		fallthrough
	case 11:
		k3 ^= uint32(tail[10]) << 16
		fallthrough
	case 10:
		k3 ^= uint32(tail[9]) << 8
		fallthrough
	case 9:
		k3 ^= uint32(tail[8])
		k3 *= c3
		k3 = bits.RotateLeft32(k3, 17)
		k3 *= c4
		h3 ^= k3
		fallthrough
	case 8:
		k2 ^= uint32(tail[7]) << 24
		fallthrough
	case 7:
		k2 ^= uint32(tail[6]) << 16
		fallthrough
	case 6:
		k2 ^= uint32(tail[5]) << 8
		fallthrough
	case 5:
		k2 ^= uint32(tail[4])
		k2 *= c2
		k2 = bits.RotateLeft32(k2, 16)
		k2 *= c3
		h2 ^= k2
		fallthrough
	case 4:
		k1 ^= uint32(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0])
		k1 *= c1
		k1 = bits.RotateLeft32(k1, 15)
		k1 *= c2
		h1 ^= k1
	}

	h1 ^= d.len
	h2 ^= d.len
	h3 ^= d.len
	h4 ^= d.len

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	h1 = fmix32(h1)
	h2 = fmix32(h2)
	h3 = fmix32(h3)
	h4 = fmix32(h4)

	h1 += h2
	h1 += h3
	h1 += h4
	h2 += h1
	h3 += h1
	h4 += h1

	var buf [16]byte
	binary.BigEndian.PutUint32(buf[0:], h1)
	binary.BigEndian.PutUint32(buf[4:], h2)
	binary.BigEndian.PutUint32(buf[8:], h3)
	binary.BigEndian.PutUint32(buf[12:], h4)
	return append(in, buf[:]...)
}

// --- murmur3f: 128-bit x64 ---

type murmur3fDigest struct {
	h1, h2 uint64
	seed   uint64
	buf    [16]byte
	n      int
	len    uint64
}

func newMurmur3f() *murmur3fDigest {
	return &murmur3fDigest{}
}

func (d *murmur3fDigest) SetSeed(seed uint64) {
	d.seed = seed
	d.h1 = seed
	d.h2 = seed
}

func (d *murmur3fDigest) Size() int      { return 16 }
func (d *murmur3fDigest) BlockSize() int { return 16 }
func (d *murmur3fDigest) Reset() {
	s := d.seed
	*d = murmur3fDigest{seed: s, h1: s, h2: s}
}
func (d *murmur3fDigest) Clone() Hash    { c := *d; return &c }

// PHP format: [h[0]_lo, h[0]_hi, h[1]_lo, h[1]_hi, carry[0]_lo, carry[0]_hi,
// carry[1]_lo, carry[1]_hi, len] — PHP_MURMUR3F_SPEC "qqqql", where each 64-bit
// value is serialized as (lo, hi) int32 pair. The carry buffer is 16 bytes split
// across 2 uint64s; a partial uint64 packs bytes toward the high end (byte i at
// position (8-n_partial+i)). The low byte of carry[1] holds the carry length
// (0..15), per PHP's pcarry[1] = (k2 & ~0xff) | n.
func (d *murmur3fDigest) PHPAlgo() string { return "murmur3f" }
func (d *murmur3fDigest) MarshalPHP() []any {
	h1lo, h1hi := u64toi32pair(d.h1)
	h2lo, h2hi := u64toi32pair(d.h2)
	var carry [2]uint64
	nFull := d.n / 8
	partial := d.n % 8
	for i := 0; i < nFull; i++ {
		carry[i] = binary.LittleEndian.Uint64(d.buf[i*8:])
	}
	if partial > 0 {
		var pk uint64
		for i := 0; i < partial; i++ {
			pk |= uint64(d.buf[nFull*8+i]) << ((8 - partial + i) * 8)
		}
		carry[nFull] = pk
	}
	carry[1] = (carry[1] &^ 0xff) | uint64(d.n)
	c0lo, c0hi := u64toi32pair(carry[0])
	c1lo, c1hi := u64toi32pair(carry[1])
	return []any{
		h1lo, h1hi, h2lo, h2hi,
		c0lo, c0hi, c1lo, c1hi,
		int32(d.len),
	}
}
func (d *murmur3fDigest) UnmarshalPHP(state []any) error {
	if len(state) < 9 {
		return fmt.Errorf("anyhash: murmur3f PHP state needs 9 elements, got %d", len(state))
	}
	d.h1 = i32pairtou64(phpInt(state, 0), phpInt(state, 1))
	d.h2 = i32pairtou64(phpInt(state, 2), phpInt(state, 3))
	var carry [2]uint64
	carry[0] = i32pairtou64(phpInt(state, 4), phpInt(state, 5))
	carry[1] = i32pairtou64(phpInt(state, 6), phpInt(state, 7))
	d.len = uint64(uint32(phpInt(state, 8)))
	d.n = int(carry[1] & 0xf)
	d.buf = [16]byte{}
	nFull := d.n / 8
	partial := d.n % 8
	for i := 0; i < nFull; i++ {
		binary.LittleEndian.PutUint64(d.buf[i*8:], carry[i])
	}
	if partial > 0 {
		pk := carry[nFull]
		for i := 0; i < partial; i++ {
			d.buf[nFull*8+i] = byte(pk >> ((8 - partial + i) * 8))
		}
	}
	return nil
}

func (d *murmur3fDigest) Write(p []byte) (int, error) {
	total := len(p)
	d.len += uint64(total)

	if d.n > 0 {
		fill := copy(d.buf[d.n:], p)
		d.n += fill
		p = p[fill:]
		if d.n == 16 {
			d.processBlock3f(d.buf[:])
			d.n = 0
		}
	}

	for len(p) >= 16 {
		d.processBlock3f(p[:16])
		p = p[16:]
	}

	if len(p) > 0 {
		d.n = copy(d.buf[:], p)
	}
	return total, nil
}

const (
	murmur3fc1 = 0x87c37b91114253d5
	murmur3fc2 = 0x4cf5ad432745937f
)

func (d *murmur3fDigest) processBlock3f(b []byte) {
	k1 := binary.LittleEndian.Uint64(b[0:])
	k2 := binary.LittleEndian.Uint64(b[8:])

	k1 *= murmur3fc1
	k1 = bits.RotateLeft64(k1, 31)
	k1 *= murmur3fc2
	d.h1 ^= k1
	d.h1 = bits.RotateLeft64(d.h1, 27)
	d.h1 += d.h2
	d.h1 = d.h1*5 + 0x52dce729

	k2 *= murmur3fc2
	k2 = bits.RotateLeft64(k2, 33)
	k2 *= murmur3fc1
	d.h2 ^= k2
	d.h2 = bits.RotateLeft64(d.h2, 31)
	d.h2 += d.h1
	d.h2 = d.h2*5 + 0x38495ab5
}

func (d *murmur3fDigest) Sum(in []byte) []byte {
	h1, h2 := d.h1, d.h2
	var k1, k2 uint64
	tail := d.buf[:d.n]

	switch d.n {
	case 15:
		k2 ^= uint64(tail[14]) << 48
		fallthrough
	case 14:
		k2 ^= uint64(tail[13]) << 40
		fallthrough
	case 13:
		k2 ^= uint64(tail[12]) << 32
		fallthrough
	case 12:
		k2 ^= uint64(tail[11]) << 24
		fallthrough
	case 11:
		k2 ^= uint64(tail[10]) << 16
		fallthrough
	case 10:
		k2 ^= uint64(tail[9]) << 8
		fallthrough
	case 9:
		k2 ^= uint64(tail[8])
		k2 *= murmur3fc2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= murmur3fc1
		h2 ^= k2
		fallthrough
	case 8:
		k1 ^= uint64(tail[7]) << 56
		fallthrough
	case 7:
		k1 ^= uint64(tail[6]) << 48
		fallthrough
	case 6:
		k1 ^= uint64(tail[5]) << 40
		fallthrough
	case 5:
		k1 ^= uint64(tail[4]) << 32
		fallthrough
	case 4:
		k1 ^= uint64(tail[3]) << 24
		fallthrough
	case 3:
		k1 ^= uint64(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint64(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint64(tail[0])
		k1 *= murmur3fc1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= murmur3fc2
		h1 ^= k1
	}

	h1 ^= d.len
	h2 ^= d.len
	h1 += h2
	h2 += h1
	h1 = fmix64(h1)
	h2 = fmix64(h2)
	h1 += h2
	h2 += h1

	var buf [16]byte
	binary.BigEndian.PutUint64(buf[0:], h1)
	binary.BigEndian.PutUint64(buf[8:], h2)
	return append(in, buf[:]...)
}

func fmix64(h uint64) uint64 {
	h ^= h >> 33
	h *= 0xff51afd7ed558ccd
	h ^= h >> 33
	h *= 0xc4ceb9fe1a85ec53
	h ^= h >> 33
	return h
}
