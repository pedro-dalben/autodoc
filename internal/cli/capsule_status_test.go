package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/agent"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func writeSB(t *testing.T, dir, title string) string {
	t.Helper()
	p := filepath.Join(dir, "storyboard.yml")
	sb := "version: 1\nmeta: {title: \"" + title + "\", language: \"pt-BR\"}\n" +
		"config: {base_url: \"http://localhost:8099\"}\nsetup: {start_url: \"/\"}\n" +
		"scenes:\n  - id: scene-001\n    title: S\n    beats: [{id: b1, sequence: [{speech: {text: \"Oi\"}}]}]\n"
	if err := os.WriteFile(p, []byte(sb), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCapsuleStatusDetectsStoryboardEdit(t *testing.T) {
	d := t.TempDir()
	sbPath := writeSB(t, d, "T")
	sb, err := storyboard.LoadFile(sbPath)
	if err != nil {
		t.Fatal(err)
	}
	c, err := agent.NewCapsule("chat-send", sb, nil)
	if err != nil {
		t.Fatal(err)
	}
	capsDir := filepath.Join(d, "capsules")
	if err := os.MkdirAll(capsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	capsPath, err := agent.SaveCapsule(capsDir, c)
	if err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) string {
		root := NewRoot()
		buf := new(bytes.Buffer)
		root.SetOut(buf)
		root.SetErr(buf)
		root.SetArgs(args)
		if err := root.Execute(); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
		return buf.String()
	}
	old, _ := os.Getwd()
	defer os.Chdir(old)
	if err := os.Chdir(d); err != nil {
		t.Fatal(err)
	}
	if out := run("agent", "capsule-status", "--capsule", capsPath, "--storyboard", sbPath); !strings.Contains(out, "REUSE") {
		t.Fatalf("unchanged storyboard must REUSE, got %q", out)
	}
	writeSB(t, d, "Renamed")
	if out := run("agent", "capsule-status", "--capsule", capsPath, "--storyboard", sbPath); !strings.Contains(out, "STALE") {
		t.Fatalf("edited storyboard must report STALE, got %q", out)
	}
}
