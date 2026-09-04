package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

// Capsule is a compact, secret-free warm-run handoff. It records only stable
// targets and explicit source dependencies—not browser payloads or action
// values—so unrelated repository edits do not invalidate a tutorial.
type Capsule struct {
	Flow           string            `json:"flow"`
	StoryboardHash string            `json:"storyboard_hash"`
	Targets        map[string]string `json:"targets,omitempty"`
	SourceFiles    map[string]string `json:"source_files,omitempty"`
	CreatedAt      string            `json:"created_at"`
}

func NewCapsule(flow string, sb *storyboard.Storyboard, sourceFiles []string) (Capsule, error) {
	if flow == "" {
		return Capsule{}, fmt.Errorf("flow is required")
	}
	c := Capsule{Flow: flow, StoryboardHash: sb.SourceHash(), Targets: map[string]string{}, SourceFiles: map[string]string{}, CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	for _, sc := range sb.Scenes {
		for name, target := range sc.Targets {
			c.Targets[sc.ID+"."+name] = (&target).Describe()
		}
		for _, beat := range sc.Beats {
			for _, ev := range beat.Sequence {
				if ev.Action != nil && ev.Action.Target != nil {
					c.Targets[sc.ID+"."+beat.ID] = ev.Action.Target.Describe()
				}
			}
		}
	}
	for _, path := range sourceFiles {
		h, err := fileHash(path)
		if err != nil {
			return Capsule{}, err
		}
		c.SourceFiles[path] = h
	}
	return c, nil
}

func SaveCapsule(dir string, c Capsule) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, safeName(c.Flow)+".json")
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", err
	}
	return path, os.WriteFile(path, append(b, '\n'), 0o644)
}

func LoadCapsule(path string) (Capsule, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Capsule{}, err
	}
	var c Capsule
	if err := json.Unmarshal(b, &c); err != nil {
		return Capsule{}, err
	}
	return c, nil
}

// ChangedSources returns only dependencies whose bytes changed or disappeared.
// A README outside this explicit set cannot invalidate the capsule.
func (c Capsule) ChangedSources() []string {
	var changed []string
	for path, old := range c.SourceFiles {
		now, err := fileHash(path)
		if err != nil || now != old {
			changed = append(changed, path)
		}
	}
	sort.Strings(changed)
	return changed
}

func fileHash(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])[:16], nil
}

func safeName(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			return r
		}
		return '-'
	}, strings.ToLower(s))
	return strings.Trim(s, "-")
}
