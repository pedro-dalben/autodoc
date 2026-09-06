// Package runreport aggregates facts the AutoDoc pipeline already produces
// (recipe, timeline, TTS cache report, capture events, final timeline,
// cinematic QA, rendered MP4) into one deterministic, local evidence
// document per run: <workdir>/<run>/evidence.json plus a human-readable
// rendering. It is observation only: it never changes pipeline behavior,
// and missing artifacts yield available:false sections instead of errors.
//
// Privacy: the report carries IDs, hashes, counts and metrics. It never
// copies speech text, URLs, DOM, storage state, cookies or secret values.
package runreport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// SchemaVersion versions the evidence.json format. It is an explicit
// integer, never a timestamp.
const SchemaVersion = 1

// Report is the deterministic per-run evidence document.
type Report struct {
	Version        string    `json:"version"`
	RunID          string    `json:"run_id"`
	AutodocVersion string    `json:"autodoc_version"`
	Tutorial       string    `json:"tutorial"`
	StoryboardHash string    `json:"storyboard_hash"`
	Pipeline       Pipeline  `json:"pipeline"`
	TTS            TTS       `json:"tts"`
	Capture        Capture   `json:"capture"`
	Recovery       Recovery  `json:"recovery"`
	Sync           Sync      `json:"sync"`
	Cinematic      Cinematic `json:"cinematic"`
	Security       Security  `json:"security"`
	Output         Output    `json:"output"`
}

type Pipeline struct {
	Available bool     `json:"available"`
	Scenes    int      `json:"scenes_total"`
	Beats     int      `json:"beats_total"`
	Actions   int      `json:"actions_total"`
	Waits     int      `json:"waits_total"`
	Holds     int      `json:"holds_total"`
	Speech    int      `json:"speech_segments"`
	SceneIDs  []string `json:"scene_ids"`
}

type TTS struct {
	Available bool    `json:"available"`
	Segments  int     `json:"segments_total"`
	Hits      int     `json:"cache_hits"`
	Misses    int     `json:"cache_misses"`
	HitRatio  float64 `json:"cache_hit_ratio"`
	SynthSecs float64 `json:"synthesized_duration_s"`
}

type Capture struct {
	Available   bool     `json:"available"`
	ScenesTotal int      `json:"scenes_total"`
	Recorded    int      `json:"scenes_recorded"`
	Reused      int      `json:"scenes_reused"`
	Retaken     int      `json:"scenes_retaken"`
	Missing     int      `json:"scenes_missing"`
	ReuseRatio  float64  `json:"capture_reuse_ratio"`
	RecordedIDs []string `json:"recorded_ids"`
	ReusedIDs   []string `json:"reused_ids"`
	EventsTotal int      `json:"events_total"`
}

// Recovery carries what the pipeline can prove automatically. Attempts,
// successes beyond retakes, and manual interventions are recorded by the
// dogfood log, never invented here.
type Recovery struct {
	Available     bool     `json:"available"`
	FailedScenes  []string `json:"failed_scenes"`
	RetakenScenes []string `json:"retaken_scenes"`
	ManualNote    string   `json:"manual_interventions_note"`
}

type Sync struct {
	Available        bool    `json:"available"`
	MaxDriftMs       float64 `json:"max_drift_ms"`
	MeanDriftMs      float64 `json:"mean_drift_ms"`
	OutsideThreshold int     `json:"outside_threshold"`
	Pass             bool    `json:"pass"`
}

type Cinematic struct {
	Available bool `json:"available"`
	Requested int  `json:"directives_requested"`
	Honored   int  `json:"directives_honored"`
	Ignored   int  `json:"directives_ignored"`
	QAPass    bool `json:"qa_pass"`
	QAScore   int  `json:"qa_score"`
}

type Security struct {
	Available       bool   `json:"available"`
	SecretScan      string `json:"secret_scan"`
	RedactSelectors int    `json:"redact_selectors"`
	MaskPassword    bool   `json:"mask_password_inputs"`
}

