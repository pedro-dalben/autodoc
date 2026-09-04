package cinematic

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// SceneDirectorExplanation contains the explanations for one scene.
type SceneDirectorExplanation struct {
	SceneID   string              `json:"scene_id"`
	Camera    []CameraDecision    `json:"camera"`
	Attention []AttentionDecision `json:"attention"`
	HoldsMs   int                 `json:"holds_added_ms,omitempty"`
	Summary   string              `json:"summary"`
}

// ExplainRun reads the cinematic plan and recipe from runDir and explains
// the decisions made by the cinematic director for each scene.
func ExplainRun(runDir, sceneFilter string) ([]SceneDirectorExplanation, string, error) {
	var plan CinematicPlan
	planPath := filepath.Join(runDir, "cinematic_plan.json")
	if planData, err := os.ReadFile(planPath); err == nil {
		_ = json.Unmarshal(planData, &plan)
	}

	var rec *recipe.Recipe
	recPath := filepath.Join(runDir, "recipe.json")
	if rd, err := os.ReadFile(recPath); err == nil {
		var r recipe.Recipe
		if json.Unmarshal(rd, &r) == nil {
			rec = &r
		}
	}

	if plan.Attention == nil && plan.Camera == nil && rec == nil {
		return nil, "", fmt.Errorf("neither cinematic_plan.json nor recipe.json found in %s", runDir)
	}

	// Fallback: if cinematic_plan.json was not yet written, direct attention from recipe
	if plan.Attention == nil && rec != nil {
		scDoc := PlanScenes(rec, nil)
		plan.Attention = DirectAttention(scDoc.Beats, visual.DefaultCinematic())
	}

	// Group camera decisions by scene
	camByScene := map[string][]CameraDecision{}
	if plan.Camera != nil {
		for _, d := range plan.Camera.Decisions {
			camByScene[d.SceneID] = append(camByScene[d.SceneID], d)
		}
	}

	// Group attention decisions by scene
	attByScene := map[string][]AttentionDecision{}
	if plan.Attention != nil {
		for _, d := range plan.Attention.Decisions {
			// Find scene for this beat
			scID := d.SceneID
			if scID == "" && rec != nil {
				curIdx := 0
				for _, sc := range rec.Scenes {
					if d.BeatIndex >= curIdx && d.BeatIndex < curIdx+len(sc.Beats) {
						scID = sc.ID
						break
					}
					curIdx += len(sc.Beats)
				}
			}
			attByScene[scID] = append(attByScene[scID], d)
		}
	}

	// Collect all scene IDs
	sceneSet := map[string]bool{}
	for s := range camByScene {
		if s != "" {
			sceneSet[s] = true
		}
	}
	if rec != nil {
		for _, sc := range rec.Scenes {
			sceneSet[sc.ID] = true
		}
	}
	var scenes []string
	for s := range sceneSet {
		if sceneFilter == "" || s == sceneFilter {
			scenes = append(scenes, s)
		}
	}
	sort.Strings(scenes)

	var res []SceneDirectorExplanation
	var out strings.Builder

	fmt.Fprintf(&out, "Cinematic Director Explanation (run: %s)\n", filepath.Base(runDir))
	if plan.CameraAdjusted > 0 || plan.HoldsAddedMs > 0 {
		fmt.Fprintf(&out, "Global adjustments: %d camera segments modified, %dms result holds added\n\n",
			plan.CameraAdjusted, plan.HoldsAddedMs)
	}

	for _, sc := range scenes {
		cams := camByScene[sc]
		atts := attByScene[sc]
		exp := SceneDirectorExplanation{
			SceneID:   sc,
			Camera:    cams,
			Attention: atts,
		}

		fmt.Fprintf(&out, "=== Scene: %s ===\n", sc)
		if len(cams) > 0 {
			out.WriteString("  Camera Decisions:\n")
			for _, c := range cams {
				zoomStr := fmt.Sprintf("%.2fx", c.Zoom)
				if c.Zoom <= 1.01 {
					zoomStr = "1.00x (full)"
				}
				reversalStr := ""
				if c.Reversal {
					reversalStr = " [REVERSAL]"
				}
				if c.Suppressed {
					reversalStr = " [OSCILLATION SUPPRESSED]"
				}
				fmt.Fprintf(&out, "    [%s] %-6s move=%-14s zoom=%-11s reason=%s%s\n",
					sc, c.Kind, c.Move, zoomStr, c.Reason, reversalStr)
			}
		}
		if len(atts) > 0 {
			out.WriteString("  Attention Decisions:\n")
			for _, a := range atts {
				antic := ""
				if a.Anticipate {
					antic = " [anticipate]"
				}
				fmt.Fprintf(&out, "    beat %-2d strategy=%-14s reason=%s%s\n",
					a.BeatIndex, a.Strategy, a.Reason, antic)
			}
		}
		out.WriteString("\n")
		res = append(res, exp)
	}

	return res, out.String(), nil
}
