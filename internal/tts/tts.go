package tts

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/recipe"
)

type Request struct {
	Text     string
	Voice    string
	Model    string
	Language string
	Speed    float64
	Format   string
}

type Result struct {
	ID       string
	Text     string
	Hash     string
	Path     string
	Cached   bool
	Duration float64
	Bytes    int64
}

type Provider interface {
	Name() string
	Synthesize(ctx context.Context, req Request) ([]byte, error)
}

func SegmentHash(text, voice, model, language string, speed float64) string {
	return recipe.SpeechHash(text, voice, model, language, speed)
}

// CacheKey scopes a speech content hash to the synthesis backend that
// produced it. Different providers (or response formats) render different
// bytes and durations for identical text+voice, so sharing one cache entry
// across them would desync the timeline. Provider switches miss once and
// repopulate; they never collide.
func CacheKey(speechHash, provider, format string) string {
	h := sha256.New()
	fmt.Fprintf(h, "tts-cache-v1|%s|%s|%s", speechHash, provider, format)
	return hex.EncodeToString(h.Sum(nil))[:16]
}

type Cache struct {
	Dir string
}

func (c *Cache) PathFor(hash string) string {
	return filepath.Join(c.Dir, hash+".wav")
}

func (c *Cache) Lookup(hash string) (string, bool) {
	p := c.PathFor(hash)
	st, err := os.Stat(p)
	if err != nil || st.Size() == 0 {
		return "", false
	}
	return p, true
}

func (c *Cache) Store(hash string, wav []byte) (string, error) {
	if err := os.MkdirAll(c.Dir, 0o755); err != nil {
		return "", err
	}
	p := c.PathFor(hash)
	if err := os.WriteFile(p, wav, 0o644); err != nil {
		return "", err
	}
	return p, nil
}

type OpenAICompatible struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
}

func (o *OpenAICompatible) Name() string { return "openai-compatible" }

func (o *OpenAICompatible) endpoint() string {
	return strings.TrimRight(o.BaseURL, "/") + "/audio/speech"
}

func (o *OpenAICompatible) Synthesize(ctx context.Context, req Request) ([]byte, error) {
	format := req.Format
	if format == "" {
		format = "wav"
	}
	body := map[string]any{
		"model":           req.Model,
		"input":           req.Text,
		"voice":           req.Voice,
		"response_format": format,
	}
	if req.Speed != 0 {
		body["speed"] = req.Speed
	}
	if req.Language != "" {
		body["language"] = req.Language
	}
	payload, _ := json.Marshal(body)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, o.endpoint(), bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if o.APIKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	}
	client := o.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 120 * time.Second}
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("tts request to %s failed: %w", o.endpoint(), err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 100<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("tts endpoint %s returned %d: %s", o.endpoint(), resp.StatusCode, truncate(string(data), 500))
	}
	if !IsWAV(data) {
		return nil, fmt.Errorf("tts endpoint did not return WAV (got %d bytes, content-type %q)", len(data), resp.Header.Get("Content-Type"))
	}
	return data, nil
}

type Disabled struct{}

func (Disabled) Name() string { return "disabled" }

func (Disabled) Synthesize(_ context.Context, req Request) ([]byte, error) {
	dur := EstimateDuration(req.Text, req.Speed)
	return SilentWAV(dur, 22050), nil
}

func EstimateDuration(text string, speed float64) float64 {
	if speed <= 0 {
		speed = 1.0
	}
	words := len(strings.Fields(text))
	if words == 0 {
		return 0.5
	}
	cps := 15.0
	dur := float64(len([]rune(text))) / cps / speed
	if alt := float64(words) / 2.5 / speed; alt > dur {
		dur = alt
	}
	if dur < 0.5 {
		dur = 0.5
	}
	return dur
}

func IsWAV(data []byte) bool {
	return len(data) >= 12 && string(data[0:4]) == "RIFF" && string(data[8:12]) == "WAVE"
}

func WavDurationSeconds(data []byte) (float64, error) {
	if !IsWAV(data) {
		return 0, fmt.Errorf("not a WAV file")
	}
	if len(data) < 44 {
		return 0, fmt.Errorf("wav too short")
	}
	numChannels := binary.LittleEndian.Uint16(data[22:24])
	sampleRate := binary.LittleEndian.Uint32(data[24:28])
	bitsPerSample := binary.LittleEndian.Uint16(data[34:36])
	if sampleRate == 0 || numChannels == 0 || bitsPerSample == 0 {
		return 0, fmt.Errorf("invalid wav header")
	}
	dataSize := uint32(len(data) - 44)
	if string(data[36:40]) == "data" {
		dataSize = binary.LittleEndian.Uint32(data[40:44])
	}
	bytesPerSec := float64(sampleRate) * float64(numChannels) * float64(bitsPerSample) / 8.0
	if bytesPerSec == 0 {
		return 0, fmt.Errorf("invalid wav format")
	}
	return float64(dataSize) / bytesPerSec, nil
}

func SilentWAV(seconds float64, sampleRate int) []byte {
	if seconds < 0.1 {
		seconds = 0.1
	}
	n := int(math.Ceil(seconds * float64(sampleRate)))
	data := make([]byte, 44+n*2)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+n*2))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(data[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(n*2))
	return data
}

func SineWAV(seconds float64, sampleRate int, freqHz float64) []byte {
	if seconds < 0.1 {
		seconds = 0.1
	}
	n := int(math.Ceil(seconds * float64(sampleRate)))
	data := make([]byte, 44+n*2)
	copy(data[0:4], "RIFF")
	binary.LittleEndian.PutUint32(data[4:8], uint32(36+n*2))
	copy(data[8:12], "WAVE")
	copy(data[12:16], "fmt ")
	binary.LittleEndian.PutUint32(data[16:20], 16)
	binary.LittleEndian.PutUint16(data[20:22], 1)
	binary.LittleEndian.PutUint16(data[22:24], 1)
	binary.LittleEndian.PutUint32(data[24:28], uint32(sampleRate))
	binary.LittleEndian.PutUint32(data[28:32], uint32(sampleRate*2))
	binary.LittleEndian.PutUint16(data[32:34], 2)
	binary.LittleEndian.PutUint16(data[34:36], 16)
	copy(data[36:40], "data")
	binary.LittleEndian.PutUint32(data[40:44], uint32(n*2))
	for i := range n {
		t := float64(i) / float64(sampleRate)
		env := 1.0
		fade := int(float64(sampleRate) * 0.02)
		if i < fade {
			env = float64(i) / float64(fade)
		} else if i > n-fade {
			env = float64(n-i) / float64(fade)
		}
		v := int16(math.Sin(2*math.Pi*freqHz*t) * 12000 * env)
		binary.LittleEndian.PutUint16(data[44+i*2:], uint16(v))
	}
	return data
}

func FileIDForTest(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])[:12]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
