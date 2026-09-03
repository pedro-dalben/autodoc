package harness_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/harness"
)

func testHome(t *testing.T) string {
	t.Helper()
	h := t.TempDir()
	t.Setenv("AUTODOC_TEST_HOME", h)
	return h
}

func canonicalSkill(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(p, []byte("# AutoDoc skill v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestAllHarnessesInstallIdempotent(t *testing.T) {
	for _, h := range harness.All() {
		home := testHome(t)
		skill := canonicalSkill(t)
		first, err := h.Install(home, skill, "autodoc")
		if err != nil {
			t.Fatalf("%s install: %v", h.Name, err)
		}
		if len(first) == 0 {
			t.Fatalf("%s: expected files on first install", h.Name)
		}
		snap := map[string]string{}
		filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(p)
				snap[p] = string(b)
			}
			return nil
		})
		second, err := h.Install(home, skill, "autodoc")
		if err != nil {
			t.Fatalf("%s reinstall: %v", h.Name, err)
		}
		if len(second) != 0 {
			t.Fatalf("%s: re-run must be ZERO diff, changed %v", h.Name, second)
		}
		filepath.Walk(home, func(p string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				b, _ := os.ReadFile(p)
				if snap[p] != string(b) {
					t.Fatalf("%s: file changed on re-run: %s", h.Name, p)
				}
			}
			return nil
		})
	}
}

func TestUninstallPreservesUserEdits(t *testing.T) {
	home := testHome(t)
	skill := canonicalSkill(t)
	h := harness.ByName("claude")
	if h == nil {
		t.Fatal("claude harness missing")
	}
	mdPath := filepath.Join(home, ".claude", "CLAUDE.md")
	_ = os.MkdirAll(filepath.Dir(mdPath), 0o755)
	_ = os.WriteFile(mdPath, []byte("# My notes\nUser line 1\n"), 0o644)
	if _, err := h.Install(home, skill, "autodoc"); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(content), "User line 1") || !strings.Contains(string(content), "AUTODOC:BEGIN") {
		t.Fatalf("merge lost content: %s", content)
	}
	extra := string(content) + "User line 2 (added after install)\n"
	_ = os.WriteFile(mdPath, []byte(extra), 0o644)
	removed, err := h.Uninstall(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) == 0 {
		t.Fatal("expected removals")
	}
	after, _ := os.ReadFile(mdPath)
	if !strings.Contains(string(after), "User line 1") || !strings.Contains(string(after), "User line 2") {
		t.Fatalf("user edits lost: %s", after)
	}
	if strings.Contains(string(after), "AUTODOC:BEGIN") {
		t.Fatalf("autodoc block not removed: %s", after)
	}
	if _, err := os.Stat(filepath.Join(home, ".claude", "skills", "autodoc", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("skill file should be removed")
	}
}

func TestAntigravityProbe(t *testing.T) {
	home := testHome(t)
	h := harness.ByName("antigravity")
	pr := h.Probe(home)
	if pr.Found || pr.Configured {
		t.Fatalf("unexpected probe: %+v", pr)
	}
	_ = os.MkdirAll(filepath.Join(home, ".config", "antigravity"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".config", "antigravity", "config.json"), []byte(`{}`), 0o644)
	pr = h.Probe(home)
	if !pr.Found || pr.Configured {
		t.Fatalf("expected found-but-unconfigured: %+v", pr)
	}
	skill := canonicalSkill(t)
	if _, err := h.Install(home, skill, "autodoc"); err != nil {
		t.Fatal(err)
	}
	pr = h.Probe(home)
	if !pr.Configured {
		t.Fatalf("expected configured: %+v", pr)
	}
}

func TestAntigravityIDEInstallMarker(t *testing.T) {
	home := testHome(t)
	h := harness.ByName("antigravity")
	_ = os.MkdirAll(filepath.Join(home, ".antigravity"), 0o755)
	_ = os.WriteFile(filepath.Join(home, ".antigravity", "argv.json"), []byte(`{}`), 0o644)
	pr := h.Probe(home)
	if !pr.Found {
		t.Fatalf("IDE install marker should count as found: %+v", pr)
	}
	if pr.Configured {
		t.Fatalf("skill missing so must not be configured: %+v", pr)
	}
}
