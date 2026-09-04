---
name: autodoc
description: Create, update, record, render, or publish a narrated UI tutorial video.
version: "3"
---

# AutoDoc

AutoDoc is deterministic execution; you provide product reasoning. Start with:

```sh
autodoc agent bootstrap --goal "<user request>"
```

It selects `CREATE`, `UPDATE`, `RETAKE`, `RENDER`, `DEBUG`, or `PUBLISH` and
reports reusable local state. Follow only that mode's next steps.

## Always

- Never record discovery or login; use fixture data and dedicated AutoDoc auth.
- Reuse storyboard, auth, evidence, and prior workspace artifacts before new discovery.
- Use `autodoc ui query --url URL --intent "..."`; use `ui diff` after one exploratory action. Read an `evidence` ref only when the compact result is insufficient.
- `storyboard.yml` is hand-authored source; `_work/recipe.json` is generated.
- Use stable locators: test id, role/name, label; never positional CSS/XPath.
- Keep secrets and real PII out of storyboards, narration, evidence, frames, and logs.
- Validate before recording. Final capture is always `autodoc record`, never an interactive browser recording.

## Mode router

`CREATE`: narrow route search, focused UI query, semantic storyboard, then
`validate → tts → record → render → validate --cinematic`.

`UPDATE` / `RETAKE`: inspect the existing storyboard and relevant git/UI delta;
patch only the affected scene, verify its target, retake it, render.

`RENDER` / `PUBLISH`: do not rediscover. Render or validate/export the existing
storyboard.

`DEBUG`: run the narrow failing command; retrieve raw evidence only when its
compact ref cannot explain the failure.

## Semantic authoring

Narrate purpose and result, not cursor movements. Keep actions/waits/speech in
their real order. Leave camera, cursor, zoom, pacing, and result holds to the
Cinematic Director unless a specific override is necessary. Use a declared
`result_target` when the user must see a result.

## Escalation

Use `autodoc evidence get REF` for raw browser evidence and `autodoc context
stats` for local byte/token estimates. Estimates are not provider usage.

Detailed reference: `docs/storyboard.md`, `docs/cinematic-v2.md`,
`docs/security.md`, and `docs/troubleshooting.md`—open only the topic needed.
