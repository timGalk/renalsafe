package password

import "testing"

func TestPasswordHashing(t *testing.T) {
	const value = "synthetic-long-password"
	a, e := Hash(value)
	if e != nil {
		t.Fatal(e)
	}
	b, e := Hash(value)
	if e != nil {
		t.Fatal(e)
	}
	if a == b || !Verify(a, value) || Verify(a, "wrong") || Verify("bad", value) {
		t.Fatal("password validation or salt isolation failed")
	}
	if _, e = Hash("short"); e == nil {
		t.Fatal("short password accepted")
	}
}
