package utils

import (
	"crypto/sha1"

	"github.com/tv42/zbase32"
)

func HashString(s string) string {
	hash := sha1.Sum([]byte(s))
	return zbase32.EncodeToString(hash[:])[:5]
}
