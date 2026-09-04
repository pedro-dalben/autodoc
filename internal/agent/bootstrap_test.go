package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassify(t *testing.T) {
	if got := Classify("regravar scene-02", true); got != Retake {
		t.Fatalf("got %s", got)
	}
	if got := Classify("crie um tutorial", false); got != Create {
		t.Fatalf("got %s", got)
	}
	if got := Classify("atualize o tutorial", true); got != Update {
		t.Fatalf("got %s", got)
	}
}

func TestBootstrapReusesLocalState(t *testing.T) {
	d := t.TempDir()
	if err := os.MkdirAll(filepath.Join(d, ".autodoc/cache/auth"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(d, "storyboard.yml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := Bootstrap(d, "atualize o botão", "storyboard.yml")
	for _, want := range []string{"mode: UPDATE", "auth: cached", "patch only affected scene"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestGuideIsScoped(t *testing.T) {
	if got, ok := Guide("cinematic"); !ok || !strings.Contains(got, "director") {
		t.Fatalf("bad guide: %q %t", got, ok)
	}
	if _, ok := Guide("everything"); ok {
		t.Fatal("unknown guide must not produce a giant fallback")
	}
}
