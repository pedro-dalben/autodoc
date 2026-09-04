// Package ui implements AutoDoc's focused, domain-scoped UI inspection.
//
// It answers one question per query — "where do I send a message?", "what
// changed after clicking Enviar?" — and returns only the controls that
// matter plus a retrieval handle for the raw inventory. It is not generic
// browser automation: recording still runs through the deterministic
// capture backend, and interactive exploration can stay with the agent's
// own Playwright MCP when genuinely needed.
package ui

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/mxschmitt/playwright-go"
	"github.com/pedro-dalben/autodoc/internal/evidence"
)

// InspectOptions configures one focused query.
type InspectOptions struct {
	BaseURL      string // e.g. http://localhost:3000
	URL          string // path or absolute URL
	Intent       string // natural-language goal, e.g. "send a message"
	StorageState string // optional storage-state.json path
	Limit        int    // max controls returned (default 12)
	ViewportW    int
	ViewportH    int
	TimeoutMs    int
}

// inventoryJS collects every visible interactive element with the fields the
// storyboard author needs to pick stable locators.
const inventoryJS = `
() => {
  const vw = innerWidth, vh = innerHeight;
  const txt = (s) => (s || '').replace(/\s+/g, ' ').trim().slice(0, 80);
  const implicitRole = (el) => {
    const t = el.tagName.toLowerCase();
    if (el.getAttribute('role')) return el.getAttribute('role');
    if (t === 'a' && el.hasAttribute('href')) return 'link';
    if (t === 'button' || (t === 'input' && ['button','submit','reset'].includes(el.type))) return 'button';
    if (t === 'input' && ['checkbox'].includes(el.type)) return 'checkbox';
    if (t === 'input' && ['radio'].includes(el.type)) return 'radio';
    if (t === 'select') return 'combobox';
    if (t === 'textarea') return 'textbox';
    if (t === 'input') return 'textbox';
    if (t === 'option') return 'option';
    if (t === 'h1' || t === 'h2' || t === 'h3') return 'heading';
    return '';
  };
  const visible = (el) => {
    const r = el.getBoundingClientRect();
    if (r.width < 2 || r.height < 2) return false;
    const st = getComputedStyle(el);
    if (st.visibility === 'hidden' || st.display === 'none' || parseFloat(st.opacity) === 0) return false;
    return true;
  };
  const labelOf = (el) => {
    let v = el.getAttribute('aria-label') || el.getAttribute('title') || el.getAttribute('placeholder');
    if (!v && el.id) {
      const l = document.querySelector('label[for="' + CSS.escape(el.id) + '"]');
      if (l) v = l.textContent;
    }
    if (!v) {
      const wrap = el.closest('label');
      if (wrap) v = wrap.textContent;
    }
    if (!v && el.getAttribute('aria-labelledby')) {
      const ref = document.getElementById(el.getAttribute('aria-labelledby'));
      if (ref) v = ref.textContent;
    }
    if (!v && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') && el.value) v = el.value;
    if (!v) v = el.innerText || el.textContent;
    return txt(v);
  };
  const openDialog = document.querySelector('dialog[open], [role="dialog"]');
  const regionOf = (el, r) => {
    if (openDialog && openDialog.contains(el)) return 'dialog';
    if (r.y < vh * 0.12) return 'header';
    if (r.y + r.height > vh * 0.93) return 'footer';
    if (r.x + r.width <= vw * 0.32 && r.width <= vw * 0.4) return 'sidebar';
    if (r.x >= vw * 0.72 && r.width <= vw * 0.34) return 'aside';
    const t = el.tagName.toLowerCase();
    const interactiveInput = ['input','textarea','select'].includes(t);
    if (interactiveInput && r.y > vh * 0.60) return 'composer';
    return 'main';
  };
  const nodes = [...document.querySelectorAll('a[href],button,input,select,textarea,[role],[data-testid]')];
  const out = [];
  const seen = new Set();
  for (const el of nodes) {
    const r = el.getBoundingClientRect();
    if (el.tagName === 'OPTION' || el.tagName === 'SCRIPT' || el.tagName === 'STYLE') continue;
    if (!visible(el)) continue;
    const rect = { x: Math.round(r.x), y: Math.round(r.y), w: Math.round(r.width), h: Math.round(r.height) };
    const e = {
      tag: el.tagName.toLowerCase(),
	  id: el.id || '',
      role: implicitRole(el),
      name: labelOf(el),
      text: txt((el.tagName === 'INPUT' || el.tagName === 'TEXTAREA') ? '' : el.innerText || el.textContent),
      test_id: el.getAttribute('data-testid') || '',
      placeholder: el.getAttribute('placeholder') || '',
      region: regionOf(el, r),
      bbox: rect,
      visible: true,
      enabled: !el.disabled && el.getAttribute('aria-disabled') !== 'true',
    };
    if (e.name && e.text && e.name === e.text) e.text = '';
    const key = e.tag + '|' + e.name + '|' + e.test_id + '|' + Math.round(rect.x) + ',' + Math.round(rect.y);
    if (seen.has(key)) continue;
    seen.add(key);
    out.push(e);
  }
  return { url: location.pathname + location.search, title: document.title, elements: out };
}
`