type Output struct {
	Available  bool    `json:"available"`
	DurationS  float64 `json:"duration_s"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
	FPS        float64 `json:"fps"`
	VideoCodec string  `json:"video_codec"`
	AudioCodec string  `json:"audio_codec"`
	Bytes      int64   `json:"bytes"`
}

// Minimal shapes of the pipeline artifacts we aggregate. Only the fields
// we need are decoded; speech text and URLs are never read.
type recipeFile struct {
	StoryboardHash string `json:"storyboard_hash"`
	Redact         struct {
		Selectors    []string `json:"selectors"`
		MaskPassword *bool    `json:"mask_password_inputs"`
	} `json:"redact"`
	Scenes []struct {
		ID        string `json:"id"`
		SceneHash string `json:"scene_hash"`
		Beats     []struct {
			ID    string `json:"id"`
			Steps []struct {
				Kind string `json:"kind"`
			} `json:"steps"`
		} `json:"beats"`
	} `json:"scenes"`
	SpeechSegments []struct {
		ID string `json:"id"`
	} `json:"speech_segments"`
}

type ttsReportFile struct {
	Segments  int     `json:"segments_total"`
	Hits      int     `json:"cache_hits"`
	Misses    int     `json:"cache_misses"`
	SynthSecs float64 `json:"synthesized_duration_s"`
}

type finalTimelineFile struct {
	StoryboardHash string `json:"storyboard_hash"`
	Sync           *struct {
		Items []struct {
			DriftMs float64 `json:"drift_ms"`
			Pass    bool    `json:"pass"`
		} `json:"items"`
		MaxDriftMs float64 `json:"max_drift_ms"`
		Pass       bool    `json:"pass"`
	} `json:"sync"`
}

type cinematicReportFile struct {
	StoryboardHash string `json:"storyboard_hash"`
	Pass           bool   `json:"pass"`
	Score          int    `json:"score"`
}

type complianceFile struct {
	Requested int `json:"requested"`
	Honored   int `json:"honored"`
	Ignored   int `json:"ignored"`
}

type cinematicBundleDir struct {
	Report     *cinematicReportFile `json:"-"`
	Compliance *complianceFile      `json:"-"`
}

// LatestRun returns the most useful run dir: the latest run containing
// both final timeline and rendered MP4 wins; then final timeline only;
// then MP4 only; then any recipe. Lexicographic order matches the
// YYYYMMDD-HHMMSS run ID clock, so "latest" is deterministic.
func LatestRun(workDir string) string {
	withRecipe := runDirsWith(workDir, "recipe.json")
	if len(withRecipe) == 0 {
		return ""
	}
	for _, want := range [][]string{{"final_timeline.json", "tutorial.mp4"}, {"final_timeline.json"}, {"tutorial.mp4"}} {
		for i := len(withRecipe) - 1; i >= 0; i-- {
			if hasAll(workDir, withRecipe[i], want) {
				return withRecipe[i]
			}
		}
	}
	return withRecipe[len(withRecipe)-1]
}

func runDirsWith(workDir, file string) []string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(workDir, e.Name(), file)); err == nil {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func hasAll(workDir, run string, files []string) bool {
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(workDir, run, f)); err != nil {
			return false
		}
	}
	return true
}

// Collect aggregates every fact available for runID under workDir.
// It never fails on partial runs: missing artifacts produce
// available:false sections.
func Collect(workDir, runID, autodocVersion, tutorial string) *Report {
	r := &Report{
		Version:        fmt.Sprintf("v%d", SchemaVersion),
		RunID:          runID,
		AutodocVersion: autodocVersion,
		Tutorial:       tutorial,
		Recovery: Recovery{
			Available:  true,
			ManualNote: "manual interventions are recorded in the dogfood log, not inferred",
		},
		Security: Security{Available: true, SecretScan: "unknown"},
	}
	runDir := filepath.Join(workDir, runID)
	rec := readRecipe(filepath.Join(runDir, "recipe.json"))
	if rec == nil {
		return r
	}
	r.StoryboardHash = rec.StoryboardHash
	r.Pipeline = pipelineOf(rec)
	r.Security.SecretScan = "pass (compile-time storyboard scan)"
	r.Security.RedactSelectors = len(rec.Redact.Selectors)
	if rec.Redact.MaskPassword != nil {
		r.Security.MaskPassword = *rec.Redact.MaskPassword
	}
	// TTS facts are content-identical for a given storyboard hash, so a
	// render-only run dir reuses the same-hash tts report. Search
	// newest-first: read-only commands also mint recipe-only run dirs.
	tts := readTTSReport(filepath.Join(runDir, "tts-report.json"))
	if tts == nil {
		if sibs := sameHashRuns(workDir, runID, rec.StoryboardHash); len(sibs) > 0 {
			for i := len(sibs) - 1; i >= 0; i-- {
				if tts = readTTSReport(filepath.Join(workDir, sibs[i], "tts-report.json")); tts != nil {
					break
				}
			}
		}
	}
	if tts != nil {
		r.TTS = TTS{Available: true, Segments: tts.Segments, Hits: tts.Hits,
			Misses: tts.Misses, SynthSecs: tts.SynthSecs}
		if tts.Segments > 0 {
			r.TTS.HitRatio = float64(tts.Hits) / float64(tts.Segments)
		}
	}
	r.Capture, r.Recovery = captureOf(workDir, runID, rec)
	if fin := readFinal(filepath.Join(runDir, "final_timeline.json")); fin != nil && fin.Sync != nil {
		r.Sync = syncOf(fin)
	}
	r.Cinematic = cinematicOf(runDir)
	r.Output = outputOf(filepath.Join(runDir, "tutorial.mp4"))
	return r
}

func readRecipe(path string) *recipeFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var rec recipeFile
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil
	}
	return &rec
}

func readTTSReport(path string) *ttsReportFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var t ttsReportFile
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

func readFinal(path string) *finalTimelineFile {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var f finalTimelineFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil
	}
	return &f
}

func pipelineOf(rec *recipeFile) Pipeline {
	p := Pipeline{Available: true, Speech: len(rec.SpeechSegments)}
	for _, sc := range rec.Scenes {
		p.SceneIDs = append(p.SceneIDs, sc.ID)
		p.Scenes++
		for _, b := range sc.Beats {
			p.Beats++
			for _, st := range b.Steps {
				switch st.Kind {
				case "action":
					p.Actions++
				case "wait":
					p.Waits++
				case "hold":
					p.Holds++
				}
			}
		}
	}
	sort.Strings(p.SceneIDs)
	return p
}

// sameHashRuns lists sibling run dirs (excluding runID) whose recipe
// matches storyboardHash, oldest first.
func sameHashRuns(workDir, runID, storyboardHash string) []string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == runID {
			continue
		}
		rec := readRecipe(filepath.Join(workDir, e.Name(), "recipe.json"))
		if rec != nil && rec.StoryboardHash == storyboardHash {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out
}

func sceneRaw(dir, sceneID string) string {
	for _, ext := range []string{".webm", ".mp4"} {
		if p := filepath.Join(dir, "raw", sceneID+ext); fileExists(p) {
			return p
		}
	}
	return ""
}

// newestSceneRaw returns the most recent sibling run dir (excluding runID)
// holding a raw capture for sceneID with a matching scene hash. Empty
// sceneHash disables hash matching (legacy recipes without scene hashes).
func newestSceneRaw(workDir, runID, sceneID, sceneHash string) string {
	entries, err := os.ReadDir(workDir)
	if err != nil {
		return ""
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() && e.Name() != runID {
			dirs = append(dirs, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(dirs)))
	for _, d := range dirs {
		if sceneRaw(filepath.Join(workDir, d), sceneID) == "" {
			continue
		}
		if sceneHash == "" {
			return d
		}
		if sib := readRecipe(filepath.Join(workDir, d, "recipe.json")); sib != nil {
			for _, sc := range sib.Scenes {
				if sc.ID == sceneID && sc.SceneHash == sceneHash {
					return d
				}
			}
		}
	}
	return ""
}

func countEvents(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n := 0
	for _, line := range bytes.Split(data, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) > 0 {
			n++
		}
	}
	return n
}

func captureOf(workDir, runID string, rec *recipeFile) (Capture, Recovery) {
	c := Capture{Available: true, ScenesTotal: len(rec.Scenes)}
	r := Recovery{Available: true, ManualNote: "manual interventions are recorded in the dogfood log, not inferred"}
	runDir := filepath.Join(workDir, runID)
	for _, sc := range rec.Scenes {
		c.EventsTotal += countEvents(filepath.Join(runDir, "events-"+sc.ID+".jsonl"))
		here := sceneRaw(runDir, sc.ID) != ""
		// Mirror the pipeline's scene-level reuse: a sibling raw counts
		// when its scene hash matches, even if the storyboard hash moved
		// (e.g. a locator fix in another scene).
		older := newestSceneRaw(workDir, runID, sc.ID, sc.SceneHash)
		switch {
		case here && older != "":
			c.Retaken++
			c.Recorded++
			c.RecordedIDs = append(c.RecordedIDs, sc.ID)
			r.RetakenScenes = append(r.RetakenScenes, sc.ID)
		case here:
			c.Recorded++
			c.RecordedIDs = append(c.RecordedIDs, sc.ID)
		case older != "":
			c.Reused++
			c.ReusedIDs = append(c.ReusedIDs, sc.ID)
		default:
			c.Missing++
			r.FailedScenes = append(r.FailedScenes, sc.ID)
		}
	}
	if c.ScenesTotal > 0 {
		c.ReuseRatio = float64(c.Reused) / float64(c.ScenesTotal)
	}
	sort.Strings(c.RecordedIDs)
	sort.Strings(c.ReusedIDs)
	sort.Strings(r.FailedScenes)
	sort.Strings(r.RetakenScenes)
	if r.FailedScenes == nil {
		r.FailedScenes = []string{}
	}
	if r.RetakenScenes == nil {
		r.RetakenScenes = []string{}
	}
	if c.RecordedIDs == nil {
		c.RecordedIDs = []string{}
	}
	if c.ReusedIDs == nil {
		c.ReusedIDs = []string{}
	}
	return c, r
}

func syncOf(fin *finalTimelineFile) Sync {
	s := Sync{Available: true, Pass: fin.Sync.Pass, MaxDriftMs: fin.Sync.MaxDriftMs}
	var sum float64
	for _, it := range fin.Sync.Items {
		sum += abs(it.DriftMs)
		if !it.Pass {
			s.OutsideThreshold++
		}
	}
	if n := len(fin.Sync.Items); n > 0 {
		s.MeanDriftMs = sum / float64(n)
	}
	return s
}

func cinematicOf(runDir string) Cinematic {
	c := Cinematic{}
	data, err := os.ReadFile(filepath.Join(runDir, "cinematic_report.json"))
	if err != nil {
		return c
	}
	var rep cinematicReportFile
	if err := json.Unmarshal(data, &rep); err != nil {
		return c
	}
	c.Available = true
	c.QAPass = rep.Pass
	c.QAScore = rep.Score
	// Directive compliance lives beside the director bundle when present.
	// (scene_plan.json carries it when the director ran).
	if raw, err := os.ReadFile(filepath.Join(runDir, "scene_plan.json")); err == nil {
		var doc struct {
			Compliance *complianceFile `json:"directive_compliance"`
		}
		if json.Unmarshal(raw, &doc) == nil && doc.Compliance != nil {
			c.Requested = doc.Compliance.Requested
			c.Honored = doc.Compliance.Honored
			c.Ignored = doc.Compliance.Ignored
		}
		_ = raw
	}
	return c
}

type probeStream struct {
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	AvgFrameRate string `json:"avg_frame_rate"`
}
type probeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
}
type probeOut struct {
	Streams []probeStream `json:"streams"`
	Format  probeFormat   `json:"format"`
}

func outputOf(mp4 string) Output {
	st, err := os.Stat(mp4)
	if err != nil {
		return Output{}
	}
	o := Output{Available: true, Bytes: st.Size()}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return o
	}
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_name,width,height,avg_frame_rate",
		"-show_entries", "format=duration,size",
		"-of", "json", mp4)
	raw, err := cmd.Output()
	if err != nil {
		return o
	}
	var p probeOut
	if err := json.Unmarshal(raw, &p); err != nil {
		return o
	}
	if len(p.Streams) > 0 {
		o.VideoCodec = p.Streams[0].CodecName
		o.Width = p.Streams[0].Width
		o.Height = p.Streams[0].Height
		o.FPS = parseFPS(p.Streams[0].AvgFrameRate)
	}
	if d, err := strconv.ParseFloat(strings.TrimSpace(p.Format.Duration), 64); err == nil {
		o.DurationS = d
	}
	o.AudioCodec = probeAudioCodec(mp4)
	return o
}

func probeAudioCodec(mp4 string) string {
	cmd := exec.Command("ffprobe", "-v", "error",
		"-select_streams", "a:0",
		"-show_entries", "stream=codec_name",
		"-of", "csv=p=0", mp4)
	raw, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(raw))
}

func parseFPS(s string) float64 {
	parts := strings.Split(s, "/")
	if len(parts) != 2 {
		f, _ := strconv.ParseFloat(s, 64)
		return f
	}
	num, err1 := strconv.ParseFloat(parts[0], 64)
	den, err2 := strconv.ParseFloat(parts[1], 64)
	if err1 != nil || err2 != nil || den == 0 {
		return 0
	}
	return num / den
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// WriteJSON persists the report deterministically (struct field order,
// sorted slices at collect time).
func (r *Report) WriteJSON(path string) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// RenderText renders the §15-style human summary. Only available
// sections print metrics; unavailable ones print a one-line note.
func (r *Report) RenderText() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Tutorial: %s\n", r.Tutorial)
	fmt.Fprintf(&b, "AutoDoc: %s  run: %s\n", r.AutodocVersion, r.RunID)
	if r.StoryboardHash != "" {
		fmt.Fprintf(&b, "Storyboard: %s\n", r.StoryboardHash)
	}
	b.WriteString("\nScenes\n")
	if r.Pipeline.Available {
		fmt.Fprintf(&b, "  total: %d  beats: %d  actions: %d  waits: %d  speech: %d\n",
			r.Pipeline.Scenes, r.Pipeline.Beats, r.Pipeline.Actions, r.Pipeline.Waits, r.Pipeline.Speech)
	} else {
		b.WriteString("  no recipe yet\n")
	}
	b.WriteString("\nTTS\n")
	if r.TTS.Available {
		fmt.Fprintf(&b, "  segments: %d  hits: %d  misses: %d  hit ratio: %.1f%%  synthesized: %.1fs\n",
			r.TTS.Segments, r.TTS.Hits, r.TTS.Misses, r.TTS.HitRatio*100, r.TTS.SynthSecs)
	} else {
		b.WriteString("  no tts report yet (run tts)\n")
	}
	b.WriteString("\nCapture\n")
	if r.Capture.Available {
		fmt.Fprintf(&b, "  recorded: %d  reused: %d  retaken: %d  missing: %d  reuse ratio: %.1f%%\n",
			r.Capture.Recorded, r.Capture.Reused, r.Capture.Retaken, r.Capture.Missing, r.Capture.ReuseRatio*100)
	} else {
		b.WriteString("  no capture yet\n")
	}
	b.WriteString("\nSync\n")
	if r.Sync.Available {
		fmt.Fprintf(&b, "  max drift: %.0f ms  mean drift: %.0f ms  outside threshold: %d  pass: %v\n",
			r.Sync.MaxDriftMs, r.Sync.MeanDriftMs, r.Sync.OutsideThreshold, r.Sync.Pass)
	} else {
		b.WriteString("  no final timeline yet (run record)\n")
	}
	b.WriteString("\nRecovery\n")
	fmt.Fprintf(&b, "  failed scenes: %d  retaken: %d\n", len(r.Recovery.FailedScenes), len(r.Recovery.RetakenScenes))
	b.WriteString("\nSecurity\n")
	fmt.Fprintf(&b, "  secret scan: %s  redact selectors: %d  mask password: %v\n",
		r.Security.SecretScan, r.Security.RedactSelectors, r.Security.MaskPassword)
	if r.Cinematic.Available {
		b.WriteString("\nDirection QA\n")
		fmt.Fprintf(&b, "  requested: %d  honored: %d  ignored: %d  qa pass: %v  score: %d\n",
			r.Cinematic.Requested, r.Cinematic.Honored, r.Cinematic.Ignored, r.Cinematic.QAPass, r.Cinematic.QAScore)
	}
	if r.Output.Available {
		b.WriteString("\nOutput\n")
		fmt.Fprintf(&b, "  duration: %.1fs  %dx%d@%.0ffps  %s+%s  bytes: %d\n",
			r.Output.DurationS, r.Output.Width, r.Output.Height, r.Output.FPS,
			r.Output.VideoCodec, r.Output.AudioCodec, r.Output.Bytes)
	}
	return b.String()
}

// Row is one comparison line between two runs.
type Row struct {
	Metric string
	Before string
	After  string
	Delta  string
}

// Compare highlights only useful differences. Metrics unavailable in
// either run render as "n/a" and never invent direction semantics.
func Compare(a, b *Report) []Row {
	rows := []Row{
		{"scenes", num(a.Pipeline.Available, float64(a.Pipeline.Scenes)), num(b.Pipeline.Available, float64(b.Pipeline.Scenes)), deltaInt(a.Pipeline.Available && b.Pipeline.Available, a.Pipeline.Scenes, b.Pipeline.Scenes)},
		{"speech segments", num(a.Pipeline.Available, float64(a.Pipeline.Speech)), num(b.Pipeline.Available, float64(b.Pipeline.Speech)), deltaInt(a.Pipeline.Available && b.Pipeline.Available, a.Pipeline.Speech, b.Pipeline.Speech)},
		{"TTS hit ratio", pct(a.TTS.Available, a.TTS.HitRatio), pct(b.TTS.Available, b.TTS.HitRatio), deltaPP(a.TTS.Available && b.TTS.Available, a.TTS.HitRatio, b.TTS.HitRatio)},
		{"capture reuse", pct(a.Capture.Available, a.Capture.ReuseRatio), pct(b.Capture.Available, b.Capture.ReuseRatio), deltaPP(a.Capture.Available && b.Capture.Available, a.Capture.ReuseRatio, b.Capture.ReuseRatio)},
		{"retakes", num(a.Capture.Available, float64(a.Capture.Retaken)), num(b.Capture.Available, float64(b.Capture.Retaken)), deltaInt(a.Capture.Available && b.Capture.Available, a.Capture.Retaken, b.Capture.Retaken)},
		{"max drift", ms(a.Sync.Available, a.Sync.MaxDriftMs), ms(b.Sync.Available, b.Sync.MaxDriftMs), deltaMs(a.Sync.Available && b.Sync.Available, a.Sync.MaxDriftMs, b.Sync.MaxDriftMs)},
		{"outside threshold", num(a.Sync.Available, float64(a.Sync.OutsideThreshold)), num(b.Sync.Available, float64(b.Sync.OutsideThreshold)), deltaInt(a.Sync.Available && b.Sync.Available, a.Sync.OutsideThreshold, b.Sync.OutsideThreshold)},
		{"output duration", dur(a.Output.Available, a.Output.DurationS), dur(b.Output.Available, b.Output.DurationS), deltaDur(a.Output.Available && b.Output.Available, a.Output.DurationS, b.Output.DurationS)},
	}
	return rows
}

func num(ok bool, v float64) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.0f", v)
}

func pct(ok bool, v float64) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", v*100)
}

func ms(ok bool, v float64) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.0fms", v)
}

func dur(ok bool, v float64) string {
	if !ok {
		return "n/a"
	}
	return fmt.Sprintf("%.1fs", v)
}

func deltaInt(ok bool, a, b int) string {
	if !ok {
		return "—"
	}
	d := b - a
	if d == 0 {
		return "0"
	}
	return fmt.Sprintf("%+d", d)
}

func deltaPP(ok bool, a, b float64) string {
	if !ok {
		return "—"
	}
	return fmt.Sprintf("%+.1fpp", (b-a)*100)
}

func deltaMs(ok bool, a, b float64) string {
	if !ok {
		return "—"
	}
	return fmt.Sprintf("%+.0fms", b-a)
}

func deltaDur(ok bool, a, b float64) string {
	if !ok {
		return "—"
	}
	return fmt.Sprintf("%+.1fs", b-a)
}

// RenderCompare renders rows as an aligned table.
func RenderCompare(rows []Row) string {
	w := len("Metric")
	for _, r := range rows {
		if len(r.Metric) > w {
			w = len(r.Metric)
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-*s  %-10s  %-10s  %s\n", w, "Metric", "Before", "After", "Delta")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-*s  %-10s  %-10s  %s\n", w, r.Metric, r.Before, r.After, r.Delta)
	}
	return b.String()
}
