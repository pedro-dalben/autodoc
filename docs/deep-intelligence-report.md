# Deep intelligence, token economy, and human director

## Delivered surfaces

- Focused UI discovery stores full inventories locally and returns ranked,
  compact results with a recovery reference.
- Evidence is content-addressed, deduplicated, redacted, and rendered raw when
  compacting would cost more.
- `agent bootstrap` selects a task mode; narrow guides and warm capsules avoid
  repeating the full protocol. A capsule hashes only its declared source files.
- Compact storyboards support named targets, defaults, and stable HTML `id`
  targets. Hashing covers every execution-affecting target and action field.
- Cinematic V2 adds continuity-aware direction, dynamic readable result holds,
  and QA that rejects only crops made unsafe by an actual zoom.

## Measured evidence (2026-09-04)

| Check | Result |
| --- | --- |
| Canonical skill | 12,349 B / 1,692 words to 2,251 B / 315 words (81.8% fewer bytes) |
| Focused browser context in the real run | 424 B visible to the agent (about 106 estimated tokens) |
| Warm capsule | `REUSE` for the chat-send flow after hashing only its two declared UI sources |
| Real Integrar Plus capture | 8/8 sync checks, maximum drift +1 ms |
| Cinematic QA | 12/12 gates, score 100/100 |
| Final media | H.264 + AAC, 1280x720, 24.03 s |

The real run used an isolated fixture conversation and a temporary work
directory. Its export contains the MP4, storyboard, subtitle files,
timeline/final timeline, thumbnail, and metadata. Credentials were supplied
through environment references and were neither written to the storyboard nor
stored in an AutoDoc profile.

## Human review

Three sampled frames cover selection, typing, and the final sent state. The
target remains readable, the action is visible, and the final state holds long
enough to verify. The captured app displays a pre-existing `Bullet Warnings`
browser badge in the lower-left corner; it is recorded here rather than hidden.

Audio was verified as a 44.1 kHz stereo AAC stream and its narration was
generated from the fixture-only script. This report does not claim a human
listening judgment beyond that technical verification.

## Reproducible gates

```sh
go test ./internal/...
go vet ./...
go test -short ./...
autodoc validate --sync
autodoc validate --cinematic
```

All listed Go checks passed on 2026-09-04. The real capture and render passed
both AutoDoc validations above.
