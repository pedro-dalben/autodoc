# Security

- **Login is never recorded.** `autodoc browser login --profile <name>` opens a
  dedicated AutoDoc Chromium profile; you authenticate there (fixture/test
  account). Recording starts after. Never point AutoDoc at your everyday
  browser profile.
- **Fixtures over real data.** Demo credentials (`demo`/`demo1234` style) and
  fixture content only. Real PII, keys, tokens must never appear in
  storyboards, narration, selectors, screenshots, video, logs, or traces.
- **Tutorial about login?** Fixture credentials only, and the password input
  is masked deterministically.
- **Redaction before/during capture** (`redact.selectors`, auto-mask of
  `input[type=password]` via DOM overlay/blur/value replacement) — not fragile
  frame post-processing. Safety never depends on the skill text alone:
  `compile`/`record` run a secret scan and fail on sentinels
  (`password`, `api_key`, `ghp_`, `sk-live`, credential-like fill values…).
- **Artifact boundaries.** Publishable: `docs/autodoc/<tutorial>/`. Never
  publish `.autodoc/_work/`, profiles, or `storage-state.json`.
- **Secrets handling.** API keys live in env vars (`tts.api_key_env`);
  `doctor` redacts secrets in all output. Never commit keys to `autodoc.toml`.
