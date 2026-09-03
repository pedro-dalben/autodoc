package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mxschmitt/playwright-go"
	"github.com/pedro-dalben/autodoc/internal/storyboard"
	"github.com/pedro-dalben/autodoc/internal/visual"
)

func init() { Register("playwright", func() CaptureBackend { return &PlaywrightBackend{} }) }

type PlaywrightBackend struct {
	pw         *playwright.Playwright
	browser    playwright.Browser
	context    playwright.BrowserContext
	page       playwright.Page
	opts       StartOptions
	vis        visual.Config
	events     []EventRecord
	started    time.Time
	videoDir   string
	chapters   []Chapter
	cursor     visual.Point
	cursorInit bool
	vw, vh     int
}

func (b *PlaywrightBackend) Start(opts StartOptions) error {
	b.opts = opts
	b.vis = opts.Visuals
	b.vis.ApplyDefaults()
	b.events = nil
	b.chapters = nil
	b.started = time.Now()
	b.cursorInit = false
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
	b.vw, b.vh = vw, vh
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
	b.ensureOverlay()
	return nil
}

func (b *PlaywrightBackend) nowMs() int64 { return time.Since(b.started).Milliseconds() }

func (b *PlaywrightBackend) record(kind, label string) {
	b.events = append(b.events, EventRecord{Kind: kind, Label: label, AtMs: b.nowMs()})
}

func (b *PlaywrightBackend) recordV(kind, label string, ev visual.VisualEvent) {
	raw, _ := json.Marshal(ev)
	b.events = append(b.events, EventRecord{Kind: kind, Label: label, AtMs: b.nowMs(), Note: string(raw)})
}

func (b *PlaywrightBackend) eval(js string) {
	if b.page == nil {
		return
	}
	_, _ = b.page.Evaluate(js)
}

func (b *PlaywrightBackend) ensureOverlay() {
	if !b.vis.CursorOn() && !b.vis.ClickOn() {
		return
	}
	b.eval(visual.EnsureOverlayJS())
}

func (b *PlaywrightBackend) parkPos() visual.Point {
	if b.vis.Cursor.ParkX >= 0 && b.vis.Cursor.ParkY >= 0 {
		return visual.Point{X: float64(b.vis.Cursor.ParkX), Y: float64(b.vis.Cursor.ParkY)}
	}
	return visual.Point{X: float64(b.vw - 90), Y: float64(b.vh - 70)}
}

func (b *PlaywrightBackend) moveCursorTo(p visual.Point) (from visual.Point, ms int) {
	from = b.cursor
	if !b.cursorInit {
		from = b.parkPos()
		b.eval(visual.CursorShowJS(from))
		b.cursorInit = true
	}
	if !b.vis.CursorOn() {
		b.cursor = p
		return from, 0
	}
	ms = visual.CursorMoveMs(from, p, b.vis.Cursor)
	b.eval(visual.CursorMoveJS(from, p, ms))
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	b.cursor = p
	return from, ms
}

func (b *PlaywrightBackend) parkCursor() {
	if !b.vis.CursorOn() || !b.cursorInit {
		return
	}
	target := b.parkPos()
	ms := b.vis.Cursor.ParkMs
	if ms <= 0 {
		ms = 200
	}
	b.eval(visual.CursorMoveJS(b.cursor, target, ms))
	time.Sleep(time.Duration(ms) * time.Millisecond)
	b.cursor = target
}

func (b *PlaywrightBackend) highlight(bb visual.BBox, ms int) {
	if !b.vis.HiliteOn() || !bb.Valid() {
		return
	}
	b.eval(visual.HighlightJS(bb, ms))
}

func (b *PlaywrightBackend) ripple(p visual.Point) {
	if !b.vis.RippleOn() {
		return
	}
	ms := b.vis.Click.RippleMs
	b.eval(visual.RippleJS(p, ms))
}

func (b *PlaywrightBackend) bboxOf(sel string) (visual.BBox, bool) {
	if b.page == nil {
		return visual.BBox{}, false
	}
	loc := b.page.Locator(sel)
	rect, err := loc.BoundingBox(playwright.LocatorBoundingBoxOptions{
		Timeout: playwright.Float(3000),
	})
	if err != nil || rect == nil || rect.Width <= 0 || rect.Height <= 0 {
		return visual.BBox{}, false
	}
	return visual.BBox{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}, true
}

