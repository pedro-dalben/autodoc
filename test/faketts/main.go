package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/pedro-dalben/autodoc/internal/tts"
)

func main() {
	addr := ":8880"
	mux := http.NewServeMux()
	mux.HandleFunc("/v1/audio/speech", handleSpeech)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("ok")) })
	fmt.Printf("fake tts on %s\n", addr)
	_ = http.ListenAndServe(addr, mux)
}

func handleSpeech(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Model          string  `json:"model"`
		Input          string  `json:"input"`
		Voice          string  `json:"voice"`
		ResponseFormat string  `json:"response_format"`
		Speed          float64 `json:"speed"`
		Language       string  `json:"language"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"invalid json"}`)
		return
	}
	if strings.TrimSpace(body.Input) == "" {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"input required"}`)
		return
	}
	speed := body.Speed
	if speed == 0 {
		speed = 1.0
	}
	dur := tts.EstimateDuration(body.Input, speed)
	if dur < 0.4 {
		dur = 0.4
	}
	freq := 220.0
	h := 0
	for _, c := range body.Input {
		h = (h*31 + int(c)) % 200
	}
	freq = 180 + float64(h)
	wav := tts.SineWAV(dur, 22050, freq)
	w.Header().Set("Content-Type", "audio/wav")
	w.Header().Set("X-Fake-Duration", fmt.Sprintf("%.3f", dur))
	http.ServeContent(w, r, "speech.wav", time.Now(), strings.NewReader(string(wav)))
}
