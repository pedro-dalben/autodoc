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

func TestDetectState(t *testing.T) {
	d := t.TempDir()

	// 1. UNKNOWN initially
	st := DetectState(d, "")
	if st.State != StateUnknown {
		t.Errorf("initial state should be UNKNOWN, got %s", st.State)
	}

	// 2. DISCOVERED when evidence exists
	_ = os.MkdirAll(filepath.Join(d, ".autodoc", "cache", "evidence"), 0o755)
	_ = os.WriteFile(filepath.Join(d, ".autodoc", "cache", "evidence", "index.jsonl"), []byte("{\"id\":\"ev1\"}\n"), 0o644)
	st = DetectState(d, "")
	if st.State != StateDiscovered || st.EvidenceCount != 1 {
		t.Errorf("expected DISCOVERED with 1 evidence, got %s (%d)", st.State, st.EvidenceCount)
	}

	// 3. PLANNED when storyboard exists
	_ = os.WriteFile(filepath.Join(d, "storyboard.yml"), []byte("title: test"), 0o644)
	st = DetectState(d, "storyboard.yml")
	if st.State != StatePlanned {
		t.Errorf("expected PLANNED, got %s", st.State)
	}

	// 4. RECORDED when scene webm exists in latest run
	runDir := filepath.Join(d, ".autodoc", "_work", "20260904-120000")
	_ = os.MkdirAll(filepath.Join(runDir, "video"), 0o755)
	_ = os.WriteFile(filepath.Join(runDir, "video", "scene-001.webm"), []byte("webm"), 0o644)
	st = DetectState(d, "storyboard.yml")
	if st.State != StateRecorded {
		t.Errorf("expected RECORDED, got %s", st.State)
	}

	// 5. RENDERED when tutorial.mp4 exists
	_ = os.WriteFile(filepath.Join(runDir, "tutorial.mp4"), []byte("mp4"), 0o644)
	st = DetectState(d, "storyboard.yml")
	if st.State != StateRendered {
		t.Errorf("expected RENDERED, got %s", st.State)
	}

	// 6. VERIFIED when qa_report.json exists
	_ = os.WriteFile(filepath.Join(runDir, "qa_report.json"), []byte("{\"passed\":true}"), 0o644)
	st = DetectState(d, "storyboard.yml")
	if st.State != StateVerified {
		t.Errorf("expected VERIFIED, got %s", st.State)
	}
}