func (b *PlaywrightBackend) isSensitive(sel string, loc playwright.Locator) bool {
	lower := strings.ToLower(sel)
	if strings.Contains(lower, "password") || strings.Contains(lower, "passwd") || strings.Contains(lower, "secret") {
		return true
	}
	out, err := loc.Evaluate(`(el) => { try {
	  if (!el) return 'missing';
	  var t = ((el.type || '') + '').toLowerCase();
	  if (t === 'password') return 'password';
	  if (el.hasAttribute && (el.hasAttribute('data-autodoc-redacted') || el.hasAttribute('data-sensitive'))) return 'redacted';
	  return t || (el.tagName || 'el');
	} catch(e) { return 'error'; } }`, nil)
	if err != nil {
		return false
	}
	s, _ := out.(string)
	return s == "password" || s == "redacted"
}

func (b *PlaywrightBackend) Navigate(url string) error {
	if url == "" {
		return nil
	}
	if b.opts.BaseURL != "" && strings.HasPrefix(url, "/") {
		url = strings.TrimRight(b.opts.BaseURL, "/") + url
	}
	if _, err := b.page.Goto(url, playwright.PageGotoOptions{WaitUntil: playwright.WaitUntilStateLoad}); err != nil {
		return fmt.Errorf("goto %s: %w", url, err)
	}
	b.ensureOverlay()
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
		      el.setAttribute('aria-hidden','true');
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

func (b *PlaywrightBackend) resolveValue(a storyboard.Action) (string, bool, error) {
	if a.SecretRef != "" {
		v, err := storyboard.ResolveSecret(a.SecretRef)
		if err != nil {
			return "", true, err
		}
		return v, true, nil
	}
	if a.Value != "" {
		return a.Value, false, nil
	}
	return a.Text, false, nil
}

func (b *PlaywrightBackend) cueFocus(sel string, loc playwright.Locator) (visual.BBox, visual.Point, bool) {
	b.ensureOverlay()
	bb, ok := b.bboxOf(sel)
	if !ok {
		return visual.BBox{}, visual.Point{}, false
	}
	target := bb.Center()
	from, _ := b.moveCursorTo(target)
	if b.vis.HiliteOn() {
		b.highlight(bb, b.vis.Pacing.HighlightMs+400)
		time.Sleep(time.Duration(b.vis.Pacing.HighlightMs) * time.Millisecond)
	}
	_ = from
	return bb, target, true
}

func (b *PlaywrightBackend) DoAction(a storyboard.Action) (ActionResult, error) {
	start := b.nowMs()
	switch a.Type {
	case "goto":
		url := a.URL
		if url == "" && a.Target != nil {
			url = a.Target.Text
		}
		if err := b.Navigate(url); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "click":
		return b.doClick(a, start)
	case "fill", "type":
		return b.doType(a, start)
	case "press":
		return b.doPress(a, start)
	case "select":
		sel := selectorFor(a.Target)
		bb, target, ok := b.cueFocus(sel, b.page.Locator(sel))
		focusAt := b.nowMs()
		opt := playwright.SelectOptionValues{Values: &[]string{a.Value}}
		if a.Text != "" && a.Value == "" {
			opt = playwright.SelectOptionValues{Labels: &[]string{a.Text}}
		}
		if _, err := b.page.Locator(sel).SelectOption(opt); err != nil {
			return ActionResult{}, fmt.Errorf("select %s: %w", sel, err)
		}
		actionAt := b.nowMs()
		b.settle()
		b.emitInteraction("select", bb, target, ok, start, focusAt, actionAt, a)
		b.record("action", "select "+sel)
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "check", "uncheck":
		sel := selectorFor(a.Target)
		bb, target, ok := b.cueFocus(sel, b.page.Locator(sel))
		focusAt := b.nowMs()
		var err error
		if a.Type == "check" {
			err = b.page.Locator(sel).Check()
		} else {
			err = b.page.Locator(sel).Uncheck()
		}
		if err != nil {
			return ActionResult{}, fmt.Errorf("%s %s: %w", a.Type, sel, err)
		}
		actionAt := b.nowMs()
		b.settle()
		b.emitInteraction(a.Type, bb, target, ok, start, focusAt, actionAt, a)
		b.record("action", a.Type+" "+sel)
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "hover":
		sel := selectorFor(a.Target)
		bb, target, ok := b.cueFocus(sel, b.page.Locator(sel))
		focusAt := b.nowMs()
		if err := b.page.Locator(sel).Hover(); err != nil {
			return ActionResult{}, fmt.Errorf("hover %s: %w", sel, err)
		}
		b.emitInteraction("hover", bb, target, ok, start, focusAt, b.nowMs(), a)
		b.record("action", "hover "+sel)
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "reload":
		if _, err := b.page.Reload(); err != nil {
			return ActionResult{}, fmt.Errorf("reload: %w", err)
		}
		b.ensureOverlay()
		b.record("action", "reload")
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "goback":
		if _, err := b.page.GoBack(); err != nil {
			return ActionResult{}, fmt.Errorf("goback: %w", err)
		}
		b.ensureOverlay()
		b.record("action", "goback")
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "expect":
		sel := selectorFor(a.Target)
		state := playwright.WaitForSelectorStateVisible
		if _, err := b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{State: state, Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("expect %s: %w", sel, err)
		}
		b.record("action", "expect "+sel)
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "screenshot":
		name := a.Name
		if name == "" {
			name = fmt.Sprintf("shot-%d.png", start)
		}
		if err := b.Screenshot(filepath.Join(b.videoDir, name)); err != nil {
			return ActionResult{}, err
		}
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	case "scroll":
		_, _ = b.page.Evaluate(`() => window.scrollBy(0, 400)`)
		b.record("action", "scroll")
		return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
	default:
		return ActionResult{}, fmt.Errorf("unsupported action type %q", a.Type)
	}
}

func (b *PlaywrightBackend) doClick(a storyboard.Action, start int64) (ActionResult, error) {
	sel := selectorFor(a.Target)
	loc := b.page.Locator(sel)
	bb, target, ok := b.cueFocus(sel, loc)
	focusAt := b.nowMs()
	if ok {
		b.ripple(target)
		time.Sleep(120 * time.Millisecond)
	}
	if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(15000)}); err != nil {
		return ActionResult{}, fmt.Errorf("click %s: %w", sel, err)
	}
	actionAt := b.nowMs()
	rippleHold := b.vis.Click.RippleMs / 2
	if rippleHold < 150 {
		rippleHold = 150
	}
	time.Sleep(time.Duration(rippleHold) * time.Millisecond)
	b.settle()
	b.emitInteraction("click", bb, target, ok, start, focusAt, actionAt, a)
	b.record("action", "click "+sel)
	return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
}

