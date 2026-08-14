package anyhash

// RIPEMD-128, RIPEMD-160, RIPEMD-256, RIPEMD-320 implementations.

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

func init() {
	registerHash("ripemd128", func() Hash { return newRIPEMD(ripemd128params) })
	registerHash("ripemd160", func() Hash { return newRIPEMD(ripemd160params) })
	registerHash("ripemd256", func() Hash { return newRIPEMD(ripemd256params) })
	registerHash("ripemd320", func() Hash { return newRIPEMD(ripemd320params) })
}

const ripemdBlockSize = 64

// ripemdVariant holds the parameters that distinguish each RIPEMD variant.
type ripemdVariant struct {
	size     int        // digest size in bytes
	nstate   int        // number of state words
	phpName  string     // PHP algorithm name
	iv       [10]uint32 // initial state values
	compress func(s []uint32, block []byte)
}

var ripemd128params = &ripemdVariant{
	size:    16,
	nstate:  4,
	phpName: "ripemd128",
	iv: [10]uint32{
		0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476,
	},
	compress: ripemdCompress128,
}

var ripemd160params = &ripemdVariant{
	size:    20,
	nstate:  5,
	phpName: "ripemd160",
	iv: [10]uint32{
		0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476, 0xc3d2e1f0,
	},
	compress: ripemdCompress160,
}

var ripemd256params = &ripemdVariant{
	size:    32,
	nstate:  8,
	phpName: "ripemd256",
	iv: [10]uint32{
		0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476,
		0x76543210, 0xfedcba98, 0x89abcdef, 0x01234567,
	},
	compress: ripemdCompress256,
}

var ripemd320params = &ripemdVariant{
	size:    40,
	nstate:  10,
	phpName: "ripemd320",
	iv: [10]uint32{
		0x67452301, 0xefcdab89, 0x98badcfe, 0x10325476, 0xc3d2e1f0,
		0x76543210, 0xfedcba98, 0x89abcdef, 0x01234567, 0x3c2d1e0f,
	},
	compress: ripemdCompress320,
}

type ripemdDigest struct {
	s      [10]uint32
	buf    [64]byte
	len    uint64
	params *ripemdVariant
}

func newRIPEMD(p *ripemdVariant) *ripemdDigest {
	d := &ripemdDigest{params: p}
	d.Reset()
	return d
}

func (d *ripemdDigest) Size() int      { return d.params.size }
func (d *ripemdDigest) BlockSize() int { return ripemdBlockSize }

func (d *ripemdDigest) Reset() {
	d.s = d.params.iv
	d.len = 0
}

func (d *ripemdDigest) Clone() Hash {
	c := *d
	return &c
}

// PHP format: [h0..h(nstate-1), bitCountLo, bitCountHi, buffer(64)]
func (d *ripemdDigest) PHPAlgo() string { return d.params.phpName }
func (d *ripemdDigest) MarshalPHP() []any {
	n := d.params.nstate
	state := make([]any, 0, n+3)
	for i := 0; i < n; i++ {
		state = append(state, int32(d.s[i]))
	}
	bitCount := d.len * 8
	lo, hi := u64toi32pair(bitCount)
	state = append(state, lo, hi)
	buf := make([]byte, 64)
	bufLen := int(d.len % ripemdBlockSize)
	copy(buf, d.buf[:bufLen])
	state = append(state, buf)
	return state
}
func (d *ripemdDigest) UnmarshalPHP(state []any) error {
	n := d.params.nstate
	if len(state) < n+3 {
		return fmt.Errorf("anyhash: %s PHP state needs %d elements, got %d", d.params.phpName, n+3, len(state))
	}
	for i := 0; i < n; i++ {
		d.s[i] = uint32(phpInt(state, i))
	}
	bitCount := i32pairtou64(phpInt(state, n), phpInt(state, n+1))
	d.len = bitCount / 8
	copy(d.buf[:], phpBuf(state, n+2))
	return nil
}

func (d *ripemdDigest) Write(p []byte) (int, error) {
	n := len(p)
	d.len += uint64(n)

	bufLen := int((d.len - uint64(n)) % ripemdBlockSize)

	if bufLen > 0 {
		fill := copy(d.buf[bufLen:], p)
		bufLen += fill
		p = p[fill:]
		if bufLen == ripemdBlockSize {
			d.params.compress(d.s[:d.params.nstate], d.buf[:])
		}
	}

	for len(p) >= ripemdBlockSize {
		d.params.compress(d.s[:d.params.nstate], p[:ripemdBlockSize])
		p = p[ripemdBlockSize:]
	}

	if len(p) > 0 {
		copy(d.buf[:], p)
	}

	return n, nil
}

