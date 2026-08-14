# anyhash

[![GoDoc](https://pkg.go.dev/badge/github.com/KarpelesLab/anyhash)](https://pkg.go.dev/github.com/KarpelesLab/anyhash)
[![Build Status](https://github.com/KarpelesLab/anyhash/actions/workflows/test.yml/badge.svg)](https://github.com/KarpelesLab/anyhash/actions/workflows/test.yml)
[![Coverage Status](https://coveralls.io/repos/github/KarpelesLab/anyhash/badge.svg?branch=master)](https://coveralls.io/github/KarpelesLab/anyhash?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/KarpelesLab/anyhash)](https://goreportcard.com/report/github.com/KarpelesLab/anyhash)

A Go hashing library that supports 60 hash algorithms selected by name. Unlike Go's standard crypto packages, anyhash accepts algorithm names as strings and provides a `Clone()` method to duplicate hash state at any point during computation.

## Features

- **String-based algorithm selection** — `New("sha256")` instead of importing `crypto/sha256`
- **Name normalization** — case-insensitive, hyphens ignored: `"SHA-256"`, `"sha256"`, `"SHA256"` all work
- **Cloneable state** — `Clone()` creates an independent copy of the hash mid-computation
- **Continue after Sum** — calling `Sum()` does not reset the hash; you can keep writing
- **HMAC support** — `NewHMAC("sha256", key)` with full Clone/continue-after-Sum support
- **HKDF support** — `NewHKDF("sha256", ikm, length, info, salt)` for key derivation (RFC 5869)
- **Zero external dependencies** — all algorithms implemented using only the Go standard library
- **PHP compatible** — algorithm names and output match PHP's `hash()` function

## Installation

```bash
go get github.com/KarpelesLab/anyhash
```

## Usage

```go
package main

import (
    "encoding/hex"
    "fmt"
    "github.com/KarpelesLab/anyhash"
)

func main() {
    // Create a hash by name
    h, err := anyhash.New("sha256")
    if err != nil {
        panic(err)
    }
    h.Write([]byte("hello "))

    // Clone the state
    h2 := h.Clone()

    // Continue writing to both independently
    h.Write([]byte("world"))
    h2.Write([]byte("there"))

    fmt.Println(hex.EncodeToString(h.Sum(nil)))  // sha256("hello world")
    fmt.Println(hex.EncodeToString(h2.Sum(nil))) // sha256("hello there")

    // HMAC
    mac, _ := anyhash.NewHMAC("sha256", []byte("secret"))
    mac.Write([]byte("message"))
    fmt.Println(hex.EncodeToString(mac.Sum(nil)))

    // HKDF (RFC 5869)
    key, _ := anyhash.NewHKDF("sha256", []byte("input key"), 32, []byte("info"), nil)
    fmt.Println(hex.EncodeToString(key))

    // List all supported algorithms
    fmt.Println(anyhash.List())
}
```

## Supported Algorithms

| Category | Algorithms |
|----------|-----------|
| MD | md2, md4, md5 |
| SHA-1/2 | sha1, sha224, sha256, sha384, sha512, sha512/224, sha512/256 |
| SHA-3 | sha3-224, sha3-256, sha3-384, sha3-512 |
| RIPEMD | ripemd128, ripemd160, ripemd256, ripemd320 |
| Tiger | tiger128,3 tiger160,3 tiger192,3 tiger128,4 tiger160,4 tiger192,4 |
| HAVAL | haval{128,160,192,224,256},{3,4,5} (15 variants) |
| Whirlpool | whirlpool |
| GOST | gost, gost-crypto |
| Snefru | snefru, snefru256 |
| Checksums | adler32, crc32, crc32b, crc32c |
| FNV | fnv132, fnv1a32, fnv164, fnv1a64 |
| MurmurHash3 | murmur3a, murmur3c, murmur3f |
| xxHash | xxh32, xxh64, xxh3, xxh128 |
| Other | joaat |

## API

```go
// Hash extends hash.Hash with cloning support.
type Hash interface {
    hash.Hash
    Clone() Hash
}

// New creates a hash by algorithm name.
func New(algo string) (Hash, error)

// NewHMAC creates an HMAC by algorithm name.
func NewHMAC(algo string, key []byte) (Hash, error)

// NewHKDF derives key material using HKDF (RFC 5869).
func NewHKDF(algo string, key []byte, length int, info []byte, salt []byte) ([]byte, error)

// List returns all supported algorithm names, sorted.
func List() []string
```

## License

MIT
