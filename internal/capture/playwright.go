package capture

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
)

func init() { Register("playwright", func() CaptureBackend { return &PlaywrightBackend{} }) }

type PlaywrightBackend struct {
	pw       *playwright.Playwright
	browser  playwright.Browser
	context  playwright.BrowserContext
	page     playwright.Page
	opts     StartOptions
	events   []EventRecord
	started  time.Time
	videoDir string
	chapters []Chapter
}

func (b *PlaywrightBackend) Start(opts StartOptions) error {
	b.opts = opts
	b.events = nil
	b.chapters = nil
	b.started = time.Now()
	if err := playwright.Install(); err != nil {
		return fmt.Errorf("playwright driver install check failed: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("playwright run: %w", err)
	}
	b.pw = pw
	vw, vh := opts.ViewportW, opts.ViewportH
	if vw == 0 {
		vw = 1280
	}
	if vh == 0 {
		vh = 720
	}
	b.videoDir = opts.VideoDir
	if b.videoDir == "" {
		b.videoDir = os.TempDir()
	}
	if err := os.MkdirAll(b.videoDir, 0o755); err != nil {
		return err
	}
	headless := opts.Headless
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(headless),
		Args: []string{
			"--no-sandbox",
			"--disable-dev-shm-usage",
			"--autoplay-policy=no-user-gesture-required",
		},
	})
	if err != nil {
		pw.Stop()
		return fmt.Errorf("chromium launch: %w", err)
	}
	b.browser = browser
	size := &playwright.Size{Width: vw, Height: vh}
	ctxOpts := playwright.BrowserNewContextOptions{
		Viewport:    size,
		RecordVideo: &playwright.RecordVideo{Dir: playwright.String(b.videoDir), Size: size},
	}
	if opts.StorageState != "" {
		if _, err := os.Stat(opts.StorageState); err == nil {
			ctxOpts.StorageStatePath = playwright.String(opts.StorageState)
		}
	}
	if opts.ProfileDir != "" {
		_ = opts.ProfileDir
	}
	ctx, err := browser.NewContext(ctxOpts)
	if err != nil {
		browser.Close()
		pw.Stop()
		return fmt.Errorf("new context: %w", err)
	}
	b.context = ctx
	page, err := ctx.NewPage()
	if err != nil {
		ctx.Close()
		browser.Close()
		pw.Stop()
		return fmt.Errorf("new page: %w", err)
	}
	b.page = page
	b.record("session", "start scene "+opts.SceneID)
	if len(opts.Redact.Selectors) > 0 || opts.Redact.MaskPasswordInputs {
		_ = b.ApplyRedaction(opts.Redact.Selectors, opts.Redact.MaskPasswordInputs)
	}
	return nil
}

func (b *PlaywrightBackend) record(kind, label string) {
	b.events = append(b.events, EventRecord{Kind: kind, Label: label, AtMs: time.Since(b.started).Milliseconds()})
}

func (b *PlaywrightBackend) Navigate(url string) error {
	if url == "" {
		return nil
	}
	if b.opts.BaseURL != "" && strings.HasPrefix(url, "/") {
		url = strings.TrimRight(b.opts.BaseURL, "/") + url
	}
	_, err := b.page.Goto(url, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateLoad})
	if err != nil {
		return fmt.Errorf("goto %s: %w", url, err)
	}
	b.record("navigate", url)
	return nil
}

func (b *PlaywrightBackend) ApplyRedaction(selectors []string, maskPasswords bool) error {
	all := append([]string{}, selectors...)
	if maskPasswords {
		all = append(all, `input[type="password"]`)
	}
	for _, sel := range all {
		js := fmt.Sprintf(`() => {
		  try {
		    document.querySelectorAll(%q).forEach(el => {
		      el.setAttribute('data-autodoc-redacted','1');
		      el.style.setProperty('filter','blur(8px)','important');
		      el.style.setProperty('background','#111','important');
		      el.style.setProperty('color','transparent','important');
		      if (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') { el.value = '••••••••'; el.textContent=''; }
		    });
		  } catch(e) {}
		}`, sel)
		_, _ = b.page.Evaluate(js)
	}
	if len(all) > 0 {
		b.record("redact", strings.Join(all, ","))
	}
	return nil
}