func (d *ripemdDigest) Sum(in []byte) []byte {
	d0 := *d

	var tmp [72]byte
	tmp[0] = 0x80
	bufLen := d0.len % ripemdBlockSize
	var padLen uint64
	if bufLen < 56 {
		padLen = 56 - bufLen
	} else {
		padLen = 64 + 56 - bufLen
	}
	binary.LittleEndian.PutUint64(tmp[padLen:], d0.len*8)
	d0.Write(tmp[:padLen+8])

	var digest [40]byte
	for i := 0; i < d0.params.nstate; i++ {
		binary.LittleEndian.PutUint32(digest[i*4:], d0.s[i])
	}
	return append(in, digest[:d0.params.size]...)
}

// Boolean functions used by all RIPEMD variants.
func ripemdF0(x, y, z uint32) uint32 { return x ^ y ^ z }
func ripemdF1(x, y, z uint32) uint32 { return (x & y) | (^x & z) }
func ripemdF2(x, y, z uint32) uint32 { return (x | ^y) ^ z }
func ripemdF3(x, y, z uint32) uint32 { return (x & z) | (y & ^z) }
func ripemdF4(x, y, z uint32) uint32 { return x ^ (y | ^z) }

// Left stream round constants.
var ripemdKL = [5]uint32{
	0x00000000, // round 0
	0x5a827999, // round 1
	0x6ed9eba1, // round 2
	0x8f1bbcdc, // round 3
	0xa953fd4e, // round 4 (160/320 only)
}

// Right stream round constants for RIPEMD-128/256 (4 rounds).
var ripemdKR128 = [4]uint32{
	0x50a28be6, // round 0
	0x5c4dd124, // round 1
	0x6d703ef3, // round 2
	0x00000000, // round 3
}

// Right stream round constants for RIPEMD-160/320 (5 rounds).
var ripemdKR160 = [5]uint32{
	0x50a28be6, // round 0
	0x5c4dd124, // round 1
	0x6d703ef3, // round 2
	0x7a6d76e9, // round 3
	0x00000000, // round 4
}

// Left stream message word selection per round (each round = 16 steps).
var ripemdRL = [80]int{
	// Round 0
	0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15,
	// Round 1
	7, 4, 13, 1, 10, 6, 15, 3, 12, 0, 9, 5, 2, 14, 11, 8,
	// Round 2
	3, 10, 14, 4, 9, 15, 8, 1, 2, 7, 0, 6, 13, 11, 5, 12,
	// Round 3
	1, 9, 11, 10, 0, 8, 12, 4, 13, 3, 7, 15, 14, 5, 6, 2,
	// Round 4 (160/320 only)
	4, 0, 5, 9, 7, 12, 2, 10, 14, 1, 3, 8, 11, 6, 15, 13,
}

// Right stream message word selection per round.
var ripemdRR = [80]int{
	// Round 0
	5, 14, 7, 0, 9, 2, 11, 4, 13, 6, 15, 8, 1, 10, 3, 12,
	// Round 1
	6, 11, 3, 7, 0, 13, 5, 10, 14, 15, 8, 12, 4, 9, 1, 2,
	// Round 2
	15, 5, 1, 3, 7, 14, 6, 9, 11, 8, 12, 2, 10, 0, 4, 13,
	// Round 3
	8, 6, 4, 1, 3, 11, 15, 0, 5, 12, 2, 13, 9, 7, 10, 14,
	// Round 4 (160/320 only)
	12, 15, 10, 4, 1, 5, 8, 7, 6, 2, 13, 14, 0, 3, 9, 11,
}

// Left stream rotation amounts per step.
var ripemdSL = [80]int{
	// Round 0
	11, 14, 15, 12, 5, 8, 7, 9, 11, 13, 14, 15, 6, 7, 9, 8,
	// Round 1
	7, 6, 8, 13, 11, 9, 7, 15, 7, 12, 15, 9, 11, 7, 13, 12,
	// Round 2
	11, 13, 6, 7, 14, 9, 13, 15, 14, 8, 13, 6, 5, 12, 7, 5,
	// Round 3
	11, 12, 14, 15, 14, 15, 9, 8, 9, 14, 5, 6, 8, 6, 5, 12,
	// Round 4 (160/320 only)
	9, 15, 5, 11, 6, 8, 13, 12, 5, 12, 13, 14, 11, 8, 5, 6,
}

