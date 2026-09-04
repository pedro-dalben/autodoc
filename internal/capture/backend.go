package capture

import (
	"fmt"

	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
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
	Visuals      visual.Config
}

type ActionResult struct {
	AtMs      int64
	ElapsedMs int64
	Note      string
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
	// DoSpeech reserves the exact narration window in the capture so the
	// raw video already runs on the narration clock (monotonic events).
	DoSpeech(speechID string, durationMs int64) (ActionResult, error)
	// DoHold reserves a hold window in the capture.
	DoHold(durationMs int64) (ActionResult, error)
	// ConfirmResult waits for the declared expected result, highlights it
	// and holds it visible (action/result/confirmation). Never fails the
	// record; QA reports absent results.
	ConfirmResult(t *storyboard.Target, holdMs int, label string)
	// DoPause inserts a small intentional pacing gap (narration padding).
	DoPause(ms int64, label string) (ActionResult, error)
	// SaveStorageState snapshots cookies/localStorage for setup reuse.
	SaveStorageState(path string) error
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
