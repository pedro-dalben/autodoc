package agent

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func TestCapsuleOnlyInvalidatesRelevantSources(t *testing.T) {
	d := t.TempDir()
	relevant, unrelated := filepath.Join(d, "chat.html"), filepath.Join(d, "README.md")
	if err := os.WriteFile(relevant, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}
	sb := &storyboard.Storyboard{Version: 1, Meta: storyboard.Meta{Title: "T", Language: "pt-BR"}, Config: storyboard.Config{BaseURL: "http://x"}}
	c, err := NewCapsule("chat-send", sb, []string{relevant})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := c.ChangedSources(); len(got) != 0 {
		t.Fatalf("unrelated source invalidated capsule: %v", got)
	}
	if err := os.WriteFile(relevant, []byte("v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := c.ChangedSources(); len(got) != 1 || got[0] != relevant {
		t.Fatalf("relevant source not invalidated: %v", got)
	}
}
