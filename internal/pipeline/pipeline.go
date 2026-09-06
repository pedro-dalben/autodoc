package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/capture"
	"github.com/pedro-dalben/autodoc/internal/cinematic"
	"github.com/pedro-dalben/autodoc/internal/config"
	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/timeline"
	"github.com/pedro-dalben/autodoc/internal/tts"
)

type Run struct {
	Root       string
	Storyboard string
	Config     *config.Config
	SB         *storyboard.Storyboard
	Recipe     *recipe.Recipe
	Timeline   *timeline.Timeline
	Final      *timeline.FinalTimeline
	Cinematic  *cinematic.Bundle
	WorkDir    string
	RunID      string
}

func NewRun(root, storyboardPath string, cfg *config.Config) (*Run, error) {
	sb, err := storyboard.LoadFile(storyboardPath)
	if err != nil {
		return nil, err
	}
	r := &Run{Root: root, Storyboard: storyboardPath, Config: cfg, SB: sb}
	r.WorkDir = filepath.Join(root, cfg.Paths.WorkDir)
	if r.WorkDir == "" {
		r.WorkDir = filepath.Join(root, ".autodoc", "_work")
	}
	r.RunID = time.Now().UTC().Format("20060102-150405")
	return r, nil
}

func (r *Run) Compile() error {
	cfg := r.Config
	r.Recipe = recipe.Compile(r.SB, cfg.TTS.Model, cfg.TTS.Voice, cfg.TTS.Language, cfg.TTS.Speed)
	recipePath := filepath.Join(r.WorkDir, r.RunID, "recipe.json")
	if err := r.Recipe.WriteJSON(recipePath); err != nil {
		return err
	}
	latest := filepath.Join(r.WorkDir, "recipe.json")
	_ = r.Recipe.WriteJSON(latest)
	return ScanSecrets(r.SB)
}

func ScanSecrets(sb *storyboard.Storyboard) error {
	var hits []string
	check := func(where, text string) {
		for _, s := range storyboard.SecretSentinels(text) {
			hits = append(hits, fmt.Sprintf("%s: possible secret marker %q", where, s))
		}
	}
	for _, sc := range sb.Scenes {
		for _, b := range sc.Beats {
			for i, ev := range b.Sequence {
				where := fmt.Sprintf("%s/%s#%d", sc.ID, b.ID, i)
				if ev.Speech != nil {
					check(where, ev.Speech.Text)
				}
				if ev.Action != nil {
					check(where, ev.Action.Value+" "+ev.Action.Text)
					if ev.Action.Type == "fill" || ev.Action.Type == "type" {
						if ev.Action.SecretRef != "" {
							continue
						}
						v := ev.Action.Value
						if v == "" {
							v = ev.Action.Text
						}
						if looksLikeSecret(v) {
							hits = append(hits, fmt.Sprintf("%s: fill value looks like a credential; use secret_ref or fixture", where))
						}
					}
				}
			}
		}
	}
	for i, st := range sb.Setup.Sequence {
		where := fmt.Sprintf("setup.sequence[%d]", i)
		if st.Action != nil {
			check(where, st.Action.Value+" "+st.Action.Text)
			if st.Action.SecretRef == "" {
				v := st.Action.Value
				if v == "" {
					v = st.Action.Text
				}
				if looksLikeSecret(v) {
					hits = append(hits, fmt.Sprintf("%s: setup value looks like a credential; use secret_ref", where))
				}
			}
		}
	}
	if len(hits) > 0 {
		return fmt.Errorf("secret scan found %d issue(s):\n  - %s", len(hits), strings.Join(hits, "\n  - "))
	}
	return nil
}

