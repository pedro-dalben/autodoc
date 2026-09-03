package e2e

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

func startBin(t *testing.T, path string, env ...string) (*exec.Cmd, string) {
	t.Helper()
	cmd := exec.Command(path)
	cmd.Env = append(os.Environ(), env...)
	var sb strings.Builder
	cmd.Stdout = &sb
	cmd.Stderr = &sb
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cmd.Process.Kill() })
	return cmd, ""
}

func waitHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for %s", url)
}

func buildBins(t *testing.T, dir string) (autodoc, fixture, faketts string) {
	t.Helper()
	root := repoRoot(t)
	autodoc = filepath.Join(dir, "autodoc")
	fixture = filepath.Join(dir, "fixture")
	faketts = filepath.Join(dir, "faketts")
	for _, b := range [][2]string{{autodoc, "github.com/pedro-dalben/autodoc/cmd/autodoc"}, {fixture, "github.com/pedro-dalben/autodoc/test/fixture"}, {faketts, "github.com/pedro-dalben/autodoc/test/faketts"}} {
		cmd := exec.Command("go", "build", "-o", b[0], b[1])
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s: %v %s", b[1], err, out)
		}
	}
	return autodoc, fixture, faketts
}

func run(t *testing.T, dir string, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v failed: %v\n%s", name, args, err, out)
	}
	return string(out)
}

func repoRoot(t *testing.T) string {
	t.Helper()
	wd, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err == nil {
			return wd
		}
		parent := filepath.Dir(wd)
		if parent == wd {
			t.Fatal("go.mod not found")
		}
		wd = parent
	}
}

