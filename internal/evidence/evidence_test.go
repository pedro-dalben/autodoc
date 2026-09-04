package evidence

import (
	"strings"
	"testing"
)

func TestStoreDeduplicatesAndRecoversExactPayload(t *testing.T) {
	s := NewStore(t.TempDir())
	payload := `{"controls":["send","message"]}`
	a, err := s.Put("ui_inventory", payload, "two controls", nil, "/chat")
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.Put("ui_inventory", payload, "two controls", nil, "/chat")
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != b.ID || s.Stats().Refs != 1 {
		t.Fatalf("dedup failed: %#v %#v", a, b)
	}
	got, err := s.Get(a.ID)
	if err != nil || got != payload {
		t.Fatalf("recovery got %q err %v", got, err)
	}
}

func TestRenderBypassesCompressionWhenMetadataCostsMore(t *testing.T) {
	raw := "small"
	out := Render(Ref{ID: "ev_small", Bytes: len(raw)}, raw, "much larger compact representation")
	if out.Mode != "raw" || out.Text != raw {
		t.Fatalf("expected raw bypass: %#v", out)
	}
}

func TestRenderCompressesLargePayloadWithRecoveryRef(t *testing.T) {
	raw := strings.Repeat("browser snapshot ", 200)
	out := Render(Ref{ID: "ev_large", Bytes: len(raw)}, raw, "2 relevant controls")
	if out.Mode != "compact" || !strings.Contains(out.Text, "autodoc evidence get ev_large") {
		t.Fatalf("expected compact recovery ref: %#v", out)
	}
}
