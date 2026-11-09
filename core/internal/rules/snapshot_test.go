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

func TestPolishDecimalGrounding(t *testing.T) {
	// Fictional numbers exercise syntax, not a real medicine's dosing rule.
	quote := "Software fixture: CrCl < 1,5; dose 2,5 mg every 12 hours."
	for _, tc := range []struct {
		name            string
		threshold, dose float64
		instructions    string
		valid           bool
	}{
		{"comma decimals", 1.5, 2.5, "2,5 mg every 12 hours", true},
		{"normalized decimals", 1.5, 2.5, "2.5 mg every 12 hours", true},
		{"fraction is not integer threshold", 5, 12, "12 mg every 12 hours", false},
		{"fraction is not integer dose", 12, 5, "5 mg every 12 hours", false},
		{"decimal prefix is not integer instruction", 12, 12, "2 mg every 12 hours", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, _ := testkit.Fixture(t)
			k.Rules = k.Rules[:1]
			r := &k.Rules[0]
			r.Range.Upper = testkit.Float(tc.threshold)
			r.Action = "REDUCE_DOSE"
			r.DoseText = tc.instructions
			r.RegimenLimit = &domain.RegimenLimit{MaxSingleDose: domain.Quantity{Value: tc.dose, Unit: "mg"}, MinIntervalHours: 12}
			r.Citation.Quote = quote
			hash := domain.BytesDigest([]byte(quote))
			k.Sources[0].DocumentSHA256, k.Sources[0].TextSHA256 = hash, hash
			r.Citation.DocumentSHA256 = hash
			testkit.Sign(r)
			b, _ := json.Marshal(k)
			_, err := rules.LoadWithFiles(b, map[string][]byte{"source.txt": []byte(quote)}, true)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v, error=%v", tc.valid, err)
			}
		})
	}
}
