package anyhash

// GOST R 34.11-94 hash function with GOST 28147-89 block cipher.

import (
	"encoding/binary"
	"fmt"
)

func init() {
	registerHash("gost", func() Hash { return newGOST(&gostTestSbox, "gost") })
	registerHash("gostcrypto", func() Hash { return newGOST(&gostCryptoProSbox, "gost-crypto") })
}

// S-box tables for GOST 28147-89, each row maps 4 bits to 4 bits.
type gostSbox [8][16]byte

var gostTestSbox = gostSbox{
	{4, 10, 9, 2, 13, 8, 0, 14, 6, 11, 1, 12, 7, 15, 5, 3},
	{14, 11, 4, 12, 6, 13, 15, 10, 2, 3, 8, 1, 0, 7, 5, 9},
	{5, 8, 1, 13, 10, 3, 4, 2, 14, 15, 12, 7, 6, 0, 9, 11},
	{7, 13, 10, 1, 0, 8, 9, 15, 14, 4, 6, 12, 11, 2, 5, 3},
	{6, 12, 7, 1, 5, 15, 13, 8, 4, 10, 9, 14, 0, 3, 11, 2},
	{4, 11, 10, 0, 7, 2, 1, 13, 3, 6, 8, 5, 9, 12, 15, 14},
	{13, 11, 4, 1, 3, 15, 5, 9, 0, 10, 14, 7, 6, 8, 2, 12},
	{1, 15, 13, 0, 5, 7, 10, 4, 9, 2, 3, 14, 6, 11, 8, 12},
}

var gostCryptoProSbox = gostSbox{
	{10, 4, 5, 6, 8, 1, 3, 7, 13, 12, 14, 0, 9, 2, 11, 15},
	{5, 15, 4, 0, 2, 13, 11, 9, 1, 7, 6, 3, 12, 14, 10, 8},
	{7, 15, 12, 14, 9, 4, 1, 0, 3, 11, 5, 2, 6, 10, 8, 13},
	{4, 10, 7, 12, 0, 15, 2, 8, 14, 1, 6, 5, 13, 11, 9, 3},
	{7, 6, 4, 11, 9, 12, 2, 10, 1, 8, 0, 14, 15, 13, 3, 5},
	{7, 6, 2, 4, 13, 9, 15, 0, 10, 1, 5, 11, 8, 14, 12, 3},
	{13, 14, 4, 1, 7, 0, 5, 10, 3, 12, 8, 15, 6, 2, 9, 11},
	{1, 3, 10, 9, 5, 11, 4, 15, 8, 6, 7, 14, 13, 0, 2, 12},
}

type gostDigest struct {
	h       [32]byte
	sum     [32]byte
	buf     [32]byte
	bufLen  int
	length  uint64
	sbox    *gostSbox
	phpAlgo string
}

func newGOST(sbox *gostSbox, phpAlgo string) *gostDigest {
	return &gostDigest{sbox: sbox, phpAlgo: phpAlgo}
}

func (d *gostDigest) Size() int      { return 32 }
func (d *gostDigest) BlockSize() int { return 32 }

func (d *gostDigest) Reset() {
	d.h = [32]byte{}
	d.sum = [32]byte{}
	d.buf = [32]byte{}
	d.bufLen = 0
	d.length = 0
}

func (d *gostDigest) Clone() Hash {
	c := *d
	return &c
}

// PHP format: [h_as_8_uint32_LE, sum_as_8_uint32_LE, bitCountLo, bitCountHi, bufLen] + buffer(32)
// Total: 19 ints + 32-byte buffer.
func (d *gostDigest) PHPAlgo() string { return d.phpAlgo }
func (d *gostDigest) MarshalPHP() []any {
	state := make([]any, 0, 20)
	for i := 0; i < 8; i++ {
		state = append(state, int32(binary.LittleEndian.Uint32(d.h[i*4:])))
	}
	for i := 0; i < 8; i++ {
		state = append(state, int32(binary.LittleEndian.Uint32(d.sum[i*4:])))
	}
	bitCount := d.length * 8
	lo, hi := u64toi32pair(bitCount)
	state = append(state, lo, hi)
	state = append(state, int32(d.bufLen))
	buf := make([]byte, 32)
	copy(buf, d.buf[:d.bufLen])
	state = append(state, buf)
	return state
}
func (d *gostDigest) UnmarshalPHP(state []any) error {
	if len(state) < 20 {
		return fmt.Errorf("anyhash: gost PHP state needs 20 elements, got %d", len(state))
	}
	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint32(d.h[i*4:], uint32(phpInt(state, i)))
	}
	for i := 0; i < 8; i++ {
		binary.LittleEndian.PutUint32(d.sum[i*4:], uint32(phpInt(state, 8+i)))
	}
	bitCount := i32pairtou64(phpInt(state, 16), phpInt(state, 17))
	d.length = bitCount / 8
	d.bufLen = int(phpInt(state, 18))
	copy(d.buf[:], phpBuf(state, 19))
	return nil
}

