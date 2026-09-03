package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/capture"
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
						v := ev.Action.Value
						if v == "" {
							v = ev.Action.Text
						}
						if looksLikeSecret(v) {
							hits = append(hits, fmt.Sprintf("%s: fill value looks like a credential; use fixture or mask", where))
						}
					}
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
		voice := cfg.TTS.Voice
		hash := recipe.SpeechHash(seg.Text, voice, cfg.TTS.Model, cfg.TTS.Language, cfg.TTS.Speed)
		rep.Segments++
		var wav []byte
		var wavPath string
		cached := false
		if p, ok := cache.Lookup(hash); ok {
			wavPath = p
			cached = true
			rep.Hits++
		} else {
			req := tts.Request{Text: seg.Text, Voice: voice, Model: cfg.TTS.Model, Language: cfg.TTS.Language, Speed: cfg.TTS.Speed, Format: cfg.TTS.ResponseFormat}
			if req.Format == "" {
				req.Format = "wav"
			}
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
	return rep, nil
}

func (r *Run) LoadTimeline() error {
	for _, p := range []string{filepath.Join(r.WorkDir, r.RunID, "timeline.json"), filepath.Join(r.WorkDir, "timeline.json")} {
		if tl, err := timeline.LoadJSON(p); err == nil {
			if tl.StoryboardHash == r.Recipe.StoryboardHash || r.Recipe.StoryboardHash == "" {
				r.Timeline = tl
				return nil
			}
		}
	}
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "timeline.json"))
	var best *timeline.Timeline
	for _, p := range entries {
		if tl, err := timeline.LoadJSON(p); err == nil && tl.StoryboardHash == r.Recipe.StoryboardHash {
			if best == nil || tl.TotalS > best.TotalS {
				best = tl
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
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "recipe.json"))
	for _, p := range entries {
		if rec, err := recipe.LoadJSON(p); err == nil && rec.StoryboardHash == r.Recipe.StoryboardHash {
			return filepath.Dir(p)
		}
	}
	return ""
}

func (r *Run) RecordScene(ctx context.Context, sceneID string, backendName string, headless bool, onEvent func(kind, label string)) (capture.Artifact, error) {
	_ = ctx
	sc := r.Recipe.FindScene(sceneID)
	if sc == nil {
		return capture.Artifact{}, fmt.Errorf("scene %q not found", sceneID)
	}
	be, err := capture.Get(backendName)
	if err != nil {
		return capture.Artifact{}, err
	}
	cfg := r.Config
	vw, vh := cfg.Browser.ViewportW, cfg.Browser.ViewportH
	if vw == 0 {
		vw = r.SB.Config.ViewportW
	}
	if vh == 0 {
		vh = r.SB.Config.ViewportH
	}
	if vw == 0 {
		vw = 1280
	}
	if vh == 0 {
		vh = 720
	}
	baseURL := cfg.Browser.StorageState
	_ = baseURL
	rawDir := filepath.Join(r.WorkDir, r.RunID, "raw")
	if err := os.MkdirAll(rawDir, 0o755); err != nil {
		return capture.Artifact{}, err
	}
	opts := capture.StartOptions{
		SceneID: sceneID, ViewportW: vw, ViewportH: vh, Headless: headless,
		BaseURL:  firstNonEmpty(r.SB.Config.BaseURL, cfg.TTS.BaseURL),
		VideoDir: rawDir, Redact: r.SB.Redact,
	}
	if r.SB.Config.BaseURL != "" {
		opts.BaseURL = r.SB.Config.BaseURL
	}
	if cfg.Browser.StorageState != "" {
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
	for _, b := range sc.Beats {
		for _, st := range b.Steps {
			switch st.Kind {
			case recipe.StepAction:
				res, err := be.DoAction(*st.Action)
				_ = res
				if err != nil {
					return capture.Artifact{}, fmt.Errorf("scene %s action %s: %w", sceneID, st.Action.Type, err)
				}
				emit("action", st.Action.Type)
			case recipe.StepWait:
				if _, err := be.DoWait(*st.Wait); err != nil {
					return capture.Artifact{}, err
				}
				emit("wait", st.Wait.State)
			case recipe.StepSpeech, recipe.StepHold:
			}
		}
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
