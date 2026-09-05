# CLI reference

_Generated from the Cobra command tree. Do not edit by hand; run the test with AUTODOC_UPDATE_CLI_DOCS=1 to regenerate._

## `autodoc agent bootstrap`

Show existing knowledge and only the next workflow steps

```
--goal string         user tutorial request
      --storyboard string   existing storyboard path
```


## `autodoc agent capsule-create`

Save a compact warm-run capsule

```
--flow string         stable flow name
      --source strings      relevant source file (repeatable)
      --storyboard string   storyboard path
```


## `autodoc agent capsule-status`

Report whether a capsule's relevant sources changed

```
--capsule string      capsule JSON path
      --storyboard string   storyboard path to compare against the capsule hash (detects storyboard edits)
```


## `autodoc agent guide`

Read one small on-demand workflow module


## `autodoc agent state`

Report current tutorial lifecycle state and next recommended commands

```
--json                output JSON format
      --storyboard string   storyboard path
```


## `autodoc auth bootstrap`

Run setup.sequence off-camera and cache the session (secrets via env, never recorded)

```
--storyboard string   path to storyboard.yml
```


## `autodoc browser check`

Check driver + browser status


## `autodoc browser install`

Install Playwright browsers (chromium)


## `autodoc browser login`

Open dedicated profile browser for manual authentication (never recorded)

```
--profile string   profile name (default "docs")
      --url string       url to open for login
```


## `autodoc compile`

Compile storyboard.yml to deterministic _work/recipe.json

```
--storyboard string   path to storyboard.yml
```


## `autodoc context log`

Log one context observation (--phase --bytes [--label] [--source])

```
--bytes int       agent-visible bytes observed
      --label string    optional label
      --phase string    skill|tool_catalog|repo|browser|cli|storyboard|evidence
      --source string   who measured it (agent|autodoc) (default "agent")
```


## `autodoc context reset`

Erase the context usage log


## `autodoc context stats`

Show context budget by phase


## `autodoc diagnose`

Run self-diagnosis on system environment and latest project run

```
--json   output JSON format
```


## `autodoc doctor`

Diagnose autodoc, media, browser, TTS, harnesses, skill

```
--json   JSON output
```


## `autodoc evidence get`

Print the raw payload behind an evidence ref


## `autodoc evidence invalidate`

Invalidate evidence by --kind and/or --url (empty = all)

```
--kind string   evidence kind (ui_inventory|ui_diff|page_state)
      --url string    page URL scope
```


## `autodoc evidence prune`

Drop evidence older than --older-than (default 168h)

```
--older-than string   drop refs older than this (Go duration) (default "168h")
```


## `autodoc evidence stats`

Evidence store size summary


## `autodoc explain`

Explain cinematic director decisions (camera, waits, result holds, attention)

```
--json                output JSON format
      --run-dir string      path to specific run directory in .autodoc/_work/
      --storyboard string   path to storyboard.yml
```


## `autodoc export`

Export publishable bundle docs/autodoc/<tutorial>/

```
--mp4 string          source mp4
      --out string          output dir
      --storyboard string   path to storyboard.yml
```


## `autodoc init`

Interactive setup: harnesses, TTS, browser, skill, MCP

```
--global                write machine-wide ~/.config/autodoc/autodoc.toml instead of ./autodoc.toml
      --harness stringArray   harness to configure (repeatable)
      --non-interactive       skip prompts
      --tts-base-url string   tts base url
      --tts-model string      tts model
      --tts-provider string   tts provider (openai-compatible|disabled)
      --tts-voice string      tts voice
```


## `autodoc mcp`

Run AutoDoc MCP server (stdio)


## `autodoc record`

Deterministic browser replay + scene capture (recordVideo backend)

```
--backend string      capture backend (default "playwright")
      --headless            headless browser (default true)
      --no-tts              skip TTS synthesis
      --retake string       re-record a single scene id
      --storyboard string   path to storyboard.yml
```


## `autodoc render`

Render final MP4 from the reconciled timeline (FFmpeg H.264+AAC)

```
--debug-cues          write cues-debug.json (bbox, zooms, segments)
      --debug-timeline      print reconciled segment table
      --fps int             output fps
      --height int          output height
      --no-zoom             disable render-time camera focus
      --out string          output mp4 path
      --storyboard string   path to storyboard.yml
      --width int           output width
```


## `autodoc storyboard validate`

Validate storyboard.yml

```
--storyboard string   path to storyboard.yml
```


## `autodoc tts`

Synthesize TTS segments (cached per speech segment)

```
--storyboard string   path to storyboard.yml
```

Subcommands:

- `autodoc tts check` — Probe the configured TTS endpoint with a one-sentence synthesis


## `autodoc tts check`

Probe the configured TTS endpoint with a one-sentence synthesis


## `autodoc ui diff`

Inspect BEFORE, perform ONE action, inspect AFTER, print semantic delta

```sh
autodoc ui diff --base-url http://localhost:3000 --url /admin/chat --fill "Digite uma mensagem=Texto"
autodoc ui diff --base-url http://localhost:3000 --url /admin/chat --click "Enviar mensagem"
```

```
--auth string       auth profile name or storage-state.json path
      --base-url string   base URL for relative paths
      --click string      click the element with this accessible name/text
      --fill string       fill NAME=VALUE on the element named NAME
      --json              print diff JSON instead of compact view
      --url string        page path or absolute URL
```


## `autodoc ui query`

Inspect a page for controls relevant to an intent

```sh
autodoc ui query --base-url http://localhost:3000 --url /admin/chat --intent "enviar mensagem"
autodoc ui query --url http://localhost:8099/materiais --limit 8
```

```
--auth string       auth profile name or storage-state.json path
      --base-url string   base URL for relative paths (e.g. http://localhost:3000)
      --intent string     what you are looking for, e.g. "enviar mensagem"
      --json              print full inventory JSON instead of compact view
      --limit int         max controls returned (default 12)
      --url string        page path (with --base-url) or absolute URL
```


## `autodoc uninstall`

Remove only AutoDoc-owned installation artifacts

```
--harness stringArray   only uninstall these harnesses
      --restore-backup        explicitly restore pre-install backups
```


## `autodoc validate`

Validate storyboard + timeline + artifacts coherence

```
--cinematic           report Cinematic V2 direction QA (fails when any gate fails)
      --storyboard string   path to storyboard.yml
      --sync                only report A/V synchronization (fails when drift exceeds tolerance)
```


## `autodoc version`

Print version

