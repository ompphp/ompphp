package anyhash

import (
	"crypto/sha3"
	"encoding/binary"
	"fmt"
	"hash"
)

func init() {
	registerHash("sha3224", func() Hash { return phpWrapHash("sha3-224", func() hash.Hash { return sha3.New224() }, makeSHA3Codec(144)) })
	registerHash("sha3256", func() Hash { return phpWrapHash("sha3-256", func() hash.Hash { return sha3.New256() }, makeSHA3Codec(136)) })
	registerHash("sha3384", func() Hash { return phpWrapHash("sha3-384", func() hash.Hash { return sha3.New384() }, makeSHA3Codec(104)) })
	registerHash("sha3512", func() Hash { return phpWrapHash("sha3-512", func() hash.Hash { return sha3.New512() }, makeSHA3Codec(72)) })
}

// Go SHA3 binary format: magic("sha\x08", 4) + rate(1) + state(200) + n(2) = 207 bytes
// PHP SHA3 format: [bufPos(int)] + state(200 bytes)
func makeSHA3Codec(rate byte) phpCodec {
	return phpCodec{
		marshal: func(goState []byte) []any {
			// state is at offset 5, 200 bytes
			stateBuf := make([]byte, 200)
			copy(stateBuf, goState[5:205])
			// n is at offset 205, 2 bytes LE
			n := int32(binary.LittleEndian.Uint16(goState[205:207]))
			return []any{n, stateBuf}
		},
		unmarshal: func(state []any) ([]byte, error) {
			if len(state) < 2 {
				return nil, fmt.Errorf("anyhash: sha3 PHP state needs 2 elements, got %d", len(state))
			}
			buf := phpBuf(state, 1)
			if len(buf) < 200 {
				return nil, fmt.Errorf("anyhash: sha3 PHP buffer needs 200 bytes, got %d", len(buf))
			}
			goState := make([]byte, 207)
			copy(goState, "sha\x08")
			goState[4] = rate
			copy(goState[5:205], buf[:200])
			binary.LittleEndian.PutUint16(goState[205:], uint16(phpInt(state, 0)))
			return goState, nil
		},
	}
}
