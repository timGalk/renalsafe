package audit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"renalsafe/core/internal/audit"
	"renalsafe/core/internal/decision"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/testkit"
	"testing"
)

func TestChainReplayAndCorruption(t *testing.T) {
	k, root := testkit.Fixture(t)
	s := testkit.Load(t, k, root)
	a := decision.Assess(testkit.Request(), s)
	p := filepath.Join(t.TempDir(), "audit.jsonl")
	one, e := audit.Append(p, a, k)
	if e != nil {
		t.Fatal(e)
	}
	two, e := audit.Append(p, a, k)
	if e != nil {
		t.Fatal(e)
	}
	if two.Previous != one.Hash || two.Sequence != 2 {
		t.Fatal(two)
	}
	b, _ := os.ReadFile(p)
	es, e := audit.Verify(b)
	if e != nil || len(es) != 2 {
		t.Fatal(e)
	}
	replay := decision.Assess(es[0].Assessment.Request, testkit.Load(t, es[0].Knowledge, root))
	if domain.Digest(replay) != domain.Digest(a) {
		t.Fatal("cannot reproduce")
	}
	es[0].Assessment.Results[0].Action = "AVOID"
	tampered, _ := json.Marshal(es[0])
	tampered = append(tampered, '\n')
	if _, e = audit.Verify(tampered); e == nil {
		t.Fatal("tamper accepted")
	}
	if _, e = audit.Verify(b[:len(b)-1]); e == nil {
		t.Fatal("partial tail accepted")
	}
	if _, e = audit.Verify(append(b, []byte("\n")...)); e == nil {
		t.Fatal("blank event accepted")
	}
}
func TestLockAndPersistenceFailure(t *testing.T) {
	k, root := testkit.Fixture(t)
	a := decision.Assess(testkit.Request(), testkit.Load(t, k, root))
	p := filepath.Join(t.TempDir(), "a")
	_ = os.WriteFile(p+".lock", nil, 0600)
	if _, e := audit.Append(p, a, k); e == nil {
		t.Fatal("concurrent append accepted")
	}
	if _, e := audit.Append(filepath.Join(t.TempDir(), "missing", "a"), a, k); e == nil {
		t.Fatal("unpersisted result")
	}
	if _, e := audit.Append(filepath.Join(t.TempDir(), "a"), a, domain.Knowledge{}); e == nil {
		t.Fatal("mismatched knowledge accepted")
	}
}
func TestCorruptLogNeverExtended(t *testing.T) {
	k, root := testkit.Fixture(t)
	a := decision.Assess(testkit.Request(), testkit.Load(t, k, root))
	p := filepath.Join(t.TempDir(), "a")
	_ = os.WriteFile(p, []byte("broken\n"), 0600)
	if _, e := audit.Append(p, a, k); e == nil {
		t.Fatal("corrupt log extended")
	}
	b, _ := os.ReadFile(p)
	if string(b) != "broken\n" {
		t.Fatal("log modified")
	}
}
