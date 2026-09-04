package pipeline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pedro-dalben/autodoc/internal/capture"
	"github.com/pedro-dalben/autodoc/internal/cinematic"
	"github.com/pedro-dalben/autodoc/internal/media"
	"github.com/pedro-dalben/autodoc/internal/timeline"
)

// ReconcileFinal merges the planned timeline with the monotonic capture
// events of every scene recorded so far (current run first, most recent
// same-hash run as fallback for retakes) and writes final_timeline.json.
// The renderer consumes the final timeline, never the plan alone.
func (r *Run) ReconcileFinal() (*timeline.FinalTimeline, error) {
	if r.Timeline == nil {
		if err := r.LoadTimeline(); err != nil {
			return nil, err
		}
	}
	sceneEvents, rawDur := r.collectSceneEvidence()
	ft := timeline.Reconcile(r.Timeline, r.Recipe, sceneEvents, rawDur, reconcileOpts(r))
	r.Final = ft
	// Cinematic V2 direction: semantic scene plan -> attention/camera ->
	// edit plan -> QA. Adjustments are sync-safe (camera zooms,
	// video-only result holds); the directed timeline is what renders.
	cine := r.SB.CinematicOrDefault()
	vis := r.SB.VisualsOrDefault()
	bundle := cinematic.Direct(r.Recipe, r.SB, ft, sceneEvents, vis, cine)
	r.Cinematic = bundle
	out := filepath.Join(r.WorkDir, r.RunID, "final_timeline.json")
	if err := ft.WriteJSON(out); err != nil {
		return nil, err
	}
	_ = ft.WriteJSON(filepath.Join(r.WorkDir, "final_timeline.json"))
	_ = cinematic.WriteBundle(filepath.Join(r.WorkDir, r.RunID), bundle)
	_ = cinematic.WriteBundle(r.WorkDir, bundle)
	return ft, nil
}

// reconcileOpts honors the cinematic editing budget (wait fast-forward
// cap) while keeping V1 defaults for storyboards without direction.
func reconcileOpts(r *Run) timeline.ReconcileOpts {
	opts := timeline.DefaultReconcileOpts()
	cine := r.SB.CinematicOrDefault()
	if cine.Editing.MaxSpeed > 0 {
		opts.WaitSpeed = cine.Editing.MaxSpeed
	}
	return opts
}

func (r *Run) collectSceneEvidence() (map[string][]timeline.ActualEvent, map[string]float64) {
	sceneEvents := map[string][]timeline.ActualEvent{}
	rawDur := map[string]float64{}
	for _, sc := range r.Recipe.Scenes {
		if evs := r.loadSceneEvents(sc.ID); len(evs) > 0 {
			sceneEvents[sc.ID] = timeline.ParseActualEvents(evs)
		}
		if p, _ := r.FindSceneVideo(sc.ID); p != "" {
			if pr, err := media.ProbeFile(p); err == nil && pr.Duration > 0 {
				rawDur[sc.ID] = pr.Duration
			}
		}
	}
	return sceneEvents, rawDur
}

// BuildCinematic rebuilds the V2 director bundle for the loaded final
// timeline (used by `validate --cinematic` on existing runs). The
// direction is idempotent: re-running over an already-directed timeline
// reproduces the same plans and report.
func (r *Run) BuildCinematic() (*cinematic.Bundle, error) {
	if r.Final == nil {
		if err := r.LoadFinal(); err != nil {
			return nil, err
		}
	}
	sceneEvents, _ := r.collectSceneEvidence()
	cine := r.SB.CinematicOrDefault()
	vis := r.SB.VisualsOrDefault()
	bundle := cinematic.Direct(r.Recipe, r.SB, r.Final, sceneEvents, vis, cine)
	r.Cinematic = bundle
	return bundle, nil
}

func (r *Run) loadSceneEvents(sceneID string) []capture.EventRecord {
	cands := []string{filepath.Join(r.WorkDir, r.RunID, "events-"+sceneID+".jsonl")}
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "events-"+sceneID+".jsonl"))
	sort.Strings(entries)
	current := filepath.Join(r.WorkDir, r.RunID, "events-"+sceneID+".jsonl")
	for i := len(entries) - 1; i >= 0; i-- {
		if entries[i] != current {
			cands = append(cands, entries[i])
		}
	}
	for _, p := range cands {
		if evs, err := readEventsJSONL(p); err == nil && len(evs) > 0 {
			return evs
		}
	}
	return nil
}

// LoadFinal loads the most recent final timeline matching this run's
// storyboard hash (current run first).
func (r *Run) LoadFinal() error {
	cands := []string{
		filepath.Join(r.WorkDir, r.RunID, "final_timeline.json"),
		filepath.Join(r.WorkDir, "final_timeline.json"),
	}
	entries, _ := filepath.Glob(filepath.Join(r.WorkDir, "*", "final_timeline.json"))
	sort.Strings(entries)
	for i := len(entries) - 1; i >= 0; i-- {
		cands = append(cands, entries[i])
	}
	for _, p := range cands {
		if ft, err := timeline.LoadFinalJSON(p); err == nil && ft.StoryboardHash == r.Recipe.StoryboardHash {
			r.Final = ft
			return nil
		}
	}
	return fmt.Errorf("no final timeline found; run record first")
}

func readEventsJSONL(path string) ([]capture.EventRecord, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []capture.EventRecord
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e capture.EventRecord
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, sc.Err()
}
