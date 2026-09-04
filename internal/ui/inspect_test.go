package ui

import (
	"testing"

	"github.com/pedro-dalben/autodoc/internal/evidence"
)

func TestFocusedInventoryPrefersMessageControls(t *testing.T) {
	controls := []evidence.Element{
		{Role: "link", Name: "Dashboard", Region: "sidebar"},
		{Role: "textbox", Name: "Mensagem", Region: "composer"},
		{Role: "button", Name: "Enviar", Region: "composer"},
	}
	got := filterControls(controls, "enviar mensagem", 2)
	if len(got) != 2 {
		t.Fatalf("unexpected focused controls: %#v", got)
	}
}

func TestSemanticDiffReportsChangedRegion(t *testing.T) {
	before := evidence.Inventory{Controls: []evidence.Element{{Role: "textbox", Name: "Mensagem", Region: "composer", Enabled: false}}}
	after := evidence.Inventory{Controls: []evidence.Element{{Role: "textbox", Name: "Mensagem", Region: "composer", Enabled: true}, {Role: "article", Text: "Olá", Region: "main"}}}
	d := semanticDiff(before, after)
	if len(d.Added) != 1 || len(d.Changed) != 1 || len(d.Changes) != 2 {
		t.Fatalf("unexpected diff: %#v", d)
	}
	if d.Region != "composer" && d.Region != "main" {
		t.Fatalf("missing meaningful region: %#v", d)
	}
}
