# Evidence report

Each pipeline run aggregates its own facts into a deterministic local
document: `.autodoc/_work/<run>/evidence.json` (plus a human-readable
`evidence.txt` beside it). No telemetry leaves the machine; the report
is observation only and never fails a render.

## Generate

The `tts`, `record`, `render` and `export` commands refresh the current
run's report automatically. To regenerate or inspect on demand:

```bash
autodoc evidence report            # latest complete run, text summary
autodoc evidence report --run 20260102-150405
autodoc evidence report --format json
autodoc evidence compare <run-a> <run-b>
```

`compare` prints a Before/After/Delta table (TTS hit ratio, capture
reuse, retakes, max drift, output duration). No global score is
computed; directionless deltas stay unsigned.

## Format

`evidence.json` carries `"version": "v1"` (explicit integer, never a
timestamp). Sections: pipeline counts (scenes/beats/actions/waits/
speech), TTS (segments, cache hits/misses/ratio, synthesized seconds),
capture (recorded/reused/retaken/missing per scene id, reuse ratio),
recovery (failed/retaken scenes; manual interventions live in the
dogfood log, never inferred), sync (max/mean drift, outside-threshold
count, pass), cinematic (directive requested/honored/ignored, QA
pass/score), security (compile-time secret scan, redact selector count,
password masking flag), output (ffprobe duration, resolution, fps,
codecs, bytes when ffprobe exists).

Missing artifacts yield `available: false` sections, never errors, so
partial and failed runs still report. Capture reuse mirrors the
pipeline's scene-level rule: a sibling raw counts when its scene hash
matches, even across storyboard changes. TTS facts reuse the newest
same-hash sibling report.

## Privacy

IDs, hashes, counts and metrics only. Never speech text, URLs, DOM,
storage state, cookies, secret values or headers.
