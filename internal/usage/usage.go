// Package usage records how many agent-visible bytes each phase of an
// AutoDoc session consumed. AutoDoc measures its own outputs exactly;
// observations from outside its boundary (browser snapshots taken by the
// driving agent, skill tokens) are ingested via `autodoc context log` and
// are labeled with their source. Estimates are always marked as estimates —
// they are never presented as provider-reported usage.
package usage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Phases are the context categories tracked for the agent budget.
const (
	PhaseSkill      = "skill"
	PhaseToolCat    = "tool_catalog"
	PhaseRepo       = "repo"
	PhaseBrowser    = "browser"
	PhaseCLI        = "cli"
	PhaseStoryboard = "storyboard"
	PhaseEvidence   = "evidence"
)

// Entry is one logged observation.
type Entry struct {
	TS       string `json:"ts"`
	Phase    string `json:"phase"`
	Bytes    int    `json:"bytes"`
	Source   string `json:"source"` // autodoc | agent
	Label    string `json:"label,omitempty"`
	EstTok   int    `json:"est_tokens,omitempty"` // bytes/4 estimate, never provider usage
	Note     string `json:"note,omitempty"`
	Estimate bool   `json:"estimate,omitempty"`
}

// Log appends entries to the JSONL usage file at dir/context.jsonl.
func Log(dir string, entries ...Entry) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, "context.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range entries {
		if e.TS == "" {
			e.TS = time.Now().UTC().Format(time.RFC3339)
		}
		if e.Source == "" {
			e.Source = "autodoc"
		}
		if e.EstTok == 0 && e.Bytes > 0 {
			e.EstTok = e.Bytes / 4
			e.Estimate = true
		}
		if err := enc.Encode(e); err != nil {
			return err
		}
	}
	return nil
}

// Load returns all entries, oldest first.
func Load(dir string) ([]Entry, error) {
	b, err := os.ReadFile(filepath.Join(dir, "context.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var entries []Entry
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var e Entry
		if json.Unmarshal([]byte(line), &e) == nil {
			entries = append(entries, e)
		}
	}
	return entries, nil
}

// Total aggregates entries for one phase.
type Total struct {
	Phase      string `json:"phase"`
	Bytes      int    `json:"bytes"`
	Entries    int    `json:"entries"`
	EstTokens  int    `json:"est_tokens"`
	ProviderOK bool   `json:"provider_reported"`
}

// Report aggregates entries by phase.
func Report(dir string) ([]Total, error) {
	entries, err := Load(dir)
	if err != nil {
		return nil, err
	}
	byPhase := map[string]*Total{}
	for _, e := range entries {
		t, ok := byPhase[e.Phase]
		if !ok {
			t = &Total{Phase: e.Phase}
			byPhase[e.Phase] = t
		}
		t.Bytes += e.Bytes
		t.Entries++
		t.EstTokens += e.EstTok
		if e.Source == "agent" && !e.Estimate {
			t.ProviderOK = true
		}
	}
	out := make([]Total, 0, len(byPhase))
	for _, t := range byPhase {
		out = append(out, *t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Bytes > out[j].Bytes })
	return out, nil
}

// Format renders the report as a compact agent-facing table.
func Format(totals []Total) string {
	if len(totals) == 0 {
		return "no context usage recorded (autodoc context log --phase browser --bytes N)"
	}
	var b strings.Builder
	b.WriteString("phase          bytes    entries  est_tokens\n")
	for _, t := range totals {
		note := ""
		if !t.ProviderOK {
			note = "~"
		}
		fmt.Fprintf(&b, "%-14s %8d %8d %10d%s\n", t.Phase, t.Bytes, t.Entries, t.EstTokens, note)
	}
	b.WriteString("(~ estimated bytes/4; provider-reported usage when available)\n")
	return b.String()
}

// Reset removes the usage log.
func Reset(dir string) error {
	err := os.Remove(filepath.Join(dir, "context.jsonl"))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}
