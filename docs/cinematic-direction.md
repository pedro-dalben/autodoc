# Directing your tutorial video

AutoDoc directs every video automatically: camera, zoom, cursor, clicks,
typing, keyboard shortcuts, spotlight, callouts, result holds. You only
speak up when you have a preference. Unspecified controls stay `auto`.

No YAML, coordinates, or FFmpeg required. Just say what you want:

```text
"Make the tutorial without zoom."
"Use strong zoom while editing the profile."
"Hide the cursor."
"Make clicks easier to see."
"Show keyboard shortcuts."
"Highlight the Save button."
"Use a calmer style."
"Hold the result longer so it can be read."
```

## What you can direct

| Control    | Values                                                        |
|------------|---------------------------------------------------------------|
| camera     | auto, static, subtle, dynamic, follow, focus, wide            |
| zoom       | auto, off, subtle, medium, strong, extreme                    |
| cursor     | auto, show, hide (plus a discreet halo for training)          |
| click      | auto, off, subtle, strong (ripple, ring, pulse, highlight)    |
| typing     | auto, instant, natural, slow, fast                            |
| keyboard   | auto, off, shortcuts, all                                     |
| spotlight  | auto, off, subtle, medium, strong                             |
| focus      | auto, off, outline, pulse                                     |
| callout    | auto, off, on                                                 |
| result     | auto, off, emphasize                                          |
| hold       | auto, short, normal, long                                     |
| transition | auto, cut, smooth, none                                       |
| camera lock| auto, off, on (pin the shot for a whole scene)                |

Negative directives always win at their scope: "no zoom", "no
spotlight", "no callouts". A scoped exception still applies ("no zoom,
but strong zoom on the login form" keeps the form zoomed).

## Styles

`"Make it calmer"` → minimal. `"More dynamic"` → dynamic.
`"Step-by-step training"` → training (keyboard, focus, longer holds).
Anything specific you add overrides the style where they overlap.

## Precedence

Action beats scene beats tutorial beats style beats the automatic
director. `autodoc explain` shows every decision with its source
(`[user:tutorial]`, `[user:scene]`, `[preset]`, `[director]`).

## Visual-only changes reuse the recording

Changing zoom, spotlight, callouts, keyboard, focus, result, or hold
only re-renders the existing capture. Changing cursor visibility,
click capture, or typing cadence needs a new browser recording.

## Advanced: storyboard `direction:` blocks

```yaml
direction:            # tutorial level
  spotlight: off
  keyboard: shortcuts
scenes:
  - id: login
    direction:        # scene level
      zoom: strong
      camera_lock: on
```

Storyboards without `direction:` behave exactly as before.
