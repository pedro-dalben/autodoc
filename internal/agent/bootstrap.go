// Package agent builds the smallest task-specific handoff an external agent
// needs. It deliberately contains no provider code: the agent reasons; AutoDoc
// supplies deterministic state and commands.
package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var guides = map[string]string{
	"discovery":  "discovery: artifacts first; narrow route search; ui query once; ui diff after one action; retrieve raw evidence only by ref\n",
	"storyboard": "storyboard: semantic ordered speech/action/wait/hold; stable locators; patch existing scenes; validate before TTS\n",
	"auth":       "auth: reuse dedicated AutoDoc state; bootstrap off-camera with fixture credentials; never record login\n",
	"recording":  "recording: run deterministic autodoc record; prefer scene retakes; never capture exploratory browser activity\n",
	"cinematic":  "cinematic: declare intent/result targets; leave cursor, camera, pacing, and holds to the director unless an override is necessary\n",
	"tts":        "tts: freeze narration before synthesis; unchanged segments are cache hits; keep provider keys in environment variables\n",
	"publishing": "publishing: validate sync and cinematic QA, then export; publish docs/autodoc only, never workspace/auth artifacts\n",
	"debug":      "debug: use the compact error code and evidence ref first; re-run the narrow failed stage before broad rediscovery\n",
	"security":   "security: fixture data only; no secrets, tokens, passwords, cookies, or unnecessary PII in storyboards, evidence, logs, or frames\n",
}

// Guide returns one on-demand operational module. It deliberately remains
// short: commands and deterministic validators hold the verbose detail.
func Guide(topic string) (string, bool) { text, ok := guides[strings.ToLower(topic)]; return text, ok }

// Mode is the narrow workflow required for a request.
type Mode string

const (
	Create  Mode = "CREATE"
	Update  Mode = "UPDATE"
	Retake  Mode = "RETAKE"
	Render  Mode = "RENDER"
	Debug   Mode = "DEBUG"
	Publish Mode = "PUBLISH"
)

// Classify maps a request to one task mode. It is intentionally conservative:
// ambiguous requests create or update a tutorial instead of guessing a retake.
func Classify(goal string, hasStoryboard bool) Mode {
	s := strings.ToLower(goal)
	for _, x := range []string{"publicar", "publish", "exportar", "export"} {
		if strings.Contains(s, x) {
			return Publish
		}
	}
	for _, x := range []string{"debug", "falha", "erro", "broken", "fix"} {
		if strings.Contains(s, x) {
			return Debug
		}
	}
	for _, x := range []string{"render", "renderizar", "gerar video", "gerar vídeo"} {
		if strings.Contains(s, x) {
			return Render
		}
	}
	for _, x := range []string{"retake", "regravar", "re-record", "regrava"} {
		if strings.Contains(s, x) {
			return Retake
		}
	}
	if hasStoryboard {
		return Update
	}
	return Create
}

// Bootstrap describes reusable local state and only the next deterministic
// steps. Paths are relative to root so output is portable and secret-free.
func Bootstrap(root, goal, storyboard string) string {
	if storyboard == "" {
		storyboard = "storyboard.yml"
	}
	hasStoryboard := exists(filepath.Join(root, storyboard))
	mode := Classify(goal, hasStoryboard)
	var lines []string
	lines = append(lines, "mode: "+string(mode))
	if hasStoryboard {
		lines = append(lines, "storyboard: "+storyboard)
	}
	if exists(filepath.Join(root, ".autodoc", "cache", "auth")) {
		lines = append(lines, "auth: cached")
	}
	if exists(filepath.Join(root, ".autodoc", "cache", "evidence", "index.jsonl")) {
		lines = append(lines, "evidence: cached (autodoc evidence stats)")
	}
	if exists(filepath.Join(root, ".autodoc", "_work")) {
		lines = append(lines, "workspace: prior run available")
	}
	lines = append(lines, "next:")
	switch mode {
	case Create:
		lines = append(lines, "- inspect existing artifacts, then autodoc ui query --url URL --intent GOAL", "- author semantic storyboard; validate, tts, record, render, validate --cinematic")
	case Update, Retake:
		lines = append(lines, "- inspect existing storyboard and git diff; patch only affected scene", "- verify target with autodoc ui query or ui diff; retake affected scene, render")
	case Render:
		lines = append(lines, "- autodoc render --storyboard "+storyboard, "- autodoc validate --storyboard "+storyboard+" --cinematic")
	case Publish:
		lines = append(lines, "- autodoc validate --storyboard "+storyboard+" --cinematic", "- autodoc export --storyboard "+storyboard)
	case Debug:
		lines = append(lines, "- run the failing narrow command; use evidence refs before full snapshots", "- fix the named scene/target, then validate")
	}
	return fmt.Sprintf("AutoDoc bootstrap\n%s\n", strings.Join(lines, "\n"))
}

func exists(path string) bool { _, err := os.Stat(path); return err == nil }
