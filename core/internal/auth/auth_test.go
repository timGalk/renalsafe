package auth

import (
	"renalsafe/core/internal/domain"
	"strings"
	"testing"
)

func TestRoleIdentityBoundToCredential(t *testing.T) {
	token := strings.Repeat("fixture-token-", 4)
	a, e := New([]Credential{{ID: "fixture-user", Role: "physician", TokenSHA256: domain.BytesDigest([]byte(token))}})
	if e != nil {
		t.Fatal(e)
	}
	who, ok := a.Authenticate("Bearer " + token)
	if !ok || who.ID != "fixture-user" || who.Role != "physician" {
		t.Fatal(who, ok)
	}
	for _, header := range []string{token, "Bearer short", "Bearer " + strings.Repeat("x", 60), "bearer " + token} {
		if _, ok = a.Authenticate(header); ok {
			t.Fatal("invalid auth accepted")
		}
	}
}
func TestInvalidCredentialConfiguration(t *testing.T) {
	hash := strings.Repeat("a", 64)
	for _, cs := range [][]Credential{nil, {{ID: "x", Role: "pharmacist", TokenSHA256: "not-a-hash"}}, {{ID: "x", Role: "root", TokenSHA256: hash}}, {{ID: "x", Role: "physician", TokenSHA256: hash}, {ID: "x", Role: "admin", TokenSHA256: hash}}} {
		if _, e := New(cs); e == nil {
			t.Fatal("invalid configuration accepted")
		}
	}
}
