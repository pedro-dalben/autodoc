package capture

import (
	"testing"
)

func TestAdaptiveTypingDelay(t *testing.T) {
	cases := []struct {
		text      string
		baseDelay int
		wantMin   float64
		wantMax   float64
	}{
		{"short", 60, 60.0, 60.0},
		{"123456789012345", 60, 60.0, 60.0},
		{"medium text example here", 60, 40.0, 50.0},
		{"this is a much longer passage that should be typed significantly faster so the video doesn't drag forever", 60, 18.0, 35.0},
	}

	for _, tc := range cases {
		d := AdaptiveTypingDelay(tc.text, tc.baseDelay)
		if d < tc.wantMin || d > tc.wantMax {
			t.Errorf("text len %d: got delay %.1f, want between %.1f and %.1f", len(tc.text), d, tc.wantMin, tc.wantMax)
		}
	}
}
