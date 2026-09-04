package doctor

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type DiagnosisReport struct {
	Healthy     bool     `json:"healthy"`
	System      *Report  `json:"system"`
	Workspace   Section  `json:"workspace"`
	Run         *Section `json:"latest_run,omitempty"`
	Remediation []string `json:"remediation,omitempty"`
}

func Diagnose(root string) *DiagnosisReport {
	sys := Run()
	diag := &DiagnosisReport{
		Healthy:   !sys.Failed(),
		System:    sys,
		Workspace: Section{Name: "Workspace & Project"},
	}

	// Check autodoc.toml
	cfgPath := filepath.Join(root, "autodoc.toml")
	if _, err := os.Stat(cfgPath); err == nil {
		diag.Workspace.Checks = append(diag.Workspace.Checks, ok("autodoc.toml", "present"))
	} else {
		diag.Workspace.Checks = append(diag.Workspace.Checks, warn("autodoc.toml", "missing (run autodoc init)"))
		diag.Remediation = append(diag.Remediation, "Run `autodoc init` to configure the project.")
	}

	// Check storyboard
	sbFound := false
	for _, cand := range []string{"storyboard.yml", "storyboard.yaml"} {
		if _, err := os.Stat(filepath.Join(root, cand)); err == nil {
			diag.Workspace.Checks = append(diag.Workspace.Checks, ok("storyboard", cand))
			sbFound = true
			break
		}
	}
	if !sbFound {
		diag.Workspace.Checks = append(diag.Workspace.Checks, warn("storyboard", "no storyboard.yml found"))
	}

	// Check latest run
	workDir := filepath.Join(root, ".autodoc", "_work")
	if entries, err := os.ReadDir(workDir); err == nil {
		var runs []string
		for _, e := range entries {
			if e.IsDir() && !strings.HasPrefix(e.Name(), ".") && e.Name() != "screenshots" {
				runs = append(runs, e.Name())
			}
		}
		sort.Strings(runs)
		if len(runs) > 0 {
			latest := filepath.Join(workDir, runs[len(runs)-1])
			for i := len(runs) - 1; i >= 0; i-- {
				dir := filepath.Join(workDir, runs[i])
				if _, err := os.Stat(filepath.Join(dir, "tutorial.mp4")); err == nil {
					latest = dir
					break
				}
			}
			rSec := Section{Name: fmt.Sprintf("Latest Run (%s)", filepath.Base(latest))}

			// Check tutorial.mp4
			if _, err := os.Stat(filepath.Join(latest, "tutorial.mp4")); err == nil {
				rSec.Checks = append(rSec.Checks, ok("video", "tutorial.mp4 present"))
			} else {
				rSec.Checks = append(rSec.Checks, warn("video", "tutorial.mp4 not rendered"))
			}

			// Check QA report
			qaPath := filepath.Join(latest, "cinematic_report.json")
			if _, err := os.Stat(qaPath); err != nil {
				qaPath = filepath.Join(latest, "qa_report.json")
			}
			if _, err := os.Stat(qaPath); err != nil {
				for _, rName := range runs {
					dir := filepath.Join(workDir, rName)
					for _, cand := range []string{"cinematic_report.json", "qa_report.json"} {
						p := filepath.Join(dir, cand)
						if _, err := os.Stat(p); err == nil {
							qaPath = p
							break
						}
					}
				}
			}
			if b, err := os.ReadFile(qaPath); err == nil {
				var doc struct {
					Passed      bool     `json:"passed"`
					Pass        bool     `json:"pass"`
					FatalErrors []string `json:"fatal_errors,omitempty"`
					Warnings    []string `json:"warnings,omitempty"`
					MaxDriftMs  float64  `json:"max_drift_ms,omitempty"`
				}
				if json.Unmarshal(b, &doc) == nil {
					passed := doc.Passed || doc.Pass
					if passed {
						rSec.Checks = append(rSec.Checks, ok("qa_report", "cinematic QA passed"))
					} else {
						rSec.Checks = append(rSec.Checks, fail("qa_report", strings.Join(doc.FatalErrors, "; ")))
						diag.Healthy = false
						diag.Remediation = append(diag.Remediation, "Review QA report failures in "+qaPath)
					}
					if doc.MaxDriftMs > 300 {
						rSec.Checks = append(rSec.Checks, warn("sync_drift", fmt.Sprintf("%.1fms drift exceeds 300ms", doc.MaxDriftMs)))
						diag.Remediation = append(diag.Remediation, "A/V drift detected: adjust pacing or review action timestamps.")
					}
				}
			}

			diag.Run = &rSec
		}
	}

	return diag
}

func (d *DiagnosisReport) Print(w io.Writer) {
	fmt.Fprintln(w, "# AutoDoc Self-Diagnosis")
	fmt.Fprintln(w)
	d.System.Print(w)
	d.Workspace.Print(w)
	fmt.Fprintln(w)
	if d.Run != nil {
		d.Run.Print(w)
		fmt.Fprintln(w)
	}
	if len(d.Remediation) > 0 {
		fmt.Fprintln(w, "## Recommended Actions")
		for _, r := range d.Remediation {
			fmt.Fprintf(w, "- %s\n", r)
		}
		fmt.Fprintln(w)
	}
	if d.Healthy {
		fmt.Fprintln(w, "Overall Status: HEALTHY ✓")
	} else {
		fmt.Fprintln(w, "Overall Status: ISSUES DETECTED ✗")
	}
}
