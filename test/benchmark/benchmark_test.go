package benchmark

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/pedro-dalben/autodoc/internal/agent"
	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/doctor"
	"github.com/pedro-dalben/autodoc/internal/pipeline"
	"github.com/pedro-dalben/autodoc/internal/tts"
	"github.com/pedro-dalben/autodoc/internal/ui"
	"github.com/pedro-dalben/autodoc/internal/usage"
)

func TestBenchmarks(t *testing.T) {
	usageDir := filepath.Join(t.TempDir(), "usage")

	// 1. Small Page vs Large Page Focused Retrieval Benchmark
	t.Run("SmallPage_vs_LargePage_Retrieval", func(t *testing.T) {
		smallSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintln(w, `<!DOCTYPE html><html><body>
				<h1>Login</h1>
				<input id="user" name="username" placeholder="Username" />
				<button id="submit">Entrar</button>
			</body></html>`)
		}))
		defer smallSrv.Close()

		largeSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			var b strings.Builder
			b.WriteString(`<!DOCTYPE html><html><body><header><span class="clock">15:30:00</span></header><main>`)
			b.WriteString(`<form id="composer"><textarea name="mensagem" placeholder="Digite sua mensagem"></textarea><button type="submit">Enviar Mensagem</button></form>`)
			b.WriteString(`<table>`)
			for i := 0; i < 150; i++ {
				fmt.Fprintf(&b, `<tr><td>Row %d</td><td><button id="btn-%d">Ação %d</button></td><td><a href="/item/%d">Link %d</a></td></tr>`, i, i, i, i, i)
			}
			b.WriteString(`</table></main></body></html>`)
			fmt.Fprintln(w, b.String())
		}))
		defer largeSrv.Close()

		// Benchmark Small Page
		startSmall := time.Now()
		smallInv, _, err := ui.Inspect(ui.InspectOptions{URL: smallSrv.URL, Intent: "entrar no sistema"})
		durSmall := time.Since(startSmall)
		if err != nil {
			t.Fatalf("small page inspect: %v", err)
		}
		smallBytes := len(fmt.Sprintf("%+v", smallInv.Controls))
		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseBrowser,
			Bytes:    smallBytes,
			Label:    fmt.Sprintf("bench_small_page (%d controls, %v)", len(smallInv.Controls), durSmall),
			Note:     "Small page retrieval benchmark",
			Estimate: true,
		})

		// Benchmark Large Page
		startLarge := time.Now()
		largeInv, _, err := ui.Inspect(ui.InspectOptions{URL: largeSrv.URL, Intent: "enviar mensagem"})
		durLarge := time.Since(startLarge)
		if err != nil {
			t.Fatalf("large page inspect: %v", err)
		}
		largeBytes := len(fmt.Sprintf("%+v", largeInv.Controls))
		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseBrowser,
			Bytes:    largeBytes,
			Label:    fmt.Sprintf("bench_large_page (%d controls, %v)", len(largeInv.Controls), durLarge),
			Note:     "Large page (150+ elements) focused retrieval benchmark",
			Estimate: true,
		})

		// Assert focused compression on large page: capped at <= 5 controls despite 150+ candidates
		if len(largeInv.Controls) > 5 {
			t.Errorf("large page should be focused to <= 5 controls, got %d", len(largeInv.Controls))
		}
		if largeInv.Confidence != "high (focused)" {
			t.Errorf("expected high confidence on large page, got %s", largeInv.Confidence)
		}
	})

	// 2. Granular Scene Hash & Cold vs Warm vs No-Change Benchmark
	t.Run("Cold_vs_Warm_vs_NoChange_Rebuild", func(t *testing.T) {
		dir := t.TempDir()
		cfg := config.Default()

		sb1Content := `version: 2
meta:
  title: "Benchmark Tutorial"
  language: "pt-BR"
config:
  base_url: "http://localhost:8080"
scenes:
  - id: scene-001
    title: Scene One
    url: "/"
    beats:
      - id: beat-01
        sequence:
          - speech:
              text: "First scene narration."
          - action:
              type: click
              target: { text: "Btn 1" }
  - id: scene-002
    title: Scene Two
    url: "/"
    beats:
      - id: beat-01
        sequence:
          - speech:
              text: "Second scene narration."
          - action:
              type: click
              target: { text: "Btn 2" }
`
		sb1Path := filepath.Join(dir, "storyboard.yml")
		if err := os.WriteFile(sb1Path, []byte(sb1Content), 0o644); err != nil {
			t.Fatal(err)
		}

		// Cold Run
		startCold := time.Now()
		run1, err := pipeline.NewRun(dir, sb1Path, cfg)
		if err != nil {
			t.Fatal(err)
		}
		run1.RunID = "20260904-000001"
		if err := run1.Compile(); err != nil {
			t.Fatal(err)
		}
		// Synthesize fake TTS for cold run
		fakeProv := tts.Disabled{}
		_, _ = run1.SynthesizeTTS(context.Background(), fakeProv, nil)
		durCold := time.Since(startCold)

		// Create mock scene-001 video in run1 work dir
		run1Dir := filepath.Join(run1.WorkDir, run1.RunID)
		_ = os.MkdirAll(filepath.Join(run1Dir, "raw"), 0o755)
		_ = os.WriteFile(filepath.Join(run1Dir, "raw", "scene-001.webm"), []byte("webm-scene-1"), 0o644)
		_ = os.WriteFile(filepath.Join(run1Dir, "raw", "scene-002.webm"), []byte("webm-scene-2"), 0o644)

		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseStoryboard,
			Bytes:    len(sb1Content),
			Label:    fmt.Sprintf("bench_cold_run (%v)", durCold),
			Note:     "Cold run compile & synthesis",
			Estimate: true,
		})

		// Warm Run: modify ONLY Scene 2 narration
		sb2Content := strings.Replace(sb1Content, "Second scene narration.", "Updated second scene narration.", 1)
		sb2Path := filepath.Join(dir, "storyboard_v2.yml")
		if err := os.WriteFile(sb2Path, []byte(sb2Content), 0o644); err != nil {
			t.Fatal(err)
		}

		startWarm := time.Now()
		run2, err := pipeline.NewRun(dir, sb2Path, cfg)
		if err != nil {
			t.Fatal(err)
		}
		run2.RunID = "20260904-000002"
		if err := run2.Compile(); err != nil {
			t.Fatal(err)
		}
		durWarm := time.Since(startWarm)

		// Verify granular scene video reuse for Scene 1!
		foundVideo, srcDir := run2.FindSceneVideo("scene-001")
		if foundVideo == "" || srcDir != run1Dir {
			t.Fatalf("warm run: scene-001 video should be reused from %s, got %s (src: %s)", run1Dir, foundVideo, srcDir)
		}

		// Verify Scene 2 is NOT reused because its hash changed
		foundVideo2, _ := run2.FindSceneVideo("scene-002")
		if foundVideo2 != "" {
			t.Fatalf("warm run: modified scene-002 video should NOT be reused, but found %s", foundVideo2)
		}

		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseStoryboard,
			Bytes:    len(sb2Content),
			Label:    fmt.Sprintf("bench_warm_run (%v, scene-001 reused)", durWarm),
			Note:     "Warm run with granular scene hash reuse",
			Estimate: true,
		})

		// No-Change Run: run with exact same sb2
		startNoChange := time.Now()
		run3, err := pipeline.NewRun(dir, sb2Path, cfg)
		if err != nil {
			t.Fatal(err)
		}
		if err := run3.Compile(); err != nil {
			t.Fatal(err)
		}
		durNoChange := time.Since(startNoChange)

		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseStoryboard,
			Bytes:    len(sb2Content),
			Label:    fmt.Sprintf("bench_no_change_run (%v)", durNoChange),
			Note:     "No-change run verification",
			Estimate: true,
		})
	})

	// 3. Failure Recovery & Self-Diagnosis Benchmark
	t.Run("Failure_Recovery_And_Diagnosis", func(t *testing.T) {
		dir := t.TempDir()

		// Initially uninitialized -> StateUnknown
		st0 := agent.DetectState(dir, "")
		if st0.State != agent.StateUnknown {
			t.Errorf("expected StateUnknown, got %s", st0.State)
		}

		// Self-diagnose uninitialized workspace
		startDiag := time.Now()
		diag := doctor.Diagnose(dir)
		durDiag := time.Since(startDiag)
		if len(diag.Remediation) == 0 {
			t.Error("expected diagnosis to provide remediation actions on empty dir")
		}

		// Recover by adding autodoc.toml and storyboard.yml
		_ = os.WriteFile(filepath.Join(dir, "autodoc.toml"), []byte(""), 0o644)
		_ = os.WriteFile(filepath.Join(dir, "storyboard.yml"), []byte("title: Recovered\nscenes: []\n"), 0o644)

		stRecovered := agent.DetectState(dir, "storyboard.yml")
		if stRecovered.State != agent.StatePlanned {
			t.Errorf("expected recovered state StatePlanned, got %s", stRecovered.State)
		}

		_ = usage.Log(usageDir, usage.Entry{
			Phase:    usage.PhaseCLI,
			Bytes:    256,
			Label:    fmt.Sprintf("bench_failure_recovery (%v)", durDiag),
			Note:     "Self-diagnosis and failure recovery benchmark",
			Estimate: true,
		})
	})

	// Generate and print summary of benchmark entries
	entries, err := usage.Load(usageDir)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("=== Benchmark Suite Results (%d benchmarks recorded) ===", len(entries))
	for _, e := range entries {
		t.Logf("  [%s] %s — %s", e.Phase, e.Label, e.Note)
	}
}
