// Package evidence implements AutoDoc's recoverable evidence store.
//
// Raw information discovered during tutorial authoring (browser inventories,
// UI diffs, page state) is persisted locally under .autodoc/cache/evidence/
// while the agent only sees a compact representation plus a retrieval handle.
// Every ref can be resolved back to the raw payload, so compression is
// recoverable; small payloads bypass compaction entirely (adaptive bypass).
package evidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const maxPayloadBytes = 8 << 20 // refuse to persist anything above 8 MB

// Ref identifies a stored raw payload and carries the metadata needed to
// reuse or invalidate it without reading the payload itself.
type Ref struct {
	ID          string            `json:"id"`
	Kind        string            `json:"kind"` // ui_inventory | ui_diff | page_state
	Bytes       int               `json:"bytes"`
	Summary     string            `json:"summary"`
	URL         string            `json:"url,omitempty"`
	Fingerprint string            `json:"fingerprint,omitempty"`
	Meta        map[string]string `json:"meta,omitempty"`
	CreatedAt   string            `json:"created_at"`
}

// Store is a local, content-addressed evidence cache.
type Store struct {
	root string
}

// NewStore returns the evidence store rooted at dir (typically
// <project>/.autodoc/cache/evidence). The directory is created lazily.
func NewStore(dir string) *Store { return &Store{root: dir} }

// Put persists payload (already free of secrets) and returns its ref.
// Identical payloads deduplicate to the existing ref. The optional scope
// argument is "url[|fingerprint]" and powers invalidation.
func (s *Store) Put(kind, payload, summary string, meta map[string]string, scope ...string) (Ref, error) {
	if len(payload) > maxPayloadBytes {
		return Ref{}, fmt.Errorf("evidence payload too large (%d bytes)", len(payload))
	}
	sum := sha256.Sum256([]byte(kind + "\x00" + payload))
	id := "ev_" + hex.EncodeToString(sum[:])[:10]
	ref := Ref{
		ID:        id,
		Kind:      kind,
		Bytes:     len(payload),
		Summary:   summary,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if len(scope) > 0 {
		parts := strings.SplitN(scope[0], "|", 2)
		ref.URL = parts[0]
		if len(parts) > 1 {
			ref.Fingerprint = parts[1]
		}
	}
	if meta != nil {
		ref.Meta = meta
	}
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return Ref{}, err
	}
	payloadPath := s.payloadPath(id)
	if _, err := os.Stat(payloadPath); os.IsNotExist(err) {
		if err := os.WriteFile(payloadPath, []byte(payload), 0o644); err != nil {
			return Ref{}, err
		}
		if err := s.appendIndex(ref); err != nil {
			return Ref{}, err
		}
	}
	return ref, nil
}

// Get returns the raw payload behind ref (recoverable compression).
func (s *Store) Get(id string) (string, error) {
	b, err := os.ReadFile(s.payloadPath(id))
	if err != nil {
		return "", fmt.Errorf("evidence %s not found", id)
	}
	return string(b), nil
}

// List returns all index entries, insertion order.
func (s *Store) List() []Ref { return s.readIndex() }

// Find returns the most recent ref matching kind and, if non-empty, url.
func (s *Store) Find(kind, url string) (Ref, bool) {
	var match Ref
	found := false
	for _, r := range s.readIndex() {
		if r.Kind != kind {
			continue
		}
		if url != "" && r.URL != url {
			continue
		}
		match, found = r, true
	}
	return match, found
}

// Invalidate removes refs matching the given scope. Empty fields match all.
// It returns the number of refs removed.
func (s *Store) Invalidate(kind, url string) int {
	kept := make([]Ref, 0, 8)
	removed := 0
	for _, r := range s.readIndex() {
		drop := (kind == "" || r.Kind == kind) && (url == "" || r.URL == url)
		if drop {
			os.Remove(s.payloadPath(r.ID))
			removed++
			continue
		}
		kept = append(kept, r)
	}
	if removed > 0 {
		s.writeIndex(kept)
	}
	return removed
}

