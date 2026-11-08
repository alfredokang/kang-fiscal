package http

import (
	"crypto/sha256"
	"encoding/hex"
)

// hash é a impressão digital do corpo, para a idempotência.
func hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
