package anyhash

// HMAC implementation using anyhash.Hash so that Clone works natively.

type hmacHash struct {
	inner    Hash // inner hash: H(ikey || message...)
	outer    Hash // outer hash template: H(okey || ...) — frozen after key setup
	iKeyHash Hash // inner hash template after ikey — for Reset
}

func newHMAC(fn newFunc, key []byte) *hmacHash {
	h := fn()
	blockSize := h.BlockSize()

	// If the key is longer than the block size, hash it.
	if len(key) > blockSize {
		h.Write(key)
		key = h.Sum(nil)
		h.Reset()
	}

	// Pad key to block size.
	padded := make([]byte, blockSize)
	copy(padded, key)

	// Compute ikey and okey.
	ikey := make([]byte, blockSize)
	okey := make([]byte, blockSize)
	for i := range padded {
		ikey[i] = padded[i] ^ 0x36
		okey[i] = padded[i] ^ 0x5c
	}

	inner := fn()
	inner.Write(ikey)

	outer := fn()
	outer.Write(okey)

	return &hmacHash{
		inner:    inner.Clone(),
		outer:    outer,
		iKeyHash: inner,
	}
}

func (h *hmacHash) Write(p []byte) (int, error) {
	return h.inner.Write(p)
}

func (h *hmacHash) Sum(in []byte) []byte {
	// inner sum without disturbing state
	innerSum := h.inner.Clone().Sum(nil)

	// outer = H(okey || innerSum)
	outer := h.outer.Clone()
	outer.Write(innerSum)
	return outer.Sum(in)
}

func (h *hmacHash) Reset() {
	h.inner = h.iKeyHash.Clone()
}

func (h *hmacHash) Size() int      { return h.outer.Size() }
func (h *hmacHash) BlockSize() int { return h.outer.BlockSize() }

func (h *hmacHash) Clone() Hash {
	return &hmacHash{
		inner:    h.inner.Clone(),
		outer:    h.outer.Clone(),
		iKeyHash: h.iKeyHash.Clone(),
	}
}
