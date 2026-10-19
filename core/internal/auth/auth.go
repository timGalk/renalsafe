// Package auth binds locally provisioned bearer credentials to server-side roles.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"os"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/store"
	"strings"
)

type Credential struct {
	ID          string `json:"id"`
	Role        string `json:"role"`
	TokenSHA256 string `json:"token_sha256"`
}
type Auth struct{ credentials []Credential }

func New(cs []Credential) (*Auth, error) {
	if len(cs) == 0 {
		return nil, fmt.Errorf("at least one provisioned credential required")
	}
	ids := map[string]bool{}
	hashes := map[string]bool{}
	for _, c := range cs {
		b, e := hex.DecodeString(c.TokenSHA256)
		if c.ID == "" || ids[c.ID] || hashes[c.TokenSHA256] || e != nil || len(b) != 32 || (c.Role != "physician" && c.Role != "pharmacist" && c.Role != "admin") {
			return nil, fmt.Errorf("invalid or duplicate credential")
		}
		ids[c.ID] = true
		hashes[c.TokenSHA256] = true
	}
	return &Auth{credentials: cs}, nil
}
func Load(path string) (*Auth, error) {
	b, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	var cs []Credential
	if e = domain.Decode(b, &cs); e != nil {
		return nil, e
	}
	return New(cs)
}
func (a *Auth) Authenticate(header string) (store.Actor, bool) {
	token, ok := strings.CutPrefix(header, "Bearer ")
	if !ok || len(token) < 32 || len(token) > 512 {
		return store.Actor{}, false
	}
	hash := sha256.Sum256([]byte(token))
	hexhash := hex.EncodeToString(hash[:])
	var actor store.Actor
	found := 0
	for _, c := range a.credentials {
		match := subtle.ConstantTimeCompare([]byte(hexhash), []byte(c.TokenSHA256))
		if match == 1 {
			actor = store.Actor{ID: c.ID, Role: c.Role}
		}
		found |= match
	}
	return actor, found == 1
}