func selectorFor(t *storyboard.Target) string {
	if t == nil {
		return "body"
	}
	return t.PlaywrightSelector()
}

func (b *PlaywrightBackend) DoAction(a storyboard.Action) (ActionResult, error) {
	start := time.Since(b.started).Milliseconds()
	switch a.Type {
	case "goto":
		url := a.URL
		if url == "" && a.Target != nil {
			url = a.Target.Text
		}
		if err := b.Navigate(url); err != nil {
			return ActionResult{}, err
		}
	case "click":
		sel := selectorFor(a.Target)
		loc := b.page.Locator(sel)
		if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("click %s: %w", sel, err)
		}
		b.record("action", "click "+sel)
	case "fill", "type":
		sel := selectorFor(a.Target)
		val := a.Value
		if val == "" {
			val = a.Text
		}
		loc := b.page.Locator(sel)
		if err := loc.Fill(val, playwright.LocatorFillOptions{Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("fill %s: %w", sel, err)
		}
		b.record("action", "fill "+sel)
	case "press":
		key := a.Key
		if key == "" {
			key = "Enter"
		}
		if a.Target != nil && !a.Target.Empty() {
			if err := b.page.Locator(selectorFor(a.Target)).Press(key); err != nil {
				return ActionResult{}, fmt.Errorf("press %s: %w", key, err)
			}
		} else if err := b.page.Keyboard().Press(key); err != nil {
			return ActionResult{}, fmt.Errorf("press %s: %w", key, err)
		}
		b.record("action", "press "+key)
	case "select":
		sel := selectorFor(a.Target)
		opt := playwright.SelectOptionValues{Values: &[]string{a.Value}}
		if a.Text != "" && a.Value == "" {
			opt = playwright.SelectOptionValues{Labels: &[]string{a.Text}}
		}
		if _, err := b.page.Locator(sel).SelectOption(opt); err != nil {
			return ActionResult{}, fmt.Errorf("select %s: %w", sel, err)
		}
		b.record("action", "select "+sel)
	case "check":
		sel := selectorFor(a.Target)
		if err := b.page.Locator(sel).Check(); err != nil {
			return ActionResult{}, fmt.Errorf("check %s: %w", sel, err)
		}
		b.record("action", "check "+sel)
	case "uncheck":
		sel := selectorFor(a.Target)
		if err := b.page.Locator(sel).Uncheck(); err != nil {
			return ActionResult{}, fmt.Errorf("uncheck %s: %w", sel, err)
		}
		b.record("action", "uncheck "+sel)
	case "hover":
		sel := selectorFor(a.Target)
		if err := b.page.Locator(sel).Hover(); err != nil {
			return ActionResult{}, fmt.Errorf("hover %s: %w", sel, err)
		}
		b.record("action", "hover "+sel)
	case "reload":
		if _, err := b.page.Reload(); err != nil {
			return ActionResult{}, fmt.Errorf("reload: %w", err)
		}
		b.record("action", "reload")
	case "goback":
		if _, err := b.page.GoBack(); err != nil {
			return ActionResult{}, fmt.Errorf("goback: %w", err)
		}
		b.record("action", "goback")
	case "expect":
		sel := selectorFor(a.Target)
		state := playwright.WaitForSelectorStateVisible
		if _, err := b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{State: state, Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("expect %s: %w", sel, err)
		}
		b.record("action", "expect "+sel)
	case "screenshot":
		name := a.Name
		if name == "" {
			name = fmt.Sprintf("shot-%d.png", start)
		}
		if err := b.Screenshot(filepath.Join(b.videoDir, name)); err != nil {
			return ActionResult{}, err
		}
	case "scroll":
		_, _ = b.page.Evaluate(`() => window.scrollBy(0, 400)`)
		b.record("action", "scroll")
	default:
		return ActionResult{}, fmt.Errorf("unsupported action type %q", a.Type)
	}
	return ActionResult{AtMs: start}, nil
}