// Inspect opens the page once, extracts the inventory and returns it with
// intent filtering applied. The raw inventory is the caller's responsibility
// to persist (it is returned inside the result).
func Inspect(opts InspectOptions) (evidence.Inventory, string, error) {
	page, closeFn, err := openPage(opts)
	if err != nil {
		return evidence.Inventory{}, "", err
	}
	defer closeFn()

	raw, err := page.Evaluate(inventoryJS)
	if err != nil {
		return evidence.Inventory{}, "", fmt.Errorf("inventory evaluate: %w", err)
	}
	inv, rawJSON, err := parseInventory(raw)
	if err != nil {
		return evidence.Inventory{}, "", err
	}
	inv.URL = resolveURL(opts)
	inv.Intent = opts.Intent
	inv.Controls = filterControls(inv.Controls, opts.Intent, opts.Limit)
	inv.Recommended = recommend(inv.Controls)
	return inv, rawJSON, nil
}

// Action is one exploration action for Diff.
type Action struct {
	Type  string // click | fill
	Label string // accessible name or visible text of the target
	Value string // fill value
}

// DiffResult is the semantic delta between before and after one action.
type DiffResult struct {
	URL       string                      `json:"url"`
	URLFrom   string                      `json:"url_from,omitempty"`
	Added     []string                    `json:"added"`
	Removed   []string                    `json:"removed"`
	Changed   []string                    `json:"changed,omitempty"`
	Changes   []evidence.MeaningfulChange `json:"changes,omitempty"`
	Dialog    string                      `json:"dialog,omitempty"` // opened | closed
	Region    string                      `json:"region,omitempty"` // dominant region of the change
	BeforeRef string                      `json:"before_ref,omitempty"`
	AfterRef  string                      `json:"after_ref,omitempty"`
}

// Compact renders the agent-facing delta (small, stable lines).
func (d DiffResult) Compact() string {
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s", d.URL)
	if d.URLFrom != "" && d.URLFrom != d.URL {
		fmt.Fprintf(&b, " (from %s)", d.URLFrom)
	}
	b.WriteString("\n")
	if d.Dialog != "" {
		fmt.Fprintf(&b, "dialog: %s\n", d.Dialog)
	}
	for _, a := range d.Added {
		fmt.Fprintf(&b, "+ %s\n", a)
	}
	for _, r := range d.Removed {
		fmt.Fprintf(&b, "- %s\n", r)
	}
	for _, c := range d.Changed {
		fmt.Fprintf(&b, "~ %s\n", c)
	}
	if len(d.Added)+len(d.Removed)+len(d.Changed) == 0 && d.Dialog == "" {
		b.WriteString("no meaningful change detected\n")
	}
	if d.Region != "" {
		fmt.Fprintf(&b, "changed region: %s\n", d.Region)
	}
	return b.String()
}

