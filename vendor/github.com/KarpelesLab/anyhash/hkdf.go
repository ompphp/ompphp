package anyhash

// HKDF implementation per RFC 5869.

import "fmt"

// NewHKDF derives key material using HKDF (RFC 5869) with the named hash
// algorithm. It returns length bytes of output key material.
//
// If salt is nil, a zero-filled salt of HashLen bytes is used per the RFC.
// info may be nil for empty context.
func NewHKDF(algo string, key []byte, length int, info []byte, salt []byte) ([]byte, error) {
	// Validate algo early.
	fn, ok := algos[normalize(algo)]
	if !ok {
		return nil, fmt.Errorf("anyhash: unknown algorithm %q", algo)
	}

	hashLen := fn().Size()

	if length < 0 || length > 255*hashLen {
		return nil, fmt.Errorf("anyhash: HKDF output length %d exceeds maximum %d", length, 255*hashLen)
	}

	// Extract: PRK = HMAC-Hash(salt, IKM)
	if salt == nil {
		salt = make([]byte, hashLen)
	}
	prkHash, _ := NewHMAC(algo, salt)
	prkHash.Write(key)
	prk := prkHash.Sum(nil)

	// Expand: T(i) = HMAC-Hash(PRK, T(i-1) || info || i)
	n := (length + hashLen - 1) / hashLen
	okm := make([]byte, 0, n*hashLen)
	var prev []byte
	for i := 1; i <= n; i++ {
		h, _ := NewHMAC(algo, prk)
		h.Write(prev)
		h.Write(info)
		h.Write([]byte{byte(i)})
		prev = h.Sum(nil)
		okm = append(okm, prev...)
	}

	return okm[:length], nil
}