// Prune drops refs older than the given duration and reports how many went.
func (s *Store) Prune(olderThan time.Duration) int {
	cutoff := time.Now().UTC().Add(-olderThan)
	kept := make([]Ref, 0, 8)
	removed := 0
	for _, r := range s.readIndex() {
		ts, err := time.Parse(time.RFC3339, r.CreatedAt)
		if err == nil && ts.Before(cutoff) {
			os.Remove(s.payloadPath(r.ID))
			removed++
			continue
		}
		kept = append(kept, r)
	}
	if removed > 0 {
		s.writeIndex(kept)
	}
	return removed
}

// Stats summarizes the store for `context stats`.
type Stats struct {
	Refs     int   `json:"refs"`
	RawBytes int64 `json:"raw_bytes"`
}

// Stats reports current store size.
func (s *Store) Stats() Stats {
	var st Stats
	for _, r := range s.readIndex() {
		st.Refs++
		st.RawBytes += int64(r.Bytes)
	}
	return st
}

func (s *Store) payloadPath(id string) string {
	return filepath.Join(s.root, id+".json")
}

func (s *Store) indexPath() string { return filepath.Join(s.root, "index.jsonl") }

func (s *Store) readIndex() []Ref {
	b, err := os.ReadFile(s.indexPath())
	if err != nil {
		return nil
	}
	var refs []Ref
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var r Ref
		if json.Unmarshal([]byte(line), &r) == nil {
			refs = append(refs, r)
		}
	}
	return refs
}

func (s *Store) appendIndex(r Ref) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(s.indexPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	return err
}

func (s *Store) writeIndex(refs []Ref) error {
	if len(refs) == 0 {
		os.Remove(s.indexPath())
		return nil
	}
	var b strings.Builder
	for _, r := range refs {
		line, err := json.Marshal(r)
		if err != nil {
			return err
		}
		b.Write(line)
		b.WriteByte('\n')
	}
	return os.WriteFile(s.indexPath(), []byte(b.String()), 0o644)
}

// Rendered reports what the agent actually sees after adaptive selection.
type Rendered struct {
	Text       string
	Mode       string // raw | compact
	RawBytes   int
	SentBytes  int
	Ref        Ref
	Compressed bool
}

// Render decides what the agent sees. If the compact form costs as much as
// the raw form (small payloads), it returns raw — compression must never pay
// for itself. Otherwise it returns the compact form plus a retrieval handle.
func Render(ref Ref, payload, compact string) Rendered {
	if payload == "" {
		payload = fmt.Sprintf("(evidence %s unavailable)", ref.ID)
	}
	if len(compact) >= len(payload) {
		return Rendered{Text: payload, Mode: "raw", RawBytes: len(payload), SentBytes: len(payload), Ref: ref}
	}
	text := fmt.Sprintf("%s\nevidence: %s (raw %dB, autodoc evidence get %s for full)", compact, ref.ID, ref.Bytes, ref.ID)
	return Rendered{
		Text:       text,
		Mode:       "compact",
		RawBytes:   ref.Bytes,
		SentBytes:  len(text),
		Ref:        ref,
		Compressed: true,
	}
}

// secretPatterns are obvious credential shapes; anything matching is masked
// before persistence. Structured redaction (user-declared selectors) remains
// the caller's responsibility — this is a last-resort net.
var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(password|passwd|secret|token|api[_-]?key|bearer)\s*[:=]\s*["']?[^\s"',}]{6,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),
}

// Redact masks obvious secret shapes in free text or JSON payloads.
func Redact(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			i := strings.IndexAny(m, ":=")
			if i < 0 {
				return "[REDACTED]"
			}
			return m[:i+1] + " [REDACTED]"
		})
	}
	return s
}

