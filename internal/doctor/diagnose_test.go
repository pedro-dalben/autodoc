package doctor

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiagnoseWorkspace(t *testing.T) {
	d := t.TempDir()

	// Initially missing config and storyboard
	diag := Diagnose(d)
	var buf bytes.Buffer
	diag.Print(&buf)
	out := buf.String()

	if !strings.Contains(out, "AutoDoc Self-Diagnosis") {
		t.Errorf("missing header in output: %s", out)
	}
	if !strings.Contains(out, "autodoc.toml — missing") {
		t.Errorf("expected warning about missing autodoc.toml: %s", out)
	}

	// Add autodoc.toml and storyboard.yml
	_ = os.WriteFile(filepath.Join(d, "autodoc.toml"), []byte(""), 0o644)
	_ = os.WriteFile(filepath.Join(d, "storyboard.yml"), []byte(""), 0o644)

	diag2 := Diagnose(d)
	buf.Reset()
	diag2.Print(&buf)
	out2 := buf.String()
	if !strings.Contains(out2, "autodoc.toml — present") {
		t.Errorf("expected autodoc.toml present: %s", out2)
	}
	if !strings.Contains(out2, "storyboard — storyboard.yml") {
		t.Errorf("expected storyboard.yml present: %s", out2)
	}
}