// Diff inspects the page, performs ONE action, inspects again and returns
// the semantic delta. Both raw inventories are returned for the evidence
// store (before JSON, after JSON).
func Diff(opts InspectOptions, action Action) (DiffResult, string, string, error) {
	page, closeFn, err := openPage(opts)
	if err != nil {
		return DiffResult{}, "", "", err
	}
	defer closeFn()

	beforeRaw, err := page.Evaluate(inventoryJS)
	if err != nil {
		return DiffResult{}, "", "", fmt.Errorf("before evaluate: %w", err)
	}
	before, beforeJSON, err := parseInventory(beforeRaw)
	if err != nil {
		return DiffResult{}, "", "", err
	}
	urlBefore := before.URL

	if err := performAction(page, action); err != nil {
		return DiffResult{}, "", "", err
	}
	waitSettled(page, opts.TimeoutMs)

	afterRaw, err := page.Evaluate(inventoryJS)
	if err != nil {
		return DiffResult{}, "", "", fmt.Errorf("after evaluate: %w", err)
	}
	after, afterJSON, err := parseInventory(afterRaw)
	if err != nil {
		return DiffResult{}, "", "", err
	}
	res := semanticDiff(before, after)
	res.URL = resolveURL(opts)
	res.URLFrom = urlBefore
	return res, beforeJSON, afterJSON, nil
}

func openPage(opts InspectOptions) (playwright.Page, func(), error) {
	if err := playwright.Install(); err != nil {
		return nil, nil, fmt.Errorf("playwright driver install check failed: %w", err)
	}
	pw, err := playwright.Run()
	if err != nil {
		return nil, nil, fmt.Errorf("playwright run: %w", err)
	}
	closeAll := func() { pw.Stop() }
	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
		Args:     []string{"--no-sandbox", "--disable-dev-shm-usage"},
	})
	if err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("chromium launch: %w", err)
	}
	closeAll = func() { browser.Close(); pw.Stop() }
	ctxOpts := playwright.BrowserNewContextOptions{}
	w, h := opts.ViewportW, opts.ViewportH
	if w == 0 {
		w = 1280
	}
	if h == 0 {
		h = 720
	}
	ctxOpts.Viewport = &playwright.Size{Width: w, Height: h}
	if opts.StorageState != "" {
		ctxOpts.StorageStatePath = playwright.String(opts.StorageState)
	}
	ctx, err := browser.NewContext(ctxOpts)
	if err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("new context: %w", err)
	}
	closeAll = func() { ctx.Close(); browser.Close(); pw.Stop() }
	page, err := ctx.NewPage()
	if err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("new page: %w", err)
	}
	timeout := opts.TimeoutMs
	if timeout <= 0 {
		timeout = 15000
	}
	target := opts.URL
	if opts.BaseURL != "" && !strings.HasPrefix(target, "http") {
		target = strings.TrimSuffix(opts.BaseURL, "/") + "/" + strings.TrimPrefix(target, "/")
	}
	if _, err := page.Goto(target, playwright.PageGotoOptions{
		Timeout:   playwright.Float(float64(timeout)),
		WaitUntil: playwright.WaitUntilStateLoad,
	}); err != nil {
		closeAll()
		return nil, nil, fmt.Errorf("goto %s: %w", target, err)
	}
	waitSettled(page, timeout)
	return page, closeAll, nil
}

func waitSettled(page playwright.Page, timeoutMs int) {
	_ = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State:   playwright.LoadStateNetworkidle,
		Timeout: playwright.Float(3500),
	})
}