func (b *PlaywrightBackend) doType(a storyboard.Action, start int64) (ActionResult, error) {
	sel := selectorFor(a.Target)
	loc := b.page.Locator(sel)
	value, fromSecret, err := b.resolveValue(a)
	if err != nil {
		return ActionResult{}, err
	}
	bb, target, ok := b.cueFocus(sel, loc)
	focusAt := b.nowMs()
	sensitive := fromSecret || b.isSensitive(sel, loc)
	progressive := b.vis.TypingOn() && b.vis.Typing.Progressive && !sensitive && !a.IsInstant()
	typingStart := b.nowMs()
	if progressive {
		if err := loc.Click(playwright.LocatorClickOptions{Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("focus %s: %w", sel, err)
		}
		if a.Type == "fill" {
			if err := loc.Fill("", playwright.LocatorFillOptions{Timeout: playwright.Float(15000)}); err != nil {
				return ActionResult{}, fmt.Errorf("clear %s: %w", sel, err)
			}
		}
		delay := float64(b.vis.Typing.CharDelayMs)
		if err := loc.PressSequentially(value, playwright.LocatorPressSequentiallyOptions{Delay: playwright.Float(delay)}); err != nil {
			return ActionResult{}, fmt.Errorf("type %s: %w", sel, err)
		}
	} else {
		if err := loc.Fill(value, playwright.LocatorFillOptions{Timeout: playwright.Float(15000)}); err != nil {
			return ActionResult{}, fmt.Errorf("fill %s: %w", sel, err)
		}
	}
	typingEnd := b.nowMs()
	actionAt := typingEnd
	b.settle()
	ev := visual.VisualEvent{
		Type: "interaction", Interaction: a.Type,
		StartedAtMs: start, FocusAtMs: focusAt, ActionAtMs: actionAt, EndedAtMs: b.nowMs(),
		TypingChars: len([]rune(value)), TypingMs: typingEnd - typingStart,
		Progressive: progressive, Instant: !progressive, Sensitive: sensitive,
		Zoom: b.zoomFor(a, bb, ok),
	}
	if ok {
		ev.BBox = &bb
		ev.NormBBox = visual.NormBBox(bb, b.vw, b.vh)
		ev.CursorTo = &target
	}
	b.recordV("visual", a.Type+" "+sel, ev)
	safe := sel
	if fromSecret {
		safe += " <secret>"
	}
	_ = safe
	b.record("action", "fill "+sel)
	return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
}

func (b *PlaywrightBackend) doPress(a storyboard.Action, start int64) (ActionResult, error) {
	key := a.Key
	if key == "" {
		key = "Enter"
	}
	if a.Target != nil && !a.Target.Empty() {
		sel := selectorFor(a.Target)
		bb, target, ok := b.cueFocus(sel, b.page.Locator(sel))
		focusAt := b.nowMs()
		if err := b.page.Locator(sel).Press(key); err != nil {
			return ActionResult{}, fmt.Errorf("press %s: %w", key, err)
		}
		b.emitInteraction("press", bb, target, ok, start, focusAt, b.nowMs(), a)
	} else if err := b.page.Keyboard().Press(key); err != nil {
		return ActionResult{}, fmt.Errorf("press %s: %w", key, err)
	}
	b.record("action", "press "+key)
	return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
}

func (b *PlaywrightBackend) settle() {
	ms := b.vis.Pacing.ClickSettleMs
	if ms <= 0 {
		return
	}
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

func (b *PlaywrightBackend) zoomFor(a storyboard.Action, bb visual.BBox, ok bool) float64 {
	if !ok || !b.vis.CameraOn() || !a.WantsZoom() {
		return 1
	}
	shot := visual.PlanCamera(bb, b.vw, b.vh, visual.ZoomForAction(a.Type, b.vis.Camera), b.vis.Camera)
	if !shot.Apply {
		return 1
	}
	return shot.Zoom
}

func (b *PlaywrightBackend) emitInteraction(kind string, bb visual.BBox, target visual.Point, ok bool, start, focusAt, actionAt int64, a storyboard.Action) {
	ev := visual.VisualEvent{
		Type: "interaction", Interaction: kind,
		StartedAtMs: start, FocusAtMs: focusAt, ActionAtMs: actionAt, EndedAtMs: b.nowMs(),
		Zoom: b.zoomFor(a, bb, ok),
	}
	if ok {
		ev.BBox = &bb
		ev.NormBBox = visual.NormBBox(bb, b.vw, b.vh)
		ev.CursorTo = &target
	}
	b.recordV("visual", kind+" "+selectorFor(a.Target), ev)
}

func (b *PlaywrightBackend) DoSpeech(speechID string, durationMs int64) (ActionResult, error) {
	start := b.nowMs()
	b.ensureOverlay()
	b.parkCursor()
	b.recordV("speech_start", speechID, visual.VisualEvent{Type: "speech", SpeechID: speechID, StartedAtMs: start, DurationMs: durationMs})
	if durationMs > 0 {
		time.Sleep(time.Duration(durationMs) * time.Millisecond)
	}
	end := b.nowMs()
	b.recordV("speech_end", speechID, visual.VisualEvent{Type: "speech", SpeechID: speechID, StartedAtMs: start, EndedAtMs: end, DurationMs: end - start})
	return ActionResult{AtMs: start, ElapsedMs: end - start}, nil
}

func (b *PlaywrightBackend) DoHold(durationMs int64) (ActionResult, error) {
	start := b.nowMs()
	b.recordV("hold", fmt.Sprintf("%dms", durationMs), visual.VisualEvent{Type: "hold", StartedAtMs: start, DurationMs: durationMs})
	if durationMs > 0 {
		time.Sleep(time.Duration(durationMs) * time.Millisecond)
	}
	return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
}

func (b *PlaywrightBackend) DoPause(ms int64, label string) (ActionResult, error) {
	start := b.nowMs()
	if label == "" {
		label = "pad"
	}
	b.recordV("pad", label, visual.VisualEvent{Type: "pad", StartedAtMs: start, DurationMs: ms})
	if ms > 0 {
		time.Sleep(time.Duration(ms) * time.Millisecond)
	}
	return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, nil
}

func (b *PlaywrightBackend) SaveStorageState(path string) error {
	if b.context == nil {
		return fmt.Errorf("no browser context")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	_, err := b.context.StorageState(playwright.BrowserContextStorageStateOptions{Path: playwright.String(path)})
	return err
}

func (b *PlaywrightBackend) DoWait(w storyboard.WaitEvent) (ActionResult, error) {
	start := b.nowMs()
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
		_, err := b.page.WaitForSelector(sel, playwright.PageWaitForSelectorOptions{State: &st, Timeout: playwright.Float(timeout)})
		if err != nil {
			b.record("wait_timeout", state+" "+sel)
			return ActionResult{AtMs: start, ElapsedMs: b.nowMs() - start}, fmt.Errorf("wait %s %s: %w", state, sel, err)
		}
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
		pattern := w.URL
		if pattern == "" {
			pattern = w.Value
		}
		if pattern == "" {
			time.Sleep(600 * time.Millisecond)
		} else {
			_ = b.page.WaitForURL(pattern, playwright.PageWaitForURLOptions{Timeout: playwright.Float(timeout)})
		}
		b.record("wait", "url "+pattern)
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
	end := b.nowMs()
	b.recordV("wait_span", state, visual.VisualEvent{Type: "wait", Interaction: state, StartedAtMs: start, EndedAtMs: end, DurationMs: end - start})
	return ActionResult{AtMs: start, ElapsedMs: end - start}, nil
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
	b.chapters = append(b.chapters, Chapter{Title: title, AtMs: b.nowMs()})
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