func (d *gostDigest) Write(p []byte) (int, error) {
	n := len(p)
	d.length += uint64(n)

	// Fill existing buffer first.
	if d.bufLen > 0 {
		fill := copy(d.buf[d.bufLen:], p)
		d.bufLen += fill
		p = p[fill:]
		if d.bufLen == 32 {
			d.process(d.buf[:])
			d.bufLen = 0
		}
	}

	// Process full blocks.
	for len(p) >= 32 {
		d.process(p[:32])
		p = p[32:]
	}

	// Buffer remainder.
	if len(p) > 0 {
		d.bufLen = copy(d.buf[:], p)
	}

	return n, nil
}

// process compresses one 32-byte block into the running hash state.
func (d *gostDigest) process(block []byte) {
	// Add block to running sum (256-bit addition, little-endian).
	gostAddBlocks(&d.sum, block)
	// Compress.
	gostCompress(&d.h, block, d.sbox)
}

func (d *gostDigest) Sum(in []byte) []byte {
	// Work on a copy so the caller can continue writing.
	d0 := *d

	// Process remaining partial block if any.
	if d0.bufLen > 0 {
		var padded [32]byte
		copy(padded[:], d0.buf[:d0.bufLen])
		gostAddBlocks(&d0.sum, padded[:])
		gostCompress(&d0.h, padded[:], d0.sbox)
	}

	// Compress length (in bits) as 256-bit little-endian.
	var lenBlock [32]byte
	bits := d0.length * 8
	binary.LittleEndian.PutUint64(lenBlock[0:8], bits)
	gostCompress(&d0.h, lenBlock[:], d0.sbox)

	// Compress the running sum.
	gostCompress(&d0.h, d0.sum[:], d0.sbox)

	return append(in, d0.h[:]...)
}

// gostAddBlocks adds a 32-byte block to a 256-bit accumulator (little-endian).
func gostAddBlocks(acc *[32]byte, block []byte) {
	var carry uint32
	for i := 0; i < 32; i++ {
		carry += uint32(acc[i]) + uint32(block[i])
		acc[i] = byte(carry)
		carry >>= 8
	}
}

// gostEncrypt performs GOST 28147-89 encryption of a single 64-bit block
// with a 256-bit key.
func gostEncrypt(block *[8]byte, key *[32]byte, sbox *gostSbox) {
	n1 := binary.LittleEndian.Uint32(block[0:4])
	n2 := binary.LittleEndian.Uint32(block[4:8])

	// Load key as 8 x 32-bit words (little-endian).
	var k [8]uint32
	for i := 0; i < 8; i++ {
		k[i] = binary.LittleEndian.Uint32(key[i*4 : i*4+4])
	}

	// 24 rounds with keys in order k[0..7] repeated 3 times,
	// then 8 rounds with keys in reverse order k[7..0].
	for i := 0; i < 24; i++ {
		n1, n2 = gostRound(n1, n2, k[i%8], sbox)
	}
	for i := 7; i >= 0; i-- {
		n1, n2 = gostRound(n1, n2, k[i], sbox)
	}

	// Swap halves (no swap after last round — done by swapping output).
	binary.LittleEndian.PutUint32(block[0:4], n2)
	binary.LittleEndian.PutUint32(block[4:8], n1)
}

// gostRound performs one round of GOST 28147-89.
func gostRound(n1, n2, key uint32, sbox *gostSbox) (uint32, uint32) {
	tmp := n1 + key
	// S-box substitution: process 4 bits at a time.
	var result uint32
	for i := 0; i < 8; i++ {
		result |= uint32(sbox[i][(tmp>>(4*uint(i)))&0x0f]) << (4 * uint(i))
	}
	// Rotate left 11 bits.
	result = (result << 11) | (result >> 21)
	// XOR with n2 and swap.
	return n2 ^ result, n1
}

// gostCompress performs the GOST R 34.11-94 compression function: h = f(h, m).
func gostCompress(h *[32]byte, m []byte, sbox *gostSbox) {
	// Step 1: Generate keys.
	// u = h, v = m
	var u, v [32]byte
	copy(u[:], h[:])
	copy(v[:], m)

	// w = u XOR v
	var w [32]byte
	xor32(&w, &u, &v)

	// Generate 4 keys (k1..k4), each 32 bytes.
	var keys [4][32]byte
	gostKeySchedule(&keys[0], &w)

	// For keys 2..4 we need transformations of u and v.
	for i := 1; i < 4; i++ {
		gostTransformA(&u)
		if i == 2 {
			// After step 2, XOR u with C constants.
			xor32Bytes(&u, gostC[:])
		}
		gostTransformA(&v)
		gostTransformA(&v)
		xor32(&w, &u, &v)
		gostKeySchedule(&keys[i], &w)
	}

	// Step 2: Encrypt h using keys.
	// Split h into four 8-byte blocks, encrypt each with corresponding key.
	var s [32]byte
	copy(s[:], h[:])

	for i := 0; i < 4; i++ {
		var block [8]byte
		copy(block[:], s[i*8:i*8+8])
		gostEncrypt(&block, &keys[i], sbox)
		copy(s[i*8:i*8+8], block[:])
	}

	// Step 3: Shuffle.
	gostShuffle(&s, h, m)
	copy(h[:], s[:])
}