func (b *PlaywrightBackend) DoWait(w storyboard.WaitEvent) (ActionResult, error) {
	start := time.Since(b.started).Milliseconds()
	state := strings.ToLower(w.State)
	timeout := float64(15000)
	if w.TimeoutMs > 0 {
		timeout = float64(w.TimeoutMs)
	}
	switch state {
	case "visible", "attached", "hidden", "detached":
		sel := "body"
		if w.Target != nil {
			sel = selectorFor(w.Target)
		}
		var st playwright.WaitForSelectorState
		switch state {
		case "visible":
			st = *playwright.WaitForSelectorStateVisible
		case "hidden":
			st = *playwright.WaitForSelectorStateHidden
		case "attached":
			st = *playwright.WaitForSelectorStateAttached
		case "detached":
			st = *playwright.WaitForSelectorStateDetached
		}
		_, _ = b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{State: &st, Timeout: playwright.Float(timeout)})
		b.record("wait", state+" "+sel)
	case "load":
		_ = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{Timeout: playwright.Float(timeout)})
		b.record("wait", "load")
	case "networkidle":
		_ = b.page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State:   playwright.LoadStateNetworkidle,
			Timeout: playwright.Float(timeout),
		})
		b.record("wait", "networkidle")
	case "url":
		_ = b.page.WaitForURL(w.URL, playwright.PageWaitForURLOptions{Timeout: playwright.Float(timeout)})
		b.record("wait", "url "+w.URL)
	case "timeout", "settle":
		ms := w.TimeoutMs
		if w.SettleMs > 0 {
			ms = w.SettleMs
		}
		if ms <= 0 {
			ms = 600
		}
		time.Sleep(time.Duration(ms) * time.Millisecond)
		b.record("wait", fmt.Sprintf("sleep %dms", ms))
	default:
		time.Sleep(600 * time.Millisecond)
		b.record("wait", state)
	}
	if w.SettleMs > 0 {
		time.Sleep(time.Duration(w.SettleMs) * time.Millisecond)
	}
	return ActionResult{AtMs: start}, nil
}

func (b *PlaywrightBackend) Screenshot(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := b.page.Screenshot(playwright.PageScreenshotOptions{Path: playwright.String(path)})
	if err != nil {
		return fmt.Errorf("screenshot: %w", err)
	}
	b.record("screenshot", path)
	return nil
}

func (b *PlaywrightBackend) ShowAction(label string) { b.record("chapter-action", label) }
func (b *PlaywrightBackend) ShowChapter(title string) {
	b.chapters = append(b.chapters, Chapter{Title: title, AtMs: time.Since(b.started).Milliseconds()})
	b.record("chapter", title)
}

func (b *PlaywrightBackend) Stop() (Artifact, error) {
	art := Artifact{SceneID: b.opts.SceneID, Events: b.events}
	videoPath := ""
	if b.page != nil {
		if v := b.page.Video(); v != nil {
			if p, err := v.Path(); err == nil {
				videoPath = p
			}
		}
	}
	if b.context != nil {
		_ = b.context.Close()
		b.context = nil
	}
	if b.browser != nil {
		_ = b.browser.Close()
		b.browser = nil
	}
	if b.pw != nil {
		_ = b.pw.Stop()
		b.pw = nil
	}
	if videoPath != "" {
		dst := filepath.Join(b.videoDir, b.opts.SceneID+".webm")
		if _, err := os.Stat(videoPath); err == nil {
			if videoPath != dst {
				_ = os.Rename(videoPath, dst)
				videoPath = dst
			}
		}
		art.VideoPath = videoPath
	}
	return art, nil
}

func (b *PlaywrightBackend) Close() error {
	if b.context != nil {
		_ = b.context.Close()
	}
	if b.browser != nil {
		_ = b.browser.Close()
	}
	if b.pw != nil {
		_ = b.pw.Stop()
	}
	return nil
}
