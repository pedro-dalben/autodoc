package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/config"
)

func writeTOML(t *testing.T, path, model string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	c := config.Default()
	c.TTS.Model = model
	if err := c.Save(path); err != nil {
		t.Fatal(err)
	}
}

// isolatedBase returns a fresh directory tree with no autodoc.toml in any
// ancestor the test can reach: t.TempDir() itself may sit under a dirty
// ancestor (e.g. /tmp/autodoc.toml left by manual runs), and FindConfig walks
// upward, so tests must not anchor directly at t.TempDir().
func isolatedBase(t *testing.T) string {
	t.Helper()
	base := filepath.Join(t.TempDir(), "iso")
	if err := os.MkdirAll(base, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(base, "autodoc.toml")); err == nil {
		t.Fatalf("test isolation broken: %s exists", filepath.Join(base, "autodoc.toml"))
	}
	return base
}

func TestFindConfigPrefersProject(t *testing.T) {
	base := isolatedBase(t)
	root := filepath.Join(base, "root")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTOML(t, filepath.Join(root, "autodoc.toml"), "project-model")
	t.Setenv("AUTODOC_CONFIG_HOME", t.TempDir())
	global, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	gc := config.Default()
	gc.TTS.Model = "global-model"
	if err := gc.Save(global); err != nil {
		t.Fatal(err)
	}
	lc, err := config.FindConfig(root)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Source != "project" {
		t.Fatalf("expected project, got %q (%s)", lc.Source, lc.Path)
	}
	if lc.Config.TTS.Model != "project-model" {
		t.Fatalf("project config not used: %q", lc.Config.TTS.Model)
	}
	if lc.Root != root {
		t.Fatalf("root should be project dir, got %q", lc.Root)
	}
}

func TestFindConfigFallsBackToGlobal(t *testing.T) {
	base := isolatedBase(t)
	home := t.TempDir()
	t.Setenv("AUTODOC_CONFIG_HOME", home)
	global, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	gc := config.Default()
	gc.TTS.Model = "global-model"
	if err := gc.Save(global); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(base, "proj", "sub")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	lc, err := config.FindConfig(work)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Source != "global" {
		t.Fatalf("expected global, got %q (%s)", lc.Source, lc.Path)
	}
	if lc.Config.TTS.Model != "global-model" {
		t.Fatalf("global config not used: %q", lc.Config.TTS.Model)
	}
}

func TestFindConfigDefaultWhenNothingExists(t *testing.T) {
	base := isolatedBase(t)
	t.Setenv("AUTODOC_CONFIG_HOME", filepath.Join(t.TempDir(), "does-not-exist"))
	work := filepath.Join(base, "proj", "sub")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	lc, err := config.FindConfig(work)
	if err != nil {
		t.Fatal(err)
	}
	if lc.Source != "default" {
		t.Fatalf("expected default, got %q (%s)", lc.Source, lc.Path)
	}
}

func TestFindConfigRejectsCorruptGlobal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AUTODOC_CONFIG_HOME", home)
	global, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(global), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(global, []byte("tts: [broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	work := filepath.Join(isolatedBase(t), "proj", "sub")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := config.FindConfig(work); err == nil {
		t.Fatal("expected error for corrupt global TOML")
	}
}

func TestGlobalPathRespectsEnv(t *testing.T) {
	home := t.TempDir()
	t.Setenv("AUTODOC_CONFIG_HOME", home)
	gp, err := config.GlobalPath()
	if err != nil {
		t.Fatal(err)
	}
	if gp != filepath.Join(home, "autodoc.toml") {
		t.Fatalf("unexpected global path %q", gp)
	}
}
