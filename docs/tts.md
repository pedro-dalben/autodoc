# TTS

V1 supports exactly two providers. The wizard only offers what works.

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

The provider POSTs to `<base_url>/audio/speech` with
`{model, input, voice, response_format, speed, language}` and requires a WAV
response. The same interface drives OpenAI itself (`base_url =
https://api.openai.com/v1`, `api_key_env` naming the env var holding the key,
`model = "tts-1"`, `voice = "alloy"`).

## disabled

Silent pacing audio (estimated durations), subtitles, written docs, and
keyless CI pipelines:

```toml
[tts]
provider = "disabled"
```

## Segment cache

Each `speech` event is an independent unit: `speech-<scene>-<beat>-<NNN>.wav`,
keyed by `hash(text, voice, model, language, speed)`. Unchanged segments are
cache hits — never re-synthesized. `autodoc tts` prints `hit`/`miss` per
segment. CI never touches a paid API; a real-OpenAI smoke test is opt-in via
env var only.

No STT exists anywhere in the pipeline. No second LLM, vision model, or video
API is used. Architecture leaves room for future providers (e.g. ElevenLabs)
but none are advertised until a working adapter lands.
