# TTS

AutoDoc talks to TTS through one contract: an OpenAI-compatible
`/audio/speech` endpoint that returns WAV. AutoDoc POSTs
`{model, input, voice, response_format, speed, language}` to
`<base_url>/audio/speech`. That server can run on your machine or be a
remote API. AutoDoc never calls a TTS SDK directly.

V1 supports two providers. The setup wizard only offers what works.

## openai-compatible

```toml
[tts]
provider = "openai-compatible"
base_url = "http://localhost:8880/v1"
api_key_env = ""
model = "kokoro"
voice = "af_bella"
language = "pt-BR"
speed = 1.0
response_format = "wav"
```

The same interface drives OpenAI itself (`base_url =
https://api.openai.com/v1`, `api_key_env` naming the env var holding the key,
`model = "tts-1"`, `voice = "alloy"`).

Check the endpoint before recording:

```bash
autodoc tts check     # one-sentence synthesis; reports WAV duration
autodoc doctor        # also probes endpoint reachability
```

## disabled

Silent pacing audio with estimated durations. Subtitles, written docs, and
keyless CI pipelines keep working; the video has no voiceover:

```toml
[tts]
provider = "disabled"
```

## Segment cache

Each `speech` event is an independent unit: `speech-<scene>-<beat>-<NNN>.wav`,
keyed by `hash(text, voice, model, language, speed)`. Unchanged segments are
cache hits and are never re-synthesized. `autodoc tts` prints `hit`/`miss`
per segment. CI never touches a paid API; a real-OpenAI smoke test is opt-in
via env var only.

No STT exists anywhere in the pipeline. No second LLM, vision model, or
video API is used.
