package anyhash

// Whirlpool hash algorithm (version 3.0, the final version).
//
// Whirlpool produces a 512-bit (64-byte) digest using 64-byte blocks.
// It uses a modified AES-like structure with 10 rounds operating on an
// 8x8 byte state matrix, and applies Miyaguchi-Preneel construction.
//
// Reference: "The WHIRLPOOL Hashing Function" by Paulo S.L.M. Barreto
// and Vincent Rijmen.

import (
	"encoding/binary"
	"fmt"
	"math/bits"
)

const (
	whirlpoolBlockSize = 64
	whirlpoolSize      = 64
	whirlpoolRounds    = 10
)

func init() {
	registerHash("whirlpool", func() Hash { return newWhirlpool() })
}

type whirlpoolDigest struct {
	state  [8]uint64 // hash state
	buffer [64]byte  // partial block buffer
	bufLen int       // bytes buffered
	bitLen [4]uint64 // 256-bit message length in bits (big-endian words)
}

func newWhirlpool() *whirlpoolDigest {
	return &whirlpoolDigest{}
}

func (d *whirlpoolDigest) Size() int      { return whirlpoolSize }
func (d *whirlpoolDigest) BlockSize() int { return whirlpoolBlockSize }

func (d *whirlpoolDigest) Reset() {
	*d = whirlpoolDigest{}
}

func (d *whirlpoolDigest) Clone() Hash {
	c := *d
	return &c
}

// PHP format: [state0_hi,state0_lo,...,state7_hi,state7_lo, bitLengthBuf(32 bytes), bufferBits, bufferPos] + buffer(64)
// 16 ints for state + 32 bytes for bitLength + 2 ints + 64 bytes buffer.
// The bitLength and buffer are encoded as byte strings.
func (d *whirlpoolDigest) PHPAlgo() string { return "whirlpool" }
func (d *whirlpoolDigest) MarshalPHP() []any {
	result := make([]any, 0, 21)
	// State: each uint64 as (hi, lo) pair
	for i := 0; i < 8; i++ {
		result = append(result, int32(uint32(d.state[i]>>32))) // hi
		result = append(result, int32(uint32(d.state[i])))     // lo
	}
	result = append(result, int32(d.bufLen*8)) // bufferBits
	result = append(result, int32(d.bufLen))   // bufferPos
	// bitLength as 32 bytes
	bitLenBuf := make([]byte, 32)
	for i := 0; i < 4; i++ {
		binary.BigEndian.PutUint64(bitLenBuf[i*8:], d.bitLen[i])
	}
	result = append(result, bitLenBuf)
	// buffer as 64 bytes
	bufBytes := make([]byte, 64)
	copy(bufBytes, d.buffer[:])
	result = append(result, bufBytes)
	return result
}
func (d *whirlpoolDigest) UnmarshalPHP(state []any) error {
	if len(state) < 20 {
		return fmt.Errorf("anyhash: whirlpool PHP state needs 20 elements, got %d", len(state))
	}
	for i := 0; i < 8; i++ {
		hi := uint64(uint32(phpInt(state, i*2)))
		lo := uint64(uint32(phpInt(state, i*2+1)))
		d.state[i] = (hi << 32) | lo
	}
	d.bufLen = int(phpInt(state, 17)) // bufferPos
	bitLenBuf := phpBuf(state, 18)
	if len(bitLenBuf) >= 32 {
		for i := 0; i < 4; i++ {
			d.bitLen[i] = binary.BigEndian.Uint64(bitLenBuf[i*8:])
		}
	}
	bufBytes := phpBuf(state, 19)
	if len(bufBytes) > 0 {
		copy(d.buffer[:], bufBytes)
	}
	return nil
}