// gostKeySchedule applies the P-permutation to w, producing a 256-bit key.
func gostKeySchedule(k *[32]byte, w *[32]byte) {
	for i := 0; i < 32; i++ {
		k[i] = w[gostKeyPerm[i]]
	}
}

// gostKeyPerm is the byte permutation for key schedule.
var gostKeyPerm = [32]byte{
	0, 8, 16, 24,
	1, 9, 17, 25,
	2, 10, 18, 26,
	3, 11, 19, 27,
	4, 12, 20, 28,
	5, 13, 21, 29,
	6, 14, 22, 30,
	7, 15, 23, 31,
}

// gostC is the constant used in key generation step 3.
var gostC = [32]byte{
	0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff,
	0xff, 0x00, 0xff, 0x00, 0xff, 0x00, 0xff, 0x00,
	0x00, 0xff, 0xff, 0x00, 0xff, 0x00, 0x00, 0xff,
	0xff, 0x00, 0x00, 0x00, 0xff, 0xff, 0x00, 0xff,
}

// gostTransformA is the A transformation on a 256-bit value viewed as
// four 64-bit (8-byte) chunks: x = x1 || x2 || x3 || x4.
// A(x) = x2 || x3 || x4 || (x1 ^ x2)
func gostTransformA(x *[32]byte) {
	var x1, x2 [8]byte
	copy(x1[:], x[0:8])
	copy(x2[:], x[8:16])

	// Shift: move bytes 8..31 to 0..23.
	copy(x[0:24], x[8:32])

	// Last 8 bytes = x1 XOR x2.
	for i := 0; i < 8; i++ {
		x[24+i] = x1[i] ^ x2[i]
	}
}

// xor32 computes dst = a XOR b for 32 bytes.
func xor32(dst, a, b *[32]byte) {
	for i := 0; i < 32; i++ {
		dst[i] = a[i] ^ b[i]
	}
}

// xor32Bytes XORs 32 bytes from src into dst.
func xor32Bytes(dst *[32]byte, src []byte) {
	for i := 0; i < 32; i++ {
		dst[i] ^= src[i]
	}
}

// gostShuffle is the final step of the compression function (psi^61 applied).
// s = psi^61(h XOR psi(m XOR psi^12(s)))
func gostShuffle(s *[32]byte, h *[32]byte, m []byte) {
	// Compute psi^12(s).
	var tmp [32]byte
	copy(tmp[:], s[:])
	for i := 0; i < 12; i++ {
		gostPsi(&tmp)
	}

	// XOR with m.
	for i := 0; i < 32; i++ {
		tmp[i] ^= m[i]
	}

	// Apply psi.
	gostPsi(&tmp)

	// XOR with h.
	for i := 0; i < 32; i++ {
		tmp[i] ^= h[i]
	}

	// Apply psi^61.
	for i := 0; i < 61; i++ {
		gostPsi(&tmp)
	}

	copy(s[:], tmp[:])
}

// gostPsi is the psi (linear feedback) transformation on a 256-bit value
// viewed as 16 x 16-bit words.
// psi(Gamma) = Gamma[0] XOR Gamma[1] XOR Gamma[2] XOR Gamma[3] XOR Gamma[12] XOR Gamma[15]
//
//	|| Gamma[0] || Gamma[1] || ... || Gamma[14]
//
// i.e., shift the 16-bit words left by one and put the LFSR output in front.
func gostPsi(data *[32]byte) {
	// Read 16 words of 16 bits each (little-endian).
	var g [16]uint16
	for i := 0; i < 16; i++ {
		g[i] = binary.LittleEndian.Uint16(data[i*2 : i*2+2])
	}

	newWord := g[0] ^ g[1] ^ g[2] ^ g[3] ^ g[12] ^ g[15]

	// Shift all words: g[i] = g[i+1], then g[15] = newWord.
	copy(g[0:15], g[1:16])
	g[15] = newWord

	// Write back.
	for i := 0; i < 16; i++ {
		binary.LittleEndian.PutUint16(data[i*2:i*2+2], g[i])
	}
}
