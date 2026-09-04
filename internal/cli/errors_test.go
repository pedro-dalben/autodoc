package cli

import (
	"errors"
	"strings"
	"testing"
)

func TestFormatErrorUsesStableKnownCodes(t *testing.T) {
	got := FormatError(errors.New("click target not found"))
	if !strings.Contains(got, "E_TARGET_MISSING") || !strings.Contains(got, "hint:") {
		t.Fatalf("bad target error: %s", got)
	}
	got = FormatError(errors.New("storyboard x invalid"))
	if !strings.Contains(got, "E_STORYBOARD_INVALID") {
		t.Fatalf("bad storyboard error: %s", got)
	}
}

func TestFormatErrorKeepsUnknownCause(t *testing.T) {
	if got := FormatError(errors.New("disk exploded")); got != "FAIL: disk exploded" {
		t.Fatalf("bad unknown error: %s", got)
	}
}