func performAction(page playwright.Page, action Action) error {
	switch action.Type {
	case "click":
		loc := page.GetByRole("button", playwright.PageGetByRoleOptions{Name: action.Label})
		if n, _ := loc.Count(); n == 0 {
			loc = page.GetByRole("link", playwright.PageGetByRoleOptions{Name: action.Label})
		}
		if n, _ := loc.Count(); n == 0 {
			loc = page.GetByText(action.Label, playwright.PageGetByTextOptions{Exact: playwright.Bool(false)})
		}
		if err := loc.First().Click(playwright.LocatorClickOptions{Timeout: playwright.Float(6000)}); err != nil {
			return fmt.Errorf("click %q: %w", action.Label, err)
		}
		return nil
	case "fill":
		parts := strings.SplitN(action.Label, "=", 2)
		name, value := parts[0], action.Value
		if len(parts) == 2 {
			name, value = parts[0], parts[1]
		}
		loc := page.GetByRole("textbox", playwright.PageGetByRoleOptions{Name: name})
		if n, _ := loc.Count(); n == 0 {
			loc = page.GetByPlaceholder(name)
		}
		if n, _ := loc.Count(); n == 0 {
			loc = page.GetByLabel(name)
		}
		if err := loc.First().Fill(value, playwright.LocatorFillOptions{Timeout: playwright.Float(6000)}); err != nil {
			return fmt.Errorf("fill %q: %w", name, err)
		}
		return nil
	case "":
		return nil
	default:
		return fmt.Errorf("unsupported diff action %q (click|fill)", action.Type)
	}
}

