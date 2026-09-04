package storyboard_test

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func TestCompactTargetsAndDefaultsExpandBeforeCompile(t *testing.T) {
	yml := `version: 2
meta: {title: "Chat", language: "pt-BR"}
config: {base_url: "http://localhost:3000"}
scenes:
  - id: chat
    title: "Chat"
    targets:
      message: {label: "Mensagem"}
      send: {role: button, name: Enviar}
    defaults: {wait_timeout_ms: 8000, compressible: false, result_hold_ms: 1400}
    beats:
      - id: send
        sequence:
          - action: {type: fill, target: message, value: "Olá"}
          - action: {type: click, target: send, result_target: send}
          - wait: {state: visible, target: send}
`
	sb, err := storyboard.LoadFile(writeTmp(t, yml))
	if err != nil {
		t.Fatal(err)
	}
	beat := sb.Scenes[0].Beats[0]
	if beat.Sequence[0].Action.Target.Label != "Mensagem" || beat.Sequence[1].Action.Target.Name != "Enviar" {
		t.Fatalf("aliases not expanded: %+v", beat)
	}
	if beat.Sequence[1].Action.ResultHoldMs == nil || *beat.Sequence[1].Action.ResultHoldMs != 1400 {
		t.Fatal("result default missing")
	}
	if beat.Sequence[2].Wait.TimeoutMs != 8000 || beat.Sequence[2].Wait.IsCompressible() {
		t.Fatal("wait defaults missing")
	}
	r := recipe.Compile(sb, "disabled", "", "pt-BR", 1)
	if r.Scenes[0].Beats[0].Steps[1].Action.Target.Name != "Enviar" {
		t.Fatal("compiler saw compact alias")
	}
}

func TestUnknownCompactTargetFails(t *testing.T) {
	yml := `version: 2
meta: {title: "T", language: "pt-BR"}
config: {base_url: "http://localhost:3000"}
scenes:
  - id: s
    title: "S"
    beats: [{id: b, sequence: [{action: {type: click, target: missing}}}]
`
	if _, err := storyboard.LoadFile(writeTmp(t, yml)); err == nil {
		t.Fatal("unknown alias must fail")
	}
}
