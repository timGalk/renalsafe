package rules_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"renalsafe/core/internal/domain"
	"renalsafe/core/internal/rules"
	"renalsafe/core/internal/testkit"
	"testing"
)

func TestFailClosedValidation(t *testing.T) {
	cases := map[string]func(*domain.Knowledge){"overlap": func(k *domain.Knowledge) { k.Rules[0].Range.UpperInclusive = true; testkit.Sign(&k.Rules[0]) }, "approval": func(k *domain.Knowledge) { k.Rules[0].Approval = nil }, "edited": func(k *domain.Knowledge) { k.Rules[0].Action = "NO_CHANGE" }, "quote": func(k *domain.Knowledge) { k.Rules[0].Citation.Quote = "fabricated" }, "unit": func(k *domain.Knowledge) { k.Rules[0].Unit = "mL/min/1.73m2" }, "range": func(k *domain.Knowledge) { k.Rules[0].Range.Lower = -1 }, "id": func(k *domain.Knowledge) { k.Rules[1].ID = k.Rules[0].ID }, "lifecycle": func(k *domain.Knowledge) { k.Rules[0].Status = "DRAFT" }, "source": func(k *domain.Knowledge) { k.Sources[0].DocumentSHA256 = "bad" }, "path": func(k *domain.Knowledge) { k.Sources[0].TextFile = "../escape" }, "dose": func(k *domain.Knowledge) { k.Rules[0].DoseText = "unsupported dose" }, "context": func(k *domain.Knowledge) { k.Rules[0].Context.Route = "" }}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			k, root := testkit.Fixture(t)
			mutate(&k)
			b, _ := json.Marshal(k)
			if _, e := rules.Load(b, root, true); e == nil {
				t.Fatal("unsafe KB accepted")
			}
		})
	}
}
func TestSyntheticProhibited(t *testing.T) {
	k, root := testkit.Fixture(t)
	b, _ := json.Marshal(k)
	if _, e := rules.Load(b, root, false); e == nil {
		t.Fatal("synthetic accepted")
	}
}
func TestSourceTamper(t *testing.T) {
	k, root := testkit.Fixture(t)
	_ = os.WriteFile(filepath.Join(root, "source.txt"), []byte("changed"), 0600)
	b, _ := json.Marshal(k)
	if _, e := rules.Load(b, root, true); e == nil {
		t.Fatal("tamper accepted")
	}
}
func TestStrictJSON(t *testing.T) {
	k, root := testkit.Fixture(t)
	b, _ := json.Marshal(k)
	for _, data := range [][]byte{append(b, []byte(" {}")...), []byte(`{"version":"one","version":"two","sources":[],"rules":[]}`), []byte(`{"version":"v","sources":[],"rules":[],"unknown":true}`)} {
		if _, e := rules.Load(data, root, true); e == nil {
			t.Fatal("ambiguous JSON accepted")
		}
	}
}
func TestSourceSymlinkEscape(t *testing.T) {
	k, root := testkit.Fixture(t)
	outside := filepath.Join(t.TempDir(), "source.txt")
	b, _ := os.ReadFile(filepath.Join(root, "source.txt"))
	_ = os.WriteFile(outside, b, 0600)
	if e := os.Symlink(outside, filepath.Join(root, "escape")); e != nil {
		t.Fatal(e)
	}
	k.Sources[0].TextFile = "escape"
	data, _ := json.Marshal(k)
	if _, e := rules.Load(data, root, true); e == nil {
		t.Fatal("escaped source accepted")
	}
}

func TestThresholdGrounding(t *testing.T) {
	k, root := testkit.Fixture(t)
	k.Rules[0].Range.Upper = testkit.Float(29)
	testkit.Sign(&k.Rules[0])
	b, _ := json.Marshal(k)
	if _, e := rules.Load(b, root, true); e == nil {
		t.Fatal("unsupported numeric threshold")
	}
}

func TestApprovalBoundToSourceVersion(t *testing.T) {
	k, root := testkit.Fixture(t)
	k.Sources[0].Version = "replaced-document"
	b, _ := json.Marshal(k)
	if _, e := rules.Load(b, root, true); e == nil {
		t.Fatal("approval survived replacement of source")
	}
}

func TestUnmodeledConditionsCannotBeApproved(t *testing.T) {
	k, root := testkit.Fixture(t)
	k.Rules[0].Conditions = &domain.RuleConditions{Unmodeled: []string{"SYNTHETIC condition not supported"}}
	testkit.Sign(&k.Rules[0])
	b, _ := json.Marshal(k)
	if _, e := rules.Load(b, root, true); e == nil {
		t.Fatal("unmodeled conditions approved")
	}
}