func parseInventory(raw any) (evidence.Inventory, string, error) {
	b, err := json.Marshal(raw)
	if err != nil {
		return evidence.Inventory{}, "", err
	}
	var inv evidence.Inventory
	var doc struct {
		URL      string             `json:"url"`
		Title    string             `json:"title"`
		Elements []evidence.Element `json:"elements"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return evidence.Inventory{}, "", fmt.Errorf("parse inventory: %w", err)
	}
	inv.URL, inv.Title, inv.Controls = doc.URL, doc.Title, doc.Elements
	return inv, string(b), nil
}

// intentTokens splits a natural-language intent into meaningful tokens.
func intentTokens(intent string) []string {
	stop := map[string]bool{"a": true, "o": true, "e": true, "de": true, "da": true, "do": true,
		"the": true, "to": true, "of": true, "in": true, "on": true, "for": true, "que": true}
	var toks []string
	for _, t := range strings.FieldsFunc(strings.ToLower(intent), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == 'á' || r == 'é' || r == 'ç')
	}) {
		if len(t) > 2 && !stop[t] {
			toks = append(toks, t)
		}
	}
	return toks
}

// filterControls narrows the inventory by intent (when given) and caps the
// number of controls actually sent to the agent.
func filterControls(controls []evidence.Element, intent string, limit int) []evidence.Element {
	if limit <= 0 {
		limit = 12
	}
	toks := intentTokens(intent)
	type scored struct {
		el    evidence.Element
		score int
	}
	var scoredAll []scored
	for _, el := range controls {
		s := 0
		if len(toks) > 0 {
			hay := strings.ToLower(el.Name + " " + el.Text + " " + el.Placeholder + " " + el.TestID)
			role := strings.ToLower(el.Role)
			for _, t := range toks {
				if strings.Contains(hay, t) {
					s += 3
				}
				if strings.Contains(role, t) {
					s += 2
				}
			}
			// verbs used in tutorials map to interaction kinds
			for _, t := range toks {
				switch {
				case strings.Contains("enviar send submit salvar save", t) && role == "button":
					s += 2
				case strings.Contains("escrever digitar type escreve mensagem message coment", t) &&
					(role == "textbox" || el.Tag == "textarea"):
					s += 2
				case strings.Contains("abrir open entrar acessar conversa chat", t) && role == "link":
					s += 1
				}
			}
		} else {
			// no intent: prefer primary interactive controls over headings
			if el.Role == "button" || el.Role == "link" || el.Role == "textbox" || el.Role == "combobox" {
				s = 1
			}
		}
		scoredAll = append(scoredAll, scored{el, s})
	}
	if len(toks) > 0 {
		// neighborhood: keep zero-score elements adjacent (same row) to hits
		kept := make([]scored, 0, len(scoredAll))
		for _, sc := range scoredAll {
			if sc.score > 0 {
				kept = append(kept, sc)
				continue
			}
			for _, other := range scoredAll {
				if other.score <= 0 {
					continue
				}
				a, b2 := sc.el.BBox, other.el.BBox
				sameRow := a.Y >= b2.Y-8 && a.Y <= b2.Y+b2.H+8
				nearby := a.X >= b2.X-220 && a.X <= b2.X+b2.W+220
				if sameRow && nearby {
					kept = append(kept, sc)
					break
				}
			}
		}
		scoredAll = kept
	}
	sort.SliceStable(scoredAll, func(i, j int) bool {
		if scoredAll[i].score != scoredAll[j].score {
			return scoredAll[i].score > scoredAll[j].score
		}
		return scoredAll[i].el.BBox.Y < scoredAll[j].el.BBox.Y
	})
	out := make([]evidence.Element, 0, limit)
	for _, sc := range scoredAll {
		if len(out) >= limit {
			break
		}
		out = append(out, sc.el)
	}
	return out
}

// recommend proposes the canonical pair for message-style flows.
func recommend(controls []evidence.Element) string {
	var input, button string
	for _, c := range controls {
		r := c.Role
		if (r == "textbox" || c.Tag == "textarea") && input == "" {
			input = c.CompactLine()
		}
		if r == "button" && button == "" && strings.Contains(strings.ToLower(c.Name+" "+c.Text), "enviar send submit") {
			button = c.CompactLine()
		}
	}
	if input != "" && button != "" {
		return input + " -> " + button
	}
	return ""
}

// semanticDiff compares two inventories by stable keys.
func semanticDiff(before, after evidence.Inventory) DiffResult {
	key := func(el evidence.Element) string {
		if el.TestID != "" {
			return "tid:" + el.TestID
		}
		return "el:" + el.Tag + "|" + el.Role + "|" + el.Name
	}
	beforeMap := map[string]evidence.Element{}
	for _, el := range before.Controls {
		beforeMap[key(el)] = el
	}
	afterMap := map[string]evidence.Element{}
	for _, el := range after.Controls {
		afterMap[key(el)] = el
	}
	res := DiffResult{}
	regionCount := map[string]int{}
	for k, el := range afterMap {
		prev, ok := beforeMap[k]
		if !ok {
			res.Added = append(res.Added, el.CompactLine())
			res.Changes = append(res.Changes, evidence.MeaningfulChange{Type: "content_added", Region: el.Region, Description: el.CompactLine(), BBox: el.BBox})
			regionCount[el.Region]++
			continue
		}
		if prev.Text != el.Text || prev.Name != el.Name || prev.Enabled != el.Enabled {
			res.Changed = append(res.Changed, el.CompactLine())
			res.Changes = append(res.Changes, evidence.MeaningfulChange{Type: "state_changed", Region: el.Region, Description: el.CompactLine(), BBox: el.BBox})
			regionCount[el.Region]++
		}
	}
	for k, el := range beforeMap {
		if _, ok := afterMap[k]; !ok {
			res.Removed = append(res.Removed, el.CompactLine())
			res.Changes = append(res.Changes, evidence.MeaningfulChange{Type: "content_removed", Region: el.Region, Description: el.CompactLine(), BBox: el.BBox})
			regionCount[el.Region]++
		}
	}
	dialogBefore, dialogAfter := hasDialog(before), hasDialog(after)
	if dialogAfter && !dialogBefore {
		res.Dialog = "opened"
	} else if dialogBefore && !dialogAfter {
		res.Dialog = "closed"
	}
	res.Region = dominantRegion(regionCount)
	return res
}

func hasDialog(inv evidence.Inventory) bool {
	for _, c := range inv.Controls {
		if c.Region == "dialog" {
			return true
		}
	}
	return false
}

func dominantRegion(counts map[string]int) string {
	best, n := "", 0
	for r, c := range counts {
		if c > n {
			best, n = r, c
		}
	}
	return best
}

func resolveURL(opts InspectOptions) string {
	if strings.HasPrefix(opts.URL, "http") {
		return opts.URL
	}
	return opts.URL
}
