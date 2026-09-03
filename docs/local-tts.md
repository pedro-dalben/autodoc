# Local TTS

Any server implementing the OpenAI `/audio/speech` contract works, including
fully local ones. Example with a Kokoro-compatible server on port 8880:

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

Test it:

```bash
curl -X POST http://localhost:8880/v1/audio/speech \
  -H 'Content-Type: application/json' \
  -d '{"model":"kokoro","input":"Teste do AutoDoc.","voice":"af_bella","response_format":"wav"}' \
  -o probe.wav
ffprobe -v error -show_entries format=duration probe.wav
autodoc tts --storyboard storyboard.yml   # expect all `miss` first, `hit` after
```

For CI and this repo's E2E, `test/faketts` is a deterministic fake
(OpenAI-compatible endpoint returning hash-derived sine WAVs) — no GPU, no
weights, no network beyond localhost.
