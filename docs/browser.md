# Browser and auth

Playwright browser assets are separate from the AutoDoc binary. `go install`
or the release archive alone does not make Playwright functional.

## Check and install

```bash
autodoc browser check     # driver + browser status
autodoc browser install   # downloads Playwright chromium assets
autodoc doctor            # same status inside the full report
```

`browser install` downloads browser builds into the Playwright cache
(`~/.cache/ms-playwright`). It needs network access once; afterwards
recording works offline.

## Login (off-camera)

```bash
autodoc browser login --profile docs
autodoc browser login --profile docs --url https://app.example.com/login
```

This opens a Chromium window with a dedicated AutoDoc profile. Sign in with
a fixture/test account, then press Enter in the terminal. The session is
saved under the profile directory and reused by later `record` runs.

Rules:

- Login is never recorded. Recording starts after this step.
- Never use your everyday browser profile.
- Never put real credentials in storyboards. Fixture accounts only.
- A tutorial about login uses fixture credentials plus deterministic
  password masking.

In headless CI there is no window to click through. Either run under
`xvfb-run` or provision the profile's `storage-state.json` beforehand. See
[troubleshooting.md](troubleshooting.md).

## Setup sequence in storyboards

Off-camera login steps can also live in the storyboard's `setup.sequence`
with `secret_ref: env:VAR_NAME`. The sequence runs before capture starts and
its storage state is cached under `.autodoc/cache/auth/` for reuse.
`autodoc auth bootstrap` runs it once on demand. Secrets resolve from env at
record time and never land in artifacts.

## Profiles and storage state

- Profile dir: `<data-dir>/profiles/<name>` (`autodoc browser login`
  prints the exact path).
- Never publish profiles or `storage-state.json`. The publishable bundle is
  `docs/autodoc/<tutorial>/`; see [security.md](security.md).