// Right stream rotation amounts per step.
var ripemdSR = [80]int{
	// Round 0
	8, 9, 9, 11, 13, 15, 15, 5, 7, 7, 8, 11, 14, 14, 12, 6,
	// Round 1
	9, 13, 15, 7, 12, 8, 9, 11, 7, 7, 12, 7, 6, 15, 13, 11,
	// Round 2
	9, 7, 15, 11, 8, 6, 6, 14, 12, 13, 5, 14, 13, 13, 7, 5,
	// Round 3
	15, 5, 8, 11, 14, 14, 6, 14, 6, 9, 12, 9, 12, 5, 15, 8,
	// Round 4 (160/320 only)
	8, 5, 12, 9, 12, 5, 14, 6, 8, 13, 6, 5, 15, 13, 11, 11,
}

// ripemdCompress128 compresses a single 64-byte block for RIPEMD-128.
func ripemdCompress128(s []uint32, block []byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	al, bl, cl, dl := s[0], s[1], s[2], s[3]
	ar, br, cr, dr := s[0], s[1], s[2], s[3]

	// Left stream
	for i := 0; i < 64; i++ {
		var f uint32
		round := i / 16
		switch round {
		case 0:
			f = ripemdF0(bl, cl, dl)
		case 1:
			f = ripemdF1(bl, cl, dl)
		case 2:
			f = ripemdF2(bl, cl, dl)
		case 3:
			f = ripemdF3(bl, cl, dl)
		}
		t := bits.RotateLeft32(al+f+x[ripemdRL[i]]+ripemdKL[round], ripemdSL[i])
		al = dl
		dl = cl
		cl = bl
		bl = t
	}

	// Right stream
	for i := 0; i < 64; i++ {
		var f uint32
		round := i / 16
		switch round {
		case 0:
			f = ripemdF3(br, cr, dr)
		case 1:
			f = ripemdF2(br, cr, dr)
		case 2:
			f = ripemdF1(br, cr, dr)
		case 3:
			f = ripemdF0(br, cr, dr)
		}
		t := bits.RotateLeft32(ar+f+x[ripemdRR[i]]+ripemdKR128[round], ripemdSR[i])
		ar = dr
		dr = cr
		cr = br
		br = t
	}

	t := s[1] + cl + dr
	s[1] = s[2] + dl + ar
	s[2] = s[3] + al + br
	s[3] = s[0] + bl + cr
	s[0] = t
}

// ripemdCompress160 compresses a single 64-byte block for RIPEMD-160.
func ripemdCompress160(s []uint32, block []byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	al, bl, cl, dl, el := s[0], s[1], s[2], s[3], s[4]
	ar, br, cr, dr, er := s[0], s[1], s[2], s[3], s[4]

	// Left stream
	for i := 0; i < 80; i++ {
		var f uint32
		round := i / 16
		switch round {
		case 0:
			f = ripemdF0(bl, cl, dl)
		case 1:
			f = ripemdF1(bl, cl, dl)
		case 2:
			f = ripemdF2(bl, cl, dl)
		case 3:
			f = ripemdF3(bl, cl, dl)
		case 4:
			f = ripemdF4(bl, cl, dl)
		}
		t := bits.RotateLeft32(al+f+x[ripemdRL[i]]+ripemdKL[round], ripemdSL[i]) + el
		al = el
		el = dl
		dl = bits.RotateLeft32(cl, 10)
		cl = bl
		bl = t
	}

	// Right stream
	for i := 0; i < 80; i++ {
		var f uint32
		round := i / 16
		switch round {
		case 0:
			f = ripemdF4(br, cr, dr)
		case 1:
			f = ripemdF3(br, cr, dr)
		case 2:
			f = ripemdF2(br, cr, dr)
		case 3:
			f = ripemdF1(br, cr, dr)
		case 4:
			f = ripemdF0(br, cr, dr)
		}
		t := bits.RotateLeft32(ar+f+x[ripemdRR[i]]+ripemdKR160[round], ripemdSR[i]) + er
		ar = er
		er = dr
		dr = bits.RotateLeft32(cr, 10)
		cr = br
		br = t
	}

	t := s[1] + cl + dr
	s[1] = s[2] + dl + er
	s[2] = s[3] + el + ar
	s[3] = s[4] + al + br
	s[4] = s[0] + bl + cr
	s[0] = t
}

