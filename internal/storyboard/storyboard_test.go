package storyboard_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func writeTmp(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "storyboard.yml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const validSB = `version: 1
meta:
  title: "T"
  description: "d"
  language: "pt-BR"
  resolution: "1280x720"
  fps: 30
config:
  base_url: "http://localhost:8099"
  viewport_width: 1280
  viewport_height: 720
setup:
  start_url: "/login"
redact:
  mask_password_inputs: true
scenes:
  - id: scene-001
    title: "S1"
    url: "/login"
    beats:
      - id: beat-01
        sequence:
          - speech: {text: "No menu lateral, acesse Materiais."}
          - action: {type: click, target: {role: link, name: Materiais}}
          - wait: {state: visible, target: {role: heading}, timeout_ms: 8000}
          - speech: {text: "Nesta tela encontramos os materiais."}
          - hold: {duration_ms: 600}
`

func TestValidStoryboard(t *testing.T) {
	sb, err := storyboard.LoadFile(writeTmp(t, validSB))
	if err != nil {
		t.Fatalf("expected valid: %v", err)
	}
	if len(sb.Scenes) != 1 || len(sb.Scenes[0].Beats[0].Sequence) != 5 {
		t.Fatalf("unexpected shape: %+v", sb.Scenes)
	}
	if h := sb.SourceHash(); len(h) != 16 {
		t.Fatalf("bad hash %q", h)
	}
}

func TestRejectsPercentageTiming(t *testing.T) {
	bad := `version: 1
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://x"}
setup: {start_url: "/"}
scenes:
  - id: s1
    title: S
    beats:
      - id: b1
        sequence:
          - action: {type: click, target: {css: "div > div:nth-child(3) > button"}}
`
	_, err := storyboard.LoadFile(writeTmp(t, bad))
	if err == nil {
		t.Fatal("expected validation error for css-only fragile locator without speech ordering")
	}
}

func TestRequiresSingleEventPerSequenceItem(t *testing.T) {
	bad := `version: 1
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://x"}
setup: {start_url: "/"}
scenes:
  - id: s1
    title: S
    beats:
      - id: b1
        sequence:
          - speech: {text: "a"}
            action: {type: click, target: {test_id: x}}
`
	if _, err := storyboard.LoadFile(writeTmp(t, bad)); err == nil {
		t.Fatal("expected error for dual speech+action in one sequence item")
	}
}

func TestDuplicateSceneRejected(t *testing.T) {
	bad := `version: 1
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://x"}
setup: {start_url: "/"}
scenes:
  - id: s1
    title: S
    beats: [{id: b1, sequence: [{speech: {text: "a"}}]}]
  - id: s1
    title: S2
    beats: [{id: b1, sequence: [{speech: {text: "b"}}]}]
`
	if _, err := storyboard.LoadFile(writeTmp(t, bad)); err == nil {
		t.Fatal("expected duplicate scene error")
	}
}

func TestLocatorPreference(t *testing.T) {
	tg := &storyboard.Target{Role: "link", Name: "Materiais"}
	if got := tg.PlaywrightSelector(); got != `role=link[name="Materiais"]` {
		t.Fatalf("bad selector %q", got)
	}
	tg2 := &storyboard.Target{TestID: "x"}
	if got := tg2.PlaywrightSelector(); got != `[data-testid="x"]` {
		t.Fatalf("bad selector %q", got)
	}
}

func TestSecretSentinels(t *testing.T) {
	if len(storyboard.SecretSentinels("my api_key is here")) == 0 {
		t.Fatal("expected sentinel")
	}
	if len(storyboard.SecretSentinels("hello world")) != 0 {
		t.Fatal("false positive")
	}
}
