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

func TestFilterControlsWithConfidence(t *testing.T) {
	controls := []evidence.Element{
		{Role: "link", Name: "Dashboard", Region: "sidebar"},
		{Role: "textbox", Name: "Mensagem", Region: "composer"},
		{Role: "button", Name: "Enviar Mensagem", Region: "composer"},
		{Role: "link", Name: "Ajuda", Region: "sidebar"},
		{Role: "link", Name: "Configurações", Region: "sidebar"},
		{Role: "button", Name: "Cancelar", Region: "composer"},
	}
	// High confidence intent
	got, conf, diag := FilterControlsWithConfidence(controls, "enviar mensagem", 10)
	if conf != "high (focused)" {
		t.Errorf("expected high confidence, got %s", conf)
	}
	if diag != "" {
		t.Errorf("expected empty diagnostic for high confidence, got %s", diag)
	}
	if len(got) > 5 {
		t.Errorf("high confidence should focus and cap at 5, got %d", len(got))
	}

	// Low confidence intent
	gotLow, confLow, diagLow := FilterControlsWithConfidence(controls, "relatório financeiro anual", 10)
	if confLow != "low (uncertain)" {
		t.Errorf("expected low confidence, got %s", confLow)
	}
	if diagLow == "" {
		t.Errorf("expected diagnostic hint for low confidence")
	}
	if len(gotLow) != len(controls) {
		t.Errorf("low confidence should return all controls up to limit, got %d", len(gotLow))
	}
}

func TestSemanticDiffNoiseFilteringAndAlerts(t *testing.T) {
	// 1. Clock noise should be ignored
	beforeClock := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "span", Role: "timer", Name: "Clock", Text: "14:30:00", Region: "header", Enabled: true},
	}}
	afterClock := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "span", Role: "timer", Name: "Clock", Text: "14:30:01", Region: "header", Enabled: true},
	}}
	diffClock := semanticDiff(beforeClock, afterClock)
	if len(diffClock.Changed) != 0 {
		t.Errorf("clock noise should be filtered out, got changes: %v", diffClock.Changed)
	}

	// 2. ID matching stability
	beforeID := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "button", ID: "submit-btn", Role: "button", Name: "Save", Enabled: false},
	}}
	afterID := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "button", ID: "submit-btn", Role: "button", Name: "Save", Enabled: true},
	}}
	diffID := semanticDiff(beforeID, afterID)
	if len(diffID.Changed) != 1 || len(diffID.Added) != 0 {
		t.Errorf("ID matching should track element state change, got: %+v", diffID)
	}

	// 3. Toast/Alert detection
	beforeToast := evidence.Inventory{Controls: []evidence.Element{}}
	afterToast := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "div", Role: "alert", Name: "Mensagem enviada com sucesso", Region: "header"},
	}}
	diffToast := semanticDiff(beforeToast, afterToast)
	if diffToast.Alert != "Mensagem enviada com sucesso" {
		t.Errorf("expected Alert to be detected, got %q", diffToast.Alert)
	}

	// 4. Spinner noise ignored when real content added
	beforeSpinner := evidence.Inventory{Controls: []evidence.Element{}}
	afterSpinner := evidence.Inventory{Controls: []evidence.Element{
		{Tag: "div", Role: "progressbar", Name: "spinner", Region: "main"},
		{Tag: "p", Role: "article", Name: "Content", Text: "Real content", Region: "main"},
	}}
	diffSpinner := semanticDiff(beforeSpinner, afterSpinner)
	if len(diffSpinner.Added) != 1 {
		t.Errorf("transient spinner should be filtered when real content is present, got: %v", diffSpinner.Added)
	}
}
