package ocw

import (
	"crypto/rand"
	"math/big"
)

const nanoidAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ_abcdefghijklmnopqrstuvwxyz-"

// NewRunID generates a nanoid-style unique identifier suitable for use as
// a workflow run ID, docker network name suffix, etc.
func NewRunID(n int) string {
	if n <= 0 {
		n = 12
	}
	alphabetLen := big.NewInt(int64(len(nanoidAlphabet)))
	b := make([]byte, n)
	for i := range b {
		n, err := rand.Int(rand.Reader, alphabetLen)
		if err != nil {
			// Fall back to a simpler random source on failure.
			b[i] = nanoidAlphabet[i%len(nanoidAlphabet)]
			continue
		}
		b[i] = nanoidAlphabet[n.Int64()]
	}
	return string(b)
}
