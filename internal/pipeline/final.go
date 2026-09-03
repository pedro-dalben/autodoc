package pipeline

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/pedro-dalben/autodoc/internal/capture"
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
	ft := timeline.Reconcile(r.Timeline, r.Recipe, sceneEvents, rawDur, timeline.DefaultReconcileOpts())
	r.Final = ft
	out := filepath.Join(r.WorkDir, r.RunID, "final_timeline.json")
	if err := ft.WriteJSON(out); err != nil {
		return nil, err
	}
	_ = ft.WriteJSON(filepath.Join(r.WorkDir, "final_timeline.json"))
	return ft, nil
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