func (d *whirlpoolDigest) Write(p []byte) (int, error) {
	n := len(p)

	// Update 256-bit bit counter
	bitCount := uint64(n) << 3
	d.bitLen[3] += bitCount
	if d.bitLen[3] < bitCount {
		d.bitLen[2]++
		if d.bitLen[2] == 0 {
			d.bitLen[1]++
			if d.bitLen[1] == 0 {
				d.bitLen[0]++
			}
		}
	}
	// carry from byte-level overflow of shift
	carry := uint64(n) >> 61
	if carry > 0 {
		d.bitLen[2] += carry
		if d.bitLen[2] < carry {
			d.bitLen[1]++
			if d.bitLen[1] == 0 {
				d.bitLen[0]++
			}
		}
	}

	// Fill buffer
	if d.bufLen > 0 {
		fill := copy(d.buffer[d.bufLen:], p)
		d.bufLen += fill
		p = p[fill:]
		if d.bufLen == whirlpoolBlockSize {
			whirlpoolProcessBlock(&d.state, d.buffer[:])
			d.bufLen = 0
		}
	}

	// Process full blocks
	for len(p) >= whirlpoolBlockSize {
		whirlpoolProcessBlock(&d.state, p[:whirlpoolBlockSize])
		p = p[whirlpoolBlockSize:]
	}

	// Buffer remaining
	if len(p) > 0 {
		d.bufLen = copy(d.buffer[:], p)
	}

	return n, nil
}

func (d *whirlpoolDigest) Sum(in []byte) []byte {
	// Work on a copy so caller can continue writing.
	d0 := *d

	// Padding: append 0x80, then zeros, then 256-bit length in bits.
	// The length field occupies the last 32 bytes of the final block.
	d0.buffer[d0.bufLen] = 0x80
	d0.bufLen++

	// If not enough room for the 32-byte length, pad this block and process.
	if d0.bufLen > whirlpoolBlockSize-32 {
		for i := d0.bufLen; i < whirlpoolBlockSize; i++ {
			d0.buffer[i] = 0
		}
		whirlpoolProcessBlock(&d0.state, d0.buffer[:])
		d0.bufLen = 0
	}

	// Pad with zeros up to the length field position.
	for i := d0.bufLen; i < whirlpoolBlockSize-32; i++ {
		d0.buffer[i] = 0
	}

	// Append the 256-bit message length in big-endian.
	binary.BigEndian.PutUint64(d0.buffer[32:], d0.bitLen[0])
	binary.BigEndian.PutUint64(d0.buffer[40:], d0.bitLen[1])
	binary.BigEndian.PutUint64(d0.buffer[48:], d0.bitLen[2])
	binary.BigEndian.PutUint64(d0.buffer[56:], d0.bitLen[3])
	whirlpoolProcessBlock(&d0.state, d0.buffer[:])

	// Serialize state to bytes.
	var digest [whirlpoolSize]byte
	for i := 0; i < 8; i++ {
		binary.BigEndian.PutUint64(digest[i*8:], d0.state[i])
	}
	return append(in, digest[:]...)
}

