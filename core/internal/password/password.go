// Package password hashes staff passwords; random API keys use separate token hashing.
package password

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"golang.org/x/crypto/argon2"
	"strings"
)

func Hash(value string) (string, error) {
	if len(value) < 15 || len(value) > 256 {
		return "", fmt.Errorf("password must contain 15–256 bytes")
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(value), salt, 2, 19*1024, 1, 32)
	return "argon2id$" + base64.RawStdEncoding.EncodeToString(salt) + "$" + base64.RawStdEncoding.EncodeToString(key), nil
}
func Verify(hash, value string) bool {
	if len(value) > 256 {
		return false
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false
	}
	salt, e := base64.RawStdEncoding.DecodeString(parts[1])
	if e != nil || len(salt) != 16 {
		return false
	}
	want, e := base64.RawStdEncoding.DecodeString(parts[2])
	if e != nil || len(want) != 32 {
		return false
	}
	got := argon2.IDKey([]byte(value), salt, 2, 19*1024, 1, 32)
	return subtle.ConstantTimeCompare(got, want) == 1
}
