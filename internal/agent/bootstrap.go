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
