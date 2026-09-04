package evidence_test

import (
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/evidence"
)

func TestCompactLineOmitsGeometry(t *testing.T) {
	el := evidence.Element{
		Role: "button", Name: "Enviar", TestID: "send-btn", ID: "send-btn",
		Region: "composer", BBox: evidence.BBox{X: 10, Y: 20, W: 100, H: 40},
		Enabled: true,
	}
	line := el.CompactLine()
	if strings.Contains(line, "bbox") {
		t.Fatalf("compact line must not carry geometry: %q", line)
	}
	if strings.Contains(line, "[id=") {
		t.Fatalf("compact line must not duplicate id when test_id present: %q", line)
	}
	if !strings.Contains(line, "[send-btn]") || !strings.Contains(line, "region=composer") {
		t.Fatalf("compact line must keep locator + region: %q", line)
	}
}

func TestCompactLineFallsBackToID(t *testing.T) {
	el := evidence.Element{Role: "textbox", Name: "Mensagem", ID: "msg", Region: "composer", Enabled: true}
	if line := el.CompactLine(); !strings.Contains(line, "[id=msg]") {
		t.Fatalf("compact line must fall back to id: %q", line)
	}
}