// ripemdCompress256 compresses a single 64-byte block for RIPEMD-256.
func ripemdCompress256(s []uint32, block []byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	al, bl, cl, dl := s[0], s[1], s[2], s[3]
	ar, br, cr, dr := s[4], s[5], s[6], s[7]

	// Process 4 rounds with chaining swaps between rounds.
	for round := 0; round < 4; round++ {
		// Left stream: 16 steps
		for j := 0; j < 16; j++ {
			i := round*16 + j
			var f uint32
			switch round {
			case 0:
				f = ripemdF0(bl, cl, dl)
			case 1:
				f = ripemdF1(bl, cl, dl)
			case 2:
				f = ripemdF2(bl, cl, dl)
			case 3:
				f = ripemdF3(bl, cl, dl)
			}
			t := bits.RotateLeft32(al+f+x[ripemdRL[i]]+ripemdKL[round], ripemdSL[i])
			al = dl
			dl = cl
			cl = bl
			bl = t
		}

		// Right stream: 16 steps
		for j := 0; j < 16; j++ {
			i := round*16 + j
			var f uint32
			switch round {
			case 0:
				f = ripemdF3(br, cr, dr)
			case 1:
				f = ripemdF2(br, cr, dr)
			case 2:
				f = ripemdF1(br, cr, dr)
			case 3:
				f = ripemdF0(br, cr, dr)
			}
			t := bits.RotateLeft32(ar+f+x[ripemdRR[i]]+ripemdKR128[round], ripemdSR[i])
			ar = dr
			dr = cr
			cr = br
			br = t
		}

		// Chaining swap after each round.
		switch round {
		case 0:
			al, ar = ar, al
		case 1:
			bl, br = br, bl
		case 2:
			cl, cr = cr, cl
		case 3:
			dl, dr = dr, dl
		}
	}

	s[0] += al
	s[1] += bl
	s[2] += cl
	s[3] += dl
	s[4] += ar
	s[5] += br
	s[6] += cr
	s[7] += dr
}

// ripemdCompress320 compresses a single 64-byte block for RIPEMD-320.
func ripemdCompress320(s []uint32, block []byte) {
	var x [16]uint32
	for i := 0; i < 16; i++ {
		x[i] = binary.LittleEndian.Uint32(block[i*4:])
	}

	al, bl, cl, dl, el := s[0], s[1], s[2], s[3], s[4]
	ar, br, cr, dr, er := s[5], s[6], s[7], s[8], s[9]

	// Process 5 rounds with chaining swaps between rounds.
	for round := 0; round < 5; round++ {
		// Left stream: 16 steps
		for j := 0; j < 16; j++ {
			i := round*16 + j
			var f uint32
			switch round {
			case 0:
				f = ripemdF0(bl, cl, dl)
			case 1:
				f = ripemdF1(bl, cl, dl)
			case 2:
				f = ripemdF2(bl, cl, dl)
			case 3:
				f = ripemdF3(bl, cl, dl)
			case 4:
				f = ripemdF4(bl, cl, dl)
			}
			t := bits.RotateLeft32(al+f+x[ripemdRL[i]]+ripemdKL[round], ripemdSL[i]) + el
			al = el
			el = dl
			dl = bits.RotateLeft32(cl, 10)
			cl = bl
			bl = t
		}

		// Right stream: 16 steps
		for j := 0; j < 16; j++ {
			i := round*16 + j
			var f uint32
			switch round {
			case 0:
				f = ripemdF4(br, cr, dr)
			case 1:
				f = ripemdF3(br, cr, dr)
			case 2:
				f = ripemdF2(br, cr, dr)
			case 3:
				f = ripemdF1(br, cr, dr)
			case 4:
				f = ripemdF0(br, cr, dr)
			}
			t := bits.RotateLeft32(ar+f+x[ripemdRR[i]]+ripemdKR160[round], ripemdSR[i]) + er
			ar = er
			er = dr
			dr = bits.RotateLeft32(cr, 10)
			cr = br
			br = t
		}

		// Chaining swap after each round (per RIPEMD-320 spec).
		switch round {
		case 0:
			bl, br = br, bl
		case 1:
			dl, dr = dr, dl
		case 2:
			al, ar = ar, al
		case 3:
			cl, cr = cr, cl
		case 4:
			el, er = er, el
		}
	}

	s[0] += al
	s[1] += bl
	s[2] += cl
	s[3] += dl
	s[4] += el
	s[5] += ar
	s[6] += br
	s[7] += cr
	s[8] += dr
	s[9] += er
}
