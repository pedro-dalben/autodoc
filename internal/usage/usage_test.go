package usage

import (
	"strings"
	"testing"
)

func TestReportSeparatesEstimateFromProviderUsage(t *testing.T) {
	d := t.TempDir()
	if err := Log(d, Entry{Phase: PhaseBrowser, Bytes: 400}, Entry{Phase: PhaseSkill, Bytes: 100, Source: "agent", EstTok: 42, Estimate: false}); err != nil {
		t.Fatal(err)
	}
	totals, err := Report(d)
	if err != nil {
		t.Fatal(err)
	}
	out := Format(totals)
	if !strings.Contains(out, "estimated") || !strings.Contains(out, "browser") {
		t.Fatalf("bad report: %s", out)
	}
	var provider bool
	for _, total := range totals {
		if total.Phase == PhaseSkill {
			provider = total.ProviderOK
		}
	}
	if !provider {
		t.Fatal("provider usage flag lost")
	}
}
