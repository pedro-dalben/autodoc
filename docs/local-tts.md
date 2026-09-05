# Local TTS

Any server implementing the OpenAI `/audio/speech` contract works, including
fully local ones. One tested path is Kokoro-FastAPI
([remsky/Kokoro-FastAPI](https://github.com/remsky/Kokoro-FastAPI)), a
Dockerized OpenAI-compatible wrapper around Kokoro-82M. Images are published
for CPU and NVIDIA GPU on amd64 and arm64.

## 1. Start the server

CPU-only machine:

```bash
docker run -p 8880:8880 ghcr.io/remsky/kokoro-fastapi-cpu:latest
```

NVIDIA GPU:

```bash
docker run --gpus all -p 8880:8880 ghcr.io/remsky/kokoro-fastapi-gpu:latest
```

Pin a release tag instead of `latest` for reproducible setups. Apple Silicon
and remaining variants are covered in the
[upstream README](https://github.com/remsky/Kokoro-FastAPI#get-started).

## 2. Verify the endpoint

```bash
curl -X POST http://localhost:8880/v1/audio/speech \
  -H 'Content-Type: application/json' \
  -d '{"model":"kokoro","input":"AutoDoc audio check.","voice":"af_bella","response_format":"wav"}' \
  -o probe.wav
ffprobe -v error -show_entries format=duration probe.wav
```

AutoDoc needs `response_format: wav`. If the probe returns audio, the server
is up.

## 3. Configure AutoDoc

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
Upstream ships voices per supported language (English, Spanish, French,
Hindi, Italian, Japanese, Brazilian Portuguese, Mandarin). Pick a voice that
matches `language`; the upstream repo lists the available names.

## 4. Test through AutoDoc

```bash
autodoc tts check                          # one-sentence synthesis
autodoc tts --storyboard storyboard.yml    # first run: all `miss`; rerun: `hit`
autodoc doctor                             # TTS section must pass
```

## 5. First synthesis

Record or render any storyboard; narration now uses the local server. Only
edited segments are re-synthesized afterwards.

## Notes

- For CI and this repo's E2E, `test/faketts` is a deterministic fake
  (OpenAI-compatible endpoint returning hash-derived sine WAVs). No GPU, no
  weights, no network beyond localhost.
- Keep API keys in env vars referenced by `api_key_env`. Never commit keys
  to `autodoc.toml`.
- No narration at all: `provider = "disabled"` (see [tts.md](tts.md)).