// whirlpoolProcessBlock implements the Miyaguchi-Preneel compression:
//
//	state = E(state, block) XOR state XOR block
//
// where E is the Whirlpool block cipher (W).
func whirlpoolProcessBlock(state *[8]uint64, block []byte) {
	var blk [8]uint64
	for i := 0; i < 8; i++ {
		blk[i] = binary.BigEndian.Uint64(block[i*8:])
	}

	// K = state (the key schedule starts from the current hash state)
	var K [8]uint64
	copy(K[:], state[:])

	// plaintext XOR key (initial AddRoundKey)
	var s [8]uint64
	for i := 0; i < 8; i++ {
		s[i] = blk[i] ^ K[i]
	}

	// 10 rounds
	var L [8]uint64
	for r := 0; r < whirlpoolRounds; r++ {
		// Compute round key: apply round function to K
		for i := 0; i < 8; i++ {
			L[i] = wpC0[byte(K[i]>>56)] ^
				wpC1[byte(K[(i+7)%8]>>48)] ^
				wpC2[byte(K[(i+6)%8]>>40)] ^
				wpC3[byte(K[(i+5)%8]>>32)] ^
				wpC4[byte(K[(i+4)%8]>>24)] ^
				wpC5[byte(K[(i+3)%8]>>16)] ^
				wpC6[byte(K[(i+2)%8]>>8)] ^
				wpC7[byte(K[(i+1)%8])]
		}
		L[0] ^= wpRC[r]
		copy(K[:], L[:])

		// Apply round function to state
		for i := 0; i < 8; i++ {
			L[i] = wpC0[byte(s[i]>>56)] ^
				wpC1[byte(s[(i+7)%8]>>48)] ^
				wpC2[byte(s[(i+6)%8]>>40)] ^
				wpC3[byte(s[(i+5)%8]>>32)] ^
				wpC4[byte(s[(i+4)%8]>>24)] ^
				wpC5[byte(s[(i+3)%8]>>16)] ^
				wpC6[byte(s[(i+2)%8]>>8)] ^
				wpC7[byte(s[(i+1)%8])] ^
				K[i]
		}
		copy(s[:], L[:])
	}

	// Miyaguchi-Preneel: new state = E(state, block) XOR state XOR block
	for i := 0; i < 8; i++ {
		state[i] ^= s[i] ^ blk[i]
	}
}

// Whirlpool substitution box (version 3.0).
var wpSbox = [256]byte{
	0x18, 0x23, 0xc6, 0xe8, 0x87, 0xb8, 0x01, 0x4f,
	0x36, 0xa6, 0xd2, 0xf5, 0x79, 0x6f, 0x91, 0x52,
	0x60, 0xbc, 0x9b, 0x8e, 0xa3, 0x0c, 0x7b, 0x35,
	0x1d, 0xe0, 0xd7, 0xc2, 0x2e, 0x4b, 0xfe, 0x57,
	0x15, 0x77, 0x37, 0xe5, 0x9f, 0xf0, 0x4a, 0xda,
	0x58, 0xc9, 0x29, 0x0a, 0xb1, 0xa0, 0x6b, 0x85,
	0xbd, 0x5d, 0x10, 0xf4, 0xcb, 0x3e, 0x05, 0x67,
	0xe4, 0x27, 0x41, 0x8b, 0xa7, 0x7d, 0x95, 0xd8,
	0xfb, 0xee, 0x7c, 0x66, 0xdd, 0x17, 0x47, 0x9e,
	0xca, 0x2d, 0xbf, 0x07, 0xad, 0x5a, 0x83, 0x33,
	0x63, 0x02, 0xaa, 0x71, 0xc8, 0x19, 0x49, 0xd9,
	0xf2, 0xe3, 0x5b, 0x88, 0x9a, 0x26, 0x32, 0xb0,
	0xe9, 0x0f, 0xd5, 0x80, 0xbe, 0xcd, 0x34, 0x48,
	0xff, 0x7a, 0x90, 0x5f, 0x20, 0x68, 0x1a, 0xae,
	0xb4, 0x54, 0x93, 0x22, 0x64, 0xf1, 0x73, 0x12,
	0x40, 0x08, 0xc3, 0xec, 0xdb, 0xa1, 0x8d, 0x3d,
	0x97, 0x00, 0xcf, 0x2b, 0x76, 0x82, 0xd6, 0x1b,
	0xb5, 0xaf, 0x6a, 0x50, 0x45, 0xf3, 0x30, 0xef,
	0x3f, 0x55, 0xa2, 0xea, 0x65, 0xba, 0x2f, 0xc0,
	0xde, 0x1c, 0xfd, 0x4d, 0x92, 0x75, 0x06, 0x8a,
	0xb2, 0xe6, 0x0e, 0x1f, 0x62, 0xd4, 0xa8, 0x96,
	0xf9, 0xc5, 0x25, 0x59, 0x84, 0x72, 0x39, 0x4c,
	0x5e, 0x78, 0x38, 0x8c, 0xd1, 0xa5, 0xe2, 0x61,
	0xb3, 0x21, 0x9c, 0x1e, 0x43, 0xc7, 0xfc, 0x04,
	0x51, 0x99, 0x6d, 0x0d, 0xfa, 0xdf, 0x7e, 0x24,
	0x3b, 0xab, 0xce, 0x11, 0x8f, 0x4e, 0xb7, 0xeb,
	0x3c, 0x81, 0x94, 0xf7, 0xb9, 0x13, 0x2c, 0xd3,
	0xe7, 0x6e, 0xc4, 0x03, 0x56, 0x44, 0x7f, 0xa9,
	0x2a, 0xbb, 0xc1, 0x53, 0xdc, 0x0b, 0x9d, 0x6c,
	0x31, 0x74, 0xf6, 0x46, 0xac, 0x89, 0x14, 0xe1,
	0x16, 0x3a, 0x69, 0x09, 0x70, 0xb6, 0xd0, 0xed,
	0xcc, 0x42, 0x98, 0xa4, 0x28, 0x5c, 0xf8, 0x86,
}