func looksLikeSecret(v string) bool {
	if len(v) >= 20 && !strings.Contains(v, " ") {
		hasUpper, hasLower, hasDigit := false, false, false
		for _, c := range v {
			switch {
			case c >= 'A' && c <= 'Z':
				hasUpper = true
			case c >= 'a' && c <= 'z':
				hasLower = true
			case c >= '0' && c <= '9':
				hasDigit = true
			}
		}
		kinds := 0
		if hasUpper {
			kinds++
		}
		if hasLower {
			kinds++
		}
		if hasDigit {
			kinds++
		}
		if kinds >= 2 {
			return true
		}
	}
	for _, p := range []string{"ghp_", "gho_", "sk-live", "sk-test", "xoxb-", "AKIA"} {
		if strings.Contains(v, p) {
			return true
		}
	}
	return false
}

type TTSReport struct {
	Segments int
	Hits     int
	Misses   int
	Results  []tts.Result
}

func (r *Run) SynthesizeTTS(ctx context.Context, provider tts.Provider, cache *tts.Cache) (*TTSReport, error) {
	cfg := r.Config
	audioDir := filepath.Join(r.WorkDir, r.RunID, "audio")
	cacheDir := filepath.Join(r.Root, ".autodoc", "cache", "tts")
	if cache == nil {
		cache = &tts.Cache{Dir: cacheDir}
	}
	rep := &TTSReport{}
	durations := map[string]float64{}
	for _, seg := range r.Recipe.SpeechSegments {
		voice := seg.Voice
		if voice == "" {
			voice = cfg.TTS.Voice
		}
		format := cfg.TTS.ResponseFormat
		if format == "" {
			format = "wav"
		}
		hash := tts.CacheKey(seg.Hash, provider.Name(), format)
		rep.Segments++
		var wav []byte
		var wavPath string
		cached := false
		if p, ok := cache.Lookup(hash); ok {
			wavPath = p
			cached = true
			rep.Hits++
		} else {
			req := tts.Request{Text: seg.Text, Voice: voice, Model: cfg.TTS.Model, Language: cfg.TTS.Language, Speed: cfg.TTS.Speed, Format: format}
			var err error
			wav, err = provider.Synthesize(ctx, req)
			if err != nil {
				return rep, fmt.Errorf("tts segment %s: %w", seg.ID, err)
			}
			wavPath, err = cache.Store(hash, wav)
			if err != nil {
				return rep, err
			}
			rep.Misses++
		}
		data, err := os.ReadFile(wavPath)
		if err != nil {
			return rep, err
		}
		dur, err := tts.WavDurationSeconds(data)
		if err != nil {
			if p, err2 := media.WavDuration(wavPath); err2 == nil {
				dur = p
			} else {
				dur = tts.EstimateDuration(seg.Text, cfg.TTS.Speed)
			}
		}
		durations[seg.ID] = dur
		if err := os.MkdirAll(audioDir, 0o755); err != nil {
			return rep, err
		}
		link := filepath.Join(audioDir, seg.ID+".wav")
		_ = os.Remove(link)
		_ = os.WriteFile(link, data, 0o644)
		rep.Results = append(rep.Results, tts.Result{ID: seg.ID, Text: seg.Text, Hash: hash, Path: link, Cached: cached, Duration: dur, Bytes: int64(len(data))})
	}
	tl, err := timeline.Build(r.Recipe, func(id string) (float64, bool, string) {
		for _, res := range rep.Results {
			if res.ID == id {
				return res.Duration, res.Cached, res.Path
			}
		}
		return 0, false, ""
	}, 600)
	if err != nil {
		return rep, err
	}
	r.Timeline = tl
	tlPath := filepath.Join(r.WorkDir, r.RunID, "timeline.json")
	if err := tl.WriteJSON(tlPath); err != nil {
		return rep, err
	}
	_ = tl.WriteJSON(filepath.Join(r.WorkDir, "timeline.json"))
	writeTTSReport(filepath.Join(r.WorkDir, r.RunID, "tts-report.json"), rep)
	return rep, nil
}

