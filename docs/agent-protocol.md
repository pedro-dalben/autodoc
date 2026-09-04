# Low-context agent protocol

AutoDoc keeps reasoning outside the executable. The agent asks for the next
smallest useful fact; AutoDoc stores raw evidence locally and executes the
validated storyboard deterministically.

## Bootstrap

Run `autodoc agent bootstrap --goal "..."` first. It selects one mode:
`CREATE`, `UPDATE`, `RETAKE`, `RENDER`, `DEBUG`, or `PUBLISH`; reports cached
auth/evidence/workspace state; and prints only the next commands for that mode.

For a new tutorial, use this order: existing artifacts, narrow route search,
`autodoc ui query`, semantic storyboard, validate, TTS, record, render,
cinematic validation. For a change, inspect the git/UI delta, patch the affected
scene, and retake it. Do not rediscover a tutorial merely to render or export.

## Browser evidence

`autodoc ui query --url URL --intent "send message"` returns only ranked,
relevant controls. `autodoc ui diff` records one action's semantic changes:
content added/removed or state changed, region, and bounding box. The raw
inventories are content-addressed under `.autodoc/cache/evidence`; use
`autodoc evidence get ev_…` only for escalation.

The evidence renderer selects raw output when its compact representation would
cost at least as many bytes. Otherwise it emits a compact representation plus
a recovery reference. Identical payloads share one ref.

## Context accounting

`autodoc context stats` aggregates agent-visible bytes by phase and marks
bytes/4 token values as estimates. Provider-reported usage is only shown when
explicitly logged as such. AutoDoc logs its focused browser outputs; harnesses
can log skill, tool catalog, repository, and storyboard observations with
`autodoc context log`.

## Measured skill baseline

The V2 canonical skill was 12,349 bytes / 1,692 whitespace-delimited words.
The V3 bootstrap is 2,251 bytes / 315 words: **81.8% fewer bytes**. The gain is
from routing task-specific details to CLI state and existing docs, not from
removing validation, secret, or deterministic-recording invariants.

## Cinematic direction

Cinematic V2 remains the runtime director: semantic beat planning, attention,
camera continuity, dynamic result holds, wait compression, actual-event
reconciliation, and cinematic QA. Focused UI diffs add a reusable
`MeaningfulChange` surface for discovery and future runtime result evidence;
they do not replace capture or invent browser actions.