// wpMul multiplies two elements in GF(2^8) with the Whirlpool reduction
// polynomial x^8 + x^4 + x^3 + x^2 + 1 (0x11D).
func wpMul(a, b byte) byte {
	var result byte
	aa := a
	bb := b
	for bb != 0 {
		if bb&1 != 0 {
			result ^= aa
		}
		if aa&0x80 != 0 {
			aa = (aa << 1) ^ 0x1d
		} else {
			aa <<= 1
		}
		bb >>= 1
	}
	return result
}

// MixRows matrix for Whirlpool (circulant matrix).
var wpMDS = [8]byte{0x01, 0x01, 0x04, 0x01, 0x08, 0x05, 0x02, 0x09}

// init the C-tables and round constants at startup.
var (
	wpC0 [256]uint64
	wpC1 [256]uint64
	wpC2 [256]uint64
	wpC3 [256]uint64
	wpC4 [256]uint64
	wpC5 [256]uint64
	wpC6 [256]uint64
	wpC7 [256]uint64
	wpRC [whirlpoolRounds]uint64
)

func init() {
	// Build C0 table: for each input byte x, apply SubBytes then MixRows.
	// C0[x] packs 8 bytes where byte j = MDS[j] * S[x] in GF(2^8).
	for x := 0; x < 256; x++ {
		s := wpSbox[x]
		var v uint64
		for j := 0; j < 8; j++ {
			v |= uint64(wpMul(wpMDS[j], s)) << uint(56-8*j)
		}
		wpC0[x] = v
	}

	// C1..C7 are rotations of C0 by 8, 16, ..., 56 bits.
	for x := 0; x < 256; x++ {
		wpC1[x] = bits.RotateLeft64(wpC0[x], -8)
		wpC2[x] = bits.RotateLeft64(wpC0[x], -16)
		wpC3[x] = bits.RotateLeft64(wpC0[x], -24)
		wpC4[x] = bits.RotateLeft64(wpC0[x], -32)
		wpC5[x] = bits.RotateLeft64(wpC0[x], -40)
		wpC6[x] = bits.RotateLeft64(wpC0[x], -48)
		wpC7[x] = bits.RotateLeft64(wpC0[x], -56)
	}

	// Round constants: pack 8 consecutive S-box entries as bytes into
	// a uint64. RC[r] = S[8r]||S[8r+1]||...||S[8r+7].
	for r := 0; r < whirlpoolRounds; r++ {
		var rc uint64
		for j := 0; j < 8; j++ {
			rc |= uint64(wpSbox[8*r+j]) << uint(56-8*j)
		}
		wpRC[r] = rc
	}
}
