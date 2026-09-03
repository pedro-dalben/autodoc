package capture

import (
	"fmt"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

type StartOptions struct {
	SceneID      string
	ViewportW    int
	ViewportH    int
	Headless     bool
	BaseURL      string
	StorageState string
	ProfileDir   string
	VideoDir     string
	Redact       storyboard.Redact
}

type ActionResult struct {
	AtMs int64
	Note string
}

type Chapter struct {
	Title string
	AtMs  int64
}

type CaptureBackend interface {
	Start(opts StartOptions) error
	Navigate(url string) error
	ApplyRedaction(selectors []string, maskPasswords bool) error
	DoAction(a storyboard.Action) (ActionResult, error)
	DoWait(w storyboard.WaitEvent) (ActionResult, error)
	Screenshot(path string) error
	ShowAction(label string)
	ShowChapter(title string)
	Stop() (Artifact, error)
	Close() error
}

type Artifact struct {
	SceneID   string
	VideoPath string
	Events    []EventRecord
}

type EventRecord struct {
	Kind  string `json:"kind"`
	Label string `json:"label"`
	AtMs  int64  `json:"at_ms"`
	Note  string `json:"note,omitempty"`
}

var backends = map[string]func() CaptureBackend{}

func Register(name string, fn func() CaptureBackend) {
	backends[name] = fn
}

func Get(name string) (CaptureBackend, error) {
	fn, ok := backends[name]
	if !ok {
		return nil, fmt.Errorf("unknown capture backend %q", name)
	}
	return fn(), nil
}

func BackendNames() []string {
	out := make([]string, 0, len(backends))
	for k := range backends {
		out = append(out, k)
	}
	return out
}
