package tts_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pedro-dalben/autodoc/internal/recipe"
	"github.com/pedro-dalben/autodoc/internal/tts"
)

func TestSegmentHashConsidersSoundParams(t *testing.T) {
	a := tts.SegmentHash("text", "v1", "m", "pt-BR", 1.0)
	if a != recipe.SpeechHash("text", "v1", "m", "pt-BR", 1.0) {
		t.Fatal("hash mismatch with recipe")
	}
	if tts.SegmentHash("text", "v1", "m", "pt-BR", 1.0) == tts.SegmentHash("text", "v1", "m", "pt-BR", 1.5) {
		t.Fatal("speed must affect hash")
	}
}

func TestCacheHitMissMiss(t *testing.T) {
	dir := t.TempDir()
	c := &tts.Cache{Dir: dir}
	h := tts.SegmentHash("hello", "v", "m", "pt-BR", 1.0)
	if _, ok := c.Lookup(h); ok {
		t.Fatal("expected miss")
	}
	wav := tts.SilentWAV(1.0, 22050)
	p, err := c.Store(h, wav)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := c.Lookup(h); !ok {
		t.Fatal("expected hit")
	}
	if p != filepath.Join(dir, h+".wav") {
		t.Fatalf("bad path %s", p)
	}
}

func TestIncrementalRegeneration(t *testing.T) {
	texts := []string{"speech A", "speech B", "speech C"}
	hashes := map[string]string{}
	for _, tx := range texts {
		hashes[tx] = tts.SegmentHash(tx, "v", "m", "pt-BR", 1.0)
	}
	dir := t.TempDir()
	c := &tts.Cache{Dir: dir}
	for _, tx := range texts {
		_, _ = c.Store(hashes[tx], tts.SilentWAV(0.5, 22050))
	}
	hits := 0
	for _, tx := range texts {
		if _, ok := c.Lookup(hashes[tx]); ok {
			hits++
		}
	}
	if hits != 3 {
		t.Fatalf("expected 3 hits, got %d", hits)
	}
	newHash := tts.SegmentHash("speech B edited", "v", "m", "pt-BR", 1.0)
	status := map[string]string{}
	for _, tx := range []string{"speech A", "speech B edited", "speech C"} {
		h := hashes[tx]
		if tx == "speech B edited" {
			h = newHash
		}
		if _, ok := c.Lookup(h); ok {
			status[tx] = "hit"
		} else {
			status[tx] = "miss"
		}
	}
	if status["speech A"] != "hit" || status["speech B edited"] != "miss" || status["speech C"] != "hit" {
		t.Fatalf("bad incremental status: %v", status)
	}
}

func TestOpenAICompatibleEndpoint(t *testing.T) {
	var gotPath, gotModel, gotVoice string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		gotModel, _ = body["model"].(string)
		gotVoice, _ = body["voice"].(string)
		w.Header().Set("Content-Type", "audio/wav")
		w.Write(tts.SilentWAV(0.6, 22050))
	}))
	defer srv.Close()
	p := &tts.OpenAICompatible{BaseURL: srv.URL + "/v1"}
	wav, err := p.Synthesize(context.Background(), tts.Request{Text: "Olá", Voice: "v", Model: "kokoro", Language: "pt-BR", Speed: 1.0, Format: "wav"})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/v1/audio/speech" {
		t.Fatalf("wrong endpoint %s", gotPath)
	}
	if gotModel != "kokoro" || gotVoice != "v" {
		t.Fatalf("bad payload %s %s", gotModel, gotVoice)
	}
	dur, err := tts.WavDurationSeconds(wav)
	if err != nil || dur < 0.5 || dur > 0.8 {
		t.Fatalf("bad duration %v %v", dur, err)
	}
}

func TestOpenAICompatibleRejectsNonWAV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"error":"nope"}`))
	}))
	defer srv.Close()
	p := &tts.OpenAICompatible{BaseURL: srv.URL}
	if _, err := p.Synthesize(context.Background(), tts.Request{Text: "hi"}); err == nil {
		t.Fatal("expected error for non-wav")
	}
}

func TestDisabledProvider(t *testing.T) {
	wav, err := tts.Disabled{}.Synthesize(context.Background(), tts.Request{Text: "hello world foo bar", Speed: 1.0})
	if err != nil {
		t.Fatal(err)
	}
	if !tts.IsWAV(wav) {
		t.Fatal("not wav")
	}
}

func TestNoSecretsInCache(t *testing.T) {
	dir := t.TempDir()
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatal("cache dir should start empty")
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), "sk-") {
			t.Fatal("secret in cache name")
		}
	}
}
