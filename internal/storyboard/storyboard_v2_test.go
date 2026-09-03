package storyboard_test

import (
	"os"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func TestV1StillValidWithoutVisuals(t *testing.T) {
	sb, err := storyboard.LoadFile(writeTmp(t, validSB))
	if err != nil {
		t.Fatalf("v1 storyboard must keep validating: %v", err)
	}
	vc := sb.VisualsOrDefault()
	if !vc.CursorOn() || !vc.RippleOn() || !vc.CameraOn() {
		t.Fatal("v1 storyboards must inherit cinematic defaults")
	}
}

func TestV2VisualsAndSetupSequence(t *testing.T) {
	yml := `version: 2
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:8099", viewport_width: 1280, viewport_height: 720}
setup:
  start_url: "/login"
  sequence:
    - action: {type: fill, target: {test_id: login-user}, secret_ref: "env:FIXTURE_USER"}
    - action: {type: click, target: {test_id: login-btn}}
    - wait: {state: visible, target: {test_id: new-material-btn}, timeout_ms: 8000}
visuals:
  typing: {char_delay_ms: 60}
  camera: {click_zoom: 1.1}
scenes:
  - id: scene-001
    title: "S1"
    url: "/materiais"
    beats:
      - id: beat-01
        sequence:
          - speech: {text: "Clique em Novo."}
          - action: {type: click, target: {test_id: new-material-btn}}
          - wait: {state: visible, target: {role: dialog}, timeout_ms: 8000, compressible: false}
`
	t.Setenv("FIXTURE_USER", "demo")
	sb, err := storyboard.LoadFile(writeTmp(t, yml))
	if err != nil {
		t.Fatalf("v2 storyboard must validate: %v", err)
	}
	if len(sb.Setup.Sequence) != 3 {
		t.Fatalf("setup sequence lost: %+v", sb.Setup)
	}
	if sb.Scenes[0].Beats[0].Sequence[2].Wait.IsCompressible() {
		t.Fatal("compressible:false must be honored")
	}
	if sb.VisualsOrDefault().Typing.CharDelayMs != 60 {
		t.Fatal("visuals override lost")
	}
	if v, err := storyboard.ResolveSecret("env:FIXTURE_USER"); err != nil || v != "demo" {
		t.Fatalf("secret_ref resolution failed: %q %v", v, err)
	}
}

func TestSecretRefValidation(t *testing.T) {
	bad := `version: 2
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:8099"}
setup: {start_url: "/login"}
scenes:
  - id: s1
    title: "S"
    url: "/login"
    beats:
      - id: b1
        sequence:
          - action: {type: fill, target: {test_id: x}, value: "abc", secret_ref: "env:X"}
`
	if _, err := storyboard.LoadFile(writeTmp(t, bad)); err == nil {
		t.Fatal("literal value + secret_ref must be rejected")
	}
	noref := `version: 2
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:8099"}
setup: {start_url: "/login"}
scenes:
  - id: s1
    title: "S"
    url: "/login"
    beats:
      - id: b1
        sequence:
          - action: {type: fill, target: {test_id: x}}
`
	if _, err := storyboard.LoadFile(writeTmp(t, noref)); err == nil {
		t.Fatal("fill without value/text/secret_ref must be rejected")
	}
}

func TestWaitValueFieldForURL(t *testing.T) {
	yml := `version: 1
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:3000"}
setup: {start_url: "/admin/chat"}
scenes:
  - id: chat
    title: "C"
    url: "/admin/chat"
    beats:
      - id: b1
        sequence:
          - speech: {text: "Oi."}
          - wait: {state: url, value: "/admin/chat/553", timeout_ms: 8000}
`
	sb, err := storyboard.LoadFile(writeTmp(t, yml))
	if err != nil {
		t.Fatalf("url wait with value must validate: %v", err)
	}
	w := sb.Scenes[0].Beats[0].Sequence[1].Wait
	if w.Value != "/admin/chat/553" {
		t.Fatalf("wait value lost: %+v", w)
	}
}

var _ = os.Getenv
