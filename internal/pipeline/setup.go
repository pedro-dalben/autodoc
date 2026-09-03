package pipeline

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pedro-dalben/autodoc/internal/capture"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

// setupCachePath keys the off-camera auth state by sequence content so a
// future record reuses a still-valid login without replaying credentials.
func (r *Run) setupCachePath() string {
	h := sha256.New()
	base := r.SB.Config.BaseURL
	fmt.Fprintf(h, "setup-v1|%s|%s|", base, r.SB.Setup.StartURL)
	for _, st := range r.SB.Setup.Sequence {
		switch {
		case st.Action != nil:
			t := ""
			if st.Action.Target != nil {
				t = st.Action.Target.PlaywrightSelector()
			}
			fmt.Fprintf(h, "a:%s|%s|%s|secret:%t|", st.Action.Type, t, st.Action.URL, st.Action.SecretRef != "")
		case st.Wait != nil:
			fmt.Fprintf(h, "w:%s|%s|%d|", st.Wait.State, st.Wait.URL, st.Wait.TimeoutMs)
		}
	}
	return filepath.Join(r.Root, ".autodoc", "cache", "auth", "setup-"+hex.EncodeToString(h.Sum(nil))[:16]+".json")
}

// BootstrapAuth runs the storyboard setup sequence off-camera and caches the
// resulting storage state. Recording reuses it; credentials never touch
// artifacts (secret_ref resolves from the environment at runtime).
func (r *Run) BootstrapAuth(headless bool) (string, error) {
	if len(r.SB.Setup.Sequence) == 0 {
		if r.SB.Setup.StorageState != "" {
			return absJoin(r.Root, r.SB.Setup.StorageState), nil
		}
		if r.Config.Browser.StorageState != "" {
			return absJoin(r.Root, r.Config.Browser.StorageState), nil
		}
		return "", fmt.Errorf("no setup.sequence in storyboard; nothing to bootstrap (use `browser login` for manual profiles)")
	}
	return r.runSetupOffCamera(headless)
}

func (r *Run) ensureSetupState(headless bool) (string, error) {
	if len(r.SB.Setup.Sequence) == 0 {
		if r.Config.Browser.StorageState != "" {
			return absJoin(r.Root, r.Config.Browser.StorageState), nil
		}
		if r.SB.Setup.StorageState != "" {
			return absJoin(r.Root, r.SB.Setup.StorageState), nil
		}
		return "", nil
	}
	if p := r.setupCachePath(); fileExists(p) {
		return p, nil
	}
	return r.runSetupOffCamera(headless)
}

func (r *Run) runSetupOffCamera(headless bool) (string, error) {
	tmpDir, err := os.MkdirTemp("", "autodoc-setup-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)
	be, err := capture.Get("playwright")
	if err != nil {
		return "", err
	}
	vw, vh := viewportOf(r)
	disabled := visual.Default()
	disabled.Cursor.Disabled = true
	disabled.Click.Disabled = true
	disabled.Typing.Disabled = true
	disabled.Camera.Disabled = true
	baseURL := r.SB.Config.BaseURL
	opts := capture.StartOptions{
		SceneID: "setup-discard", ViewportW: vw, ViewportH: vh, Headless: headless,
		BaseURL: baseURL, VideoDir: tmpDir, Redact: r.SB.Redact, Visuals: disabled,
	}
	if r.Config.Browser.StorageState != "" {
		opts.StorageState = absJoin(r.Root, r.Config.Browser.StorageState)
	} else if r.SB.Setup.StorageState != "" {
		opts.StorageState = absJoin(r.Root, r.SB.Setup.StorageState)
	}
	if err := be.Start(opts); err != nil {
		return "", fmt.Errorf("setup browser: %w", err)
	}
	defer be.Close()
	startURL := r.SB.Setup.StartURL
	if startURL == "" {
		startURL = "/"
	}
	if err := be.Navigate(startURL); err != nil {
		return "", fmt.Errorf("setup navigate: %w", err)
	}
	for i, st := range r.SB.Setup.Sequence {
		switch {
		case st.Action != nil:
			a := *st.Action
			if _, err := be.DoAction(a); err != nil {
				return "", fmt.Errorf("setup.sequence[%d] action %s: %w", i, a.Type, err)
			}
		case st.Wait != nil:
			if _, err := be.DoWait(*st.Wait); err != nil {
				return "", err
			}
		}
	}
	for _, es := range r.SB.Setup.ExpectedStates {
		if err := checkExpectedState(be, es); err != nil {
			return "", fmt.Errorf("setup precondition failed: %w", err)
		}
	}
	dst := r.setupCachePath()
	if err := be.SaveStorageState(dst); err != nil {
		return "", fmt.Errorf("setup state snapshot: %w", err)
	}
	return dst, nil
}

func checkExpectedState(be capture.CaptureBackend, es storyboard.ExpectedState) error {
	switch es.State {
	case "visible", "attached":
		sel := "body"
		if es.Target != nil {
			sel = es.Target.PlaywrightSelector()
		}
		if _, err := be.DoAction(storyboard.Action{Type: "expect", Target: es.Target}); err != nil {
			return fmt.Errorf("%s (%s): %w", es.Description, sel, err)
		}
	case "url":
		if _, err := be.DoWait(storyboard.WaitEvent{State: "url", URL: es.URL, TimeoutMs: es.TimeoutMs}); err != nil {
			return err
		}
	}
	return nil
}

func viewportOf(r *Run) (int, int) {
	vw, vh := r.Config.Browser.ViewportW, r.Config.Browser.ViewportH
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
	return vw, vh
}