// BBox is a pixel bounding box.
type BBox struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Element is one interactive element of a page inventory.
type Element struct {
	Tag         string `json:"tag"`
	ID          string `json:"id,omitempty"`
	Role        string `json:"role,omitempty"`
	Name        string `json:"name,omitempty"` // accessible name / aria-label
	Text        string `json:"text,omitempty"` // visible text (truncated)
	TestID      string `json:"test_id,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Region      string `json:"region"`
	BBox        BBox   `json:"bbox"`
	Visible     bool   `json:"visible"`
	Enabled     bool   `json:"enabled"`
}

// MeaningfulChange is the reusable semantic result of a UI transition. It is
// deliberately smaller than a DOM diff and carries the geometry a director or
// result validator needs without retaining the whole page snapshot.
type MeaningfulChange struct {
	Type        string `json:"type"` // content_added|content_removed|state_changed
	Region      string `json:"region"`
	Description string `json:"description"`
	BBox        BBox   `json:"bbox"`
}

// label returns the best human-facing identifier of the element.
func (e Element) label() string {
	switch {
	case e.Name != "":
		return e.Name
	case e.Text != "":
		return e.Text
	case e.Placeholder != "":
		return e.Placeholder
	case e.TestID != "":
		return e.TestID
	}
	return e.Tag
}

// kind returns role when known, else the tag.
func (e Element) kind() string {
	if e.Role != "" {
		return e.Role
	}
	return e.Tag
}

// CompactLine renders one control as a single short line. It carries only
// what the agent needs to pick a stable locator (role/name/test id/region);
// geometry stays behind the evidence ref (raw payload) and the capture
// backend re-resolves bboxes deterministically at replay.
func (e Element) CompactLine() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s %q", e.kind(), e.label())
	if e.TestID != "" && e.label() != e.TestID {
		fmt.Fprintf(&b, " [%s]", e.TestID)
	} else if e.ID != "" {
		fmt.Fprintf(&b, " [id=%s]", e.ID)
	}
	fmt.Fprintf(&b, " region=%s", e.Region)
	if !e.Enabled {
		b.WriteString(" disabled")
	}
	return b.String()
}

// Inventory is the result of one focused UI query.
type Inventory struct {
	URL         string    `json:"url"`
	Title       string    `json:"title"`
	Controls    []Element `json:"controls"`
	Recommended string    `json:"recommended,omitempty"`
	Intent      string    `json:"intent,omitempty"`
	Confidence  string    `json:"confidence,omitempty"`
	Diagnostic  string    `json:"diagnostic,omitempty"`
}

// RegionSummary returns "region(count)" strings sorted by region name.
func (inv Inventory) RegionSummary() string {
	counts := map[string]int{}
	for _, c := range inv.Controls {
		counts[c.Region]++
	}
	regions := make([]string, 0, len(counts))
	for r := range counts {
		regions = append(regions, r)
	}
	sort.Strings(regions)
	out := make([]string, 0, len(regions))
	for _, r := range regions {
		out = append(out, fmt.Sprintf("%s(%d)", r, counts[r]))
	}
	return strings.Join(out, ",")
}

// CompactInventory renders the compact form of a UI inventory.
func CompactInventory(inv Inventory) string {
	var b strings.Builder
	fmt.Fprintf(&b, "url: %s  title: %s\n", inv.URL, inv.Title)
	if inv.Confidence != "" {
		fmt.Fprintf(&b, "confidence: %s\n", inv.Confidence)
	}
	if inv.Diagnostic != "" {
		fmt.Fprintf(&b, "diagnostic: %s\n", inv.Diagnostic)
	}
	fmt.Fprintf(&b, "regions: %s\n", inv.RegionSummary())
	b.WriteString("controls:\n")
	for _, c := range inv.Controls {
		fmt.Fprintf(&b, "- %s\n", c.CompactLine())
	}
	if inv.Recommended != "" {
		fmt.Fprintf(&b, "recommended: %s\n", inv.Recommended)
	}
	return b.String()
}