func TestFullDeterministicPipeline(t *testing.T) {
	if testing.Short() {
		t.Skip("e2e needs browsers+ffmpeg")
	}
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg missing")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		t.Skip("ffprobe missing")
	}
	root := repoRoot(t)
	binDir := t.TempDir()
	autodoc, fixtureBin, fakettsBin := buildBins(t, binDir)
	ttsPort := freePort(t)
	appPort := freePort(t)
	_ = appPort
	ttsPortS := fmt.Sprintf("%d", ttsPort)
	_ = ttsPortS
	work := t.TempDir()
	startBin(t, fakettsBin)
	waitHTTP(t, "http://localhost:8880/healthz", 20*time.Second)
	startBin(t, fixtureBin)
	waitHTTP(t, "http://localhost:8099/healthz", 20*time.Second)

	sb, err := os.ReadFile(filepath.Join(root, "test", "fixture", "storyboard.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "storyboard.yml"), sb, 0o644); err != nil {
		t.Fatal(err)
	}
	toml, err := os.ReadFile(filepath.Join(root, "test", "fixture", "autodoc.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "autodoc.toml"), toml, 0o644); err != nil {
		t.Fatal(err)
	}
	out := run(t, work, autodoc, "storyboard", "validate", "--storyboard", "storyboard.yml")
	if !strings.Contains(out, "storyboard valid") {
		t.Fatalf("validate: %s", out)
	}
	out = run(t, work, autodoc, "tts", "--storyboard", "storyboard.yml")
	if !strings.Contains(out, "miss") {
		t.Fatalf("first tts should miss: %s", out)
	}
	out = run(t, work, autodoc, "tts", "--storyboard", "storyboard.yml")
	if !strings.Contains(out, "hit") || strings.Contains(out, "miss scene") {
		t.Fatalf("second tts should be all hits: %s", out)
	}
	out = run(t, work, autodoc, "record", "--storyboard", "storyboard.yml", "--headless")
	if !strings.Contains(out, "scene scene-001") || !strings.Contains(out, "scene scene-002") {
		t.Fatalf("record should cover both scenes: %s", out)
	}
	out = run(t, work, autodoc, "render", "--storyboard", "storyboard.yml")
	if !strings.Contains(out, "rendered") {
		t.Fatalf("render: %s", out)
	}
	bundle := filepath.Join(work, "bundle")
	out = run(t, work, autodoc, "export", "--storyboard", "storyboard.yml", "--out", bundle)
	if !strings.Contains(out, "exported") {
		t.Fatalf("export: %s", out)
	}
	assertBundle(t, bundle)
	assertNoSecrets(t, bundle)
	out = run(t, work, autodoc, "record", "--storyboard", "storyboard.yml", "--headless", "--retake", "scene-002")
	if !strings.Contains(out, "scene-002") {
		t.Fatalf("retake: %s", out)
	}
	if strings.Contains(out, "scene-001: video") {
		t.Fatalf("retake must only record scene-002: %s", out)
	}
}

func assertBundle(t *testing.T, bundle string) {
	t.Helper()
	for _, f := range []string{"tutorial.mp4", "tutorial.md", "subtitles.srt", "subtitles.vtt", "thumbnail.png", "storyboard.yml", "timeline.json", "metadata.json"} {
		st, err := os.Stat(filepath.Join(bundle, f))
		if err != nil || st.Size() == 0 {
			t.Fatalf("bundle missing %s", f)
		}
	}
	shots, _ := os.ReadDir(filepath.Join(bundle, "screenshots"))
	if len(shots) == 0 {
		t.Fatal("bundle screenshots empty")
	}
	probe := func(args ...string) string {
		cmd := exec.Command("ffprobe", args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("ffprobe: %v %s", err, out)
		}
		return string(out)
	}
	streams := probe("-v", "error", "-show_entries", "stream=codec_name,width,height", "-of", "csv", filepath.Join(bundle, "tutorial.mp4"))
	if !strings.Contains(streams, "h264") {
		t.Fatalf("mp4 must be H.264: %s", streams)
	}
	if !strings.Contains(streams, "aac") {
		t.Fatalf("mp4 must have AAC: %s", streams)
	}
	if !strings.Contains(streams, "1280") {
		t.Fatalf("mp4 must be 1280 wide: %s", streams)
	}
	format := probe("-v", "error", "-show_entries", "format=duration", "-of", "json", filepath.Join(bundle, "tutorial.mp4"))
	var fj struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	_ = json.Unmarshal([]byte(format), &fj)
	tlData, _ := os.ReadFile(filepath.Join(bundle, "timeline.json"))
	var tl struct {
		TotalS   float64 `json:"total_s"`
		Segments []struct {
			StartS float64 `json:"start_s"`
			EndS   float64 `json:"end_s"`
		} `json:"segments"`
	}
	_ = json.Unmarshal(tlData, &tl)
	if len(tl.Segments) == 0 {
		t.Fatal("timeline has no segments")
	}
	var dur float64
	fmt.Sscanf(fj.Format.Duration, "%f", &dur)
	if diff := dur - tl.TotalS; diff > 1.5 || diff < -1.5 {
		t.Fatalf("mp4 duration %.2f vs timeline %.2f", dur, tl.TotalS)
	}
	srt, _ := os.ReadFile(filepath.Join(bundle, "subtitles.srt"))
	if !strings.Contains(string(srt), "-->") {
		t.Fatal("srt malformed")
	}
	vtt, _ := os.ReadFile(filepath.Join(bundle, "subtitles.vtt"))
	if !strings.HasPrefix(string(vtt), "WEBVTT") {
		t.Fatal("vtt malformed")
	}
}

func assertNoSecrets(t *testing.T, bundle string) {
	t.Helper()
	sentinels := []string{"demo1234", "sk-live", "sk-test", "ghp_", "api_key", "aws_secret"}
	filepath.Walk(bundle, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if strings.HasSuffix(p, ".mp4") || strings.HasSuffix(p, ".png") {
			return nil
		}
		b, _ := os.ReadFile(p)
		lower := strings.ToLower(string(b))
		for _, s := range sentinels {
			if strings.Contains(lower, strings.ToLower(s)) {
				t.Fatalf("secret sentinel %q in %s", s, p)
			}
		}
		return nil
	})
}