// ttsReportSegment is the privacy-safe per-segment record: ID, content
// hash, cache status and duration. Speech text is never persisted here;
// the run evidence report aggregates this file.
func writeTTSReport(path string, rep *TTSReport) {
	type seg struct {
		ID       string  `json:"id"`
		Hash     string  `json:"hash"`
		Cached   bool    `json:"cached"`
		Duration float64 `json:"duration_s"`
		Bytes    int64   `json:"bytes"`
	}
	out := struct {
		Segments  int     `json:"segments_total"`
		Hits      int     `json:"cache_hits"`
		Misses    int     `json:"cache_misses"`
		SynthSecs float64 `json:"synthesized_duration_s"`
		Segments_ []seg   `json:"segments"`
	}{Segments: rep.Segments, Hits: rep.Hits, Misses: rep.Misses}
	for _, res := range rep.Results {
		out.Segments_ = append(out.Segments_, seg{ID: res.ID, Hash: res.Hash, Cached: res.Cached, Duration: res.Duration, Bytes: res.Bytes})
		if !res.Cached {
			out.SynthSecs += res.Duration
		}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(data, '\n'), 0o644)
}

func (r *Run) LoadTimeline() error {
	for _, p := range []string{filepath.Join(r.WorkDir, r.RunID, "timeline.json"), filepath.Join(r.WorkDir, "timeline.json")} {
		if tl, err := timeline.LoadJSON(p); err == nil {
			if tl.StoryboardHash == r.Recipe.StoryboardHash {
				r.Timeline = tl
				return nil
			}
		}
	}
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "timeline.json"))
	var best *timeline.Timeline
	var bestPath string
	for _, p := range entries {
		if tl, err := timeline.LoadJSON(p); err == nil && tl.StoryboardHash == r.Recipe.StoryboardHash {
			if best == nil || p > bestPath {
				best = tl
				bestPath = p
			}
		}
	}
	if best != nil {
		r.Timeline = best
		return nil
	}
	return fmt.Errorf("no timeline found; run tts first")
}

func (r *Run) LatestRunDirWithHash() string {
	// Most recent dir whose recipe.json matches this run's storyboard hash.
	// Glob returns alphabetical (oldest-first for timestamped RunIDs) order,
	// so iterate from the end and skip the current run dir.
	current := filepath.Join(r.WorkDir, r.RunID)
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "recipe.json"))
	sort.Strings(entries)
	for i := len(entries) - 1; i >= 0; i-- {
		p := entries[i]
		if filepath.Dir(p) == current {
			continue
		}
		if rec, err := recipe.LoadJSON(p); err == nil && rec.StoryboardHash == r.Recipe.StoryboardHash {
			return filepath.Dir(p)
		}
	}
	return ""
}

// LatestRunDirWithSceneVideo returns the most recent run dir (excluding the
// current run) whose recipe.json matches this run's storyboard hash (or
// matches the individual scene's SceneHash) AND that contains raw/<sceneID>.webm
// (or .mp4). Empty string when none exists.
func (r *Run) LatestRunDirWithSceneVideo(sceneID string) string {
	current := filepath.Join(r.WorkDir, r.RunID)
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "recipe.json"))
	sort.Strings(entries)
	wantScene := r.Recipe.FindScene(sceneID)
	for i := len(entries) - 1; i >= 0; i-- {
		dir := filepath.Dir(entries[i])
		if dir == current {
			continue
		}
		rec, err := recipe.LoadJSON(entries[i])
		if err != nil {
			continue
		}
		matches := rec.StoryboardHash == r.Recipe.StoryboardHash
		if !matches && wantScene != nil && wantScene.SceneHash != "" {
			if otherSc := rec.FindScene(sceneID); otherSc != nil && otherSc.SceneHash == wantScene.SceneHash {
				matches = true
			}
		}
		if !matches {
			continue
		}
		for _, ext := range []string{".webm", ".mp4"} {
			if st, err := os.Stat(filepath.Join(dir, "raw", sceneID+ext)); err == nil && !st.IsDir() {
				return dir
			}
		}
	}
	return ""
}

