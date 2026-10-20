// Package uuid generates random UUIDs; the standard library has none.
package uuid

import (
	"crypto/rand"
	"encoding/hex"
)

const (
	versionByte, version4 = 6, 0x40
	variantByte, rfc9562  = 8, 0x80
)

// NewV4 returns a random version-4 UUID in canonical lowercase form.
func NewV4() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[versionByte] = b[versionByte]&0x0f | version4
	b[variantByte] = b[variantByte]&0x3f | rfc9562
	h := hex.EncodeToString(b[:])
	return h[:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