// FindSceneVideo locates the raw capture for sceneID: current run dir first,
// then the most recent same-hash run dir containing it. Second return value
// is the run dir that provided the file ("" when not found).
func (r *Run) FindSceneVideo(sceneID string) (string, string) {
	for _, ext := range []string{".webm", ".mp4"} {
		if p := filepath.Join(r.WorkDir, r.RunID, "raw", sceneID+ext); fileExists(p) {
			return p, filepath.Join(r.WorkDir, r.RunID)
		}
	}
	if dir := r.LatestRunDirWithSceneVideo(sceneID); dir != "" {
		for _, ext := range []string{".webm", ".mp4"} {
			if p := filepath.Join(dir, "raw", sceneID+ext); fileExists(p) {
				return p, dir
			}
		}
	}
	return "", ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

func (r *Run) RecordScene(ctx context.Context, sceneID string, backendName string, headless bool, onEvent func(kind, label string)) (capture.Artifact, error) {
	_ = ctx
	sc := r.Recipe.FindScene(sceneID)
	if sc == nil {
		return capture.Artifact{}, fmt.Errorf("scene %q not found", sceneID)
	}
	if r.Timeline == nil {
		if err := r.LoadTimeline(); err != nil {
			return capture.Artifact{}, err
		}
	}
	speechMs := map[string]int64{}
	for _, seg := range r.Timeline.Segments {
		speechMs[seg.SpeechID] = int64(seg.DurationS*1000 + 0.5)
	}
	setupState, err := r.ensureSetupState(headless)
	if err != nil {
		return capture.Artifact{}, err
	}
	be, err := capture.Get(backendName)
	if err != nil {
		return capture.Artifact{}, err
	}
	cfg := r.Config
	vw, vh := viewportOf(r)
	rawDir := filepath.Join(r.WorkDir, r.RunID, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return capture.Artifact{}, err
	}
	vis := r.SB.VisualsOrDefault()
	cine := r.SB.CinematicOrDefault()
	// Record-time direction (cursor/click/typing/spotlight capture intent).
	// Render-time fields never invalidate the raw capture.
	cinematic.ApplyDirectionToCapture(r.SB, &vis, &cine)
	vis.Cinematic = cine
	opts := capture.StartOptions{
		SceneID: sceneID, ViewportW: vw, ViewportH: vh, Headless: headless,
		VideoDir: rawDir, Redact: r.SB.Redact, Visuals: vis,
	}
	if r.SB.Config.BaseURL != "" {
		opts.BaseURL = r.SB.Config.BaseURL
	}
	if setupState != "" {
		opts.StorageState = setupState
	} else if cfg.Browser.StorageState != "" {
		opts.StorageState = absJoin(r.Root, cfg.Browser.StorageState)
	} else if r.SB.Setup.StorageState != "" {
		opts.StorageState = absJoin(r.Root, r.SB.Setup.StorageState)
	}
	if err := be.Start(opts); err != nil {
		return capture.Artifact{}, err
	}
	emit := func(k, l string) {
		if onEvent != nil {
			onEvent(k, l)
		}
	}
	defer be.Close()
	if r.SB.Setup.StartURL != "" || sc.URL != "" {
		startURL := sc.URL
		if startURL == "" {
			startURL = r.SB.Setup.StartURL
		}
		if err := be.Navigate(startURL); err != nil {
			return capture.Artifact{}, err
		}
		emit("navigate", startURL)
	}
	be.ShowChapter(sc.Title)
	type item struct {
		beat string
		st   recipe.StepPlan
	}
	var items []item
	for _, b := range sc.Beats {
		for _, st := range b.Steps {
			items = append(items, item{beat: b.ID, st: st})
		}
	}
	kindOf := func(it item) recipe.StepKind { return it.st.Kind }
	prevKind := recipe.StepKind("")
	for i, it := range items {
		var nextKind recipe.StepKind
		if i+1 < len(items) {
			nextKind = kindOf(items[i+1])
		}
		// Narration padding: intentional human-feeling gaps at speech
		// boundaries. The recorder sleeps here so the raw capture already
		// runs on the narration clock.
		if it.st.Kind == recipe.StepAction && prevKind == recipe.StepSpeech {
			if _, err := be.DoPause(int64(vis.Pacing.SpeechActionGapMs), "speech-action-gap"); err != nil {
				return capture.Artifact{}, err
			}
		}
		if it.st.Kind == recipe.StepSpeech && (prevKind == recipe.StepAction || prevKind == recipe.StepWait) {
			if _, err := be.DoPause(int64(vis.Pacing.ActionSpeechGapMs), "action-speech-gap"); err != nil {
				return capture.Artifact{}, err
			}
		}
		switch it.st.Kind {
		case recipe.StepAction:
			res, err := be.DoAction(*it.st.Action)
			_ = res
			if err != nil {
				return capture.Artifact{}, fmt.Errorf("scene %s action %s: %w", sceneID, it.st.Action.Type, err)
			}
			emit("action", it.st.Action.Type)
			// Action -> result -> confirmation: declared expected
			// results are waited on, highlighted and held on camera.
			if it.st.Action.ResultTarget != nil && !it.st.Action.ResultTarget.Empty() {
				holdMs := cine.Results.MinHoldMs
				if it.st.Action.ResultHoldMs != nil && *it.st.Action.ResultHoldMs > 0 {
					holdMs = *it.st.Action.ResultHoldMs
				}
				be.ConfirmResult(it.st.Action.ResultTarget, holdMs, "result-of:"+it.st.Action.Type)
				emit("result", "result-of:"+it.st.Action.Type)
			}
		case recipe.StepWait:
			if _, err := be.DoWait(*it.st.Wait); err != nil {
				return capture.Artifact{}, err
			}
			emit("wait", it.st.Wait.State)
		case recipe.StepSpeech:
			// Authored pauses ride the narration clock without touching
			// the TTS text (pacing is orchestration, never rewriting).
			if it.st.PauseBeforeMs > 0 {
				if _, err := be.DoPause(int64(it.st.PauseBeforeMs), "pause-before-speech"); err != nil {
					return capture.Artifact{}, err
				}
			}
			ms, ok := speechMs[it.st.SpeechID]
			if !ok || ms <= 0 {
				ms = int64(tts.EstimateDuration(it.st.Text, cfg.TTS.Speed)*1000 + 0.5)
			}
			if _, err := be.DoSpeech(it.st.SpeechID, ms); err != nil {
				return capture.Artifact{}, err
			}
			emit("speech", it.st.SpeechID)
			if it.st.PauseAfterMs > 0 {
				if _, err := be.DoPause(int64(it.st.PauseAfterMs), "pause-after-speech"); err != nil {
					return capture.Artifact{}, err
				}
			}
		case recipe.StepHold:
			if _, err := be.DoHold(int64(it.st.HoldMs)); err != nil {
				return capture.Artifact{}, err
			}
			emit("hold", fmt.Sprintf("%dms", it.st.HoldMs))
		}
		_ = nextKind
		prevKind = it.st.Kind
	}
	if sc.Screenshot {
		shotsDir := filepath.Join(r.WorkDir, r.RunID, "screenshots")
		_ = os.MkdirAll(shotsDir, 0o755)
		shotPath := filepath.Join(shotsDir, sceneID+".png")
		if err := be.Screenshot(shotPath); err != nil {
			return capture.Artifact{}, err
		}
		latestShots := filepath.Join(r.WorkDir, "screenshots")
		_ = os.MkdirAll(latestShots, 0o755)
		if data, err := os.ReadFile(shotPath); err == nil {
			_ = os.WriteFile(filepath.Join(latestShots, sceneID+".png"), data, 0o644)
		}
	}
	art, err := be.Stop()
	if err != nil {
		return capture.Artifact{}, err
	}
	eventsPath := filepath.Join(r.WorkDir, r.RunID, "events-"+sceneID+".jsonl")
	writeEventsJSONL(eventsPath, art.Events)
	if _, err := r.ReconcileFinal(); err != nil {
		return art, fmt.Errorf("scene %s recorded but reconciliation failed: %w", sceneID, err)
	}
	return art, nil
}

func writeEventsJSONL(path string, events []capture.EventRecord) {
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	for _, e := range events {
		_ = enc.Encode(e)
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func absJoin(root, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(root, p)
}
