# Install

## Binary (recommended)

No Go toolchain needed.

> No stable release exists yet. The current release candidate is
> `v0.1.0-rc.3` (pre-release, for real-world validation before `v0.1.0`).
> Install it explicitly pinned — a bare install resolves `latest stable`
> and will refuse while only pre-releases exist:

```bash
curl -fsSL https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.sh | sh -s -- --version v0.1.0-rc.3
```

```powershell
.\install.ps1 -Version v0.1.0-rc.3
```

Linux or macOS (stable releases only, once they exist):

```bash
curl -fsSL https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/pedro-dalben/autodoc/main/install.ps1 | iex
```

The installer detects OS and architecture, downloads the matching archive
from GitHub Releases, verifies its SHA-256 checksum, and installs the binary
to `~/.local/bin` (or `%LOCALAPPDATA%\autodoc\bin` on Windows). A checksum
mismatch aborts the install. If the directory is not on `PATH`, the installer
prints the exact line to add.

Reproducible install of a specific version:

```bash
curl -fsSL .../install.sh | sh -s -- --version v0.1.0-rc.3
```

```powershell
.\install.ps1 -Version v0.1.0-rc.3
```

Supported platforms (these are the archives GoReleaser builds):

- Linux amd64 and arm64
- macOS amd64 and arm64
- Windows amd64

Releases are cut from version tags (`vX.Y.Z`, pre-releases as `vX.Y.Z-rc.N`)
by GitHub Actions; each release publishes archives plus `checksums.txt`.
See [release.md](release.md).

## Requirements

- FFmpeg 6+ with `libx264` and `aac` (`ffmpeg` and `ffprobe` on `PATH`).
  Install it before running `autodoc render`:

  ```bash
  sudo apt install ffmpeg          # Debian/Ubuntu
  brew install ffmpeg              # macOS
  winget install Gyan.FFmpeg       # Windows
  ```

  To use custom paths without touching `PATH`:

  ```bash
  export AUTODOC_FFMPEG=/opt/ffmpeg/bin/ffmpeg
  export AUTODOC_FFPROBE=/opt/ffmpeg/bin/ffprobe
  ```

- Playwright browser assets. The Go binary does not include them:

  ```bash
  autodoc browser install
  ```

- A TTS endpoint (any OpenAI-compatible `/audio/speech` server), or
  `provider = "disabled"` for silent runs. See [tts.md](tts.md) and
  [local-tts.md](local-tts.md).

## From source (developers)

```bash
go build -o autodoc ./cmd/autodoc
```

## Setup

```bash
autodoc init              # project: ./autodoc.toml + example storyboard.yml
autodoc init --global     # machine: ~/.config/autodoc/autodoc.toml, no storyboard
autodoc doctor            # full diagnostics
```

`init` probes installed coding agents, installs the skill and MCP entries it
owns, checks TTS/FFmpeg/browser, and writes config plus an example
`storyboard.yml` (project mode only). Re-running it changes nothing when
everything is already in place.

Every command resolves config as project, then global, then built-in
defaults. See [config.md](config.md).

## Verify

```bash
autodoc version
autodoc doctor
autodoc browser check
```

`doctor` answers "is my installation ready". Each failing check prints the
command or doc page that fixes it.

## Uninstall

```bash
autodoc uninstall
```

This removes only AutoDoc-owned entries (manifest-tracked, marker-delimited).
Your own edits survive. Delete the binary separately (`rm ~/.local/bin/autodoc`).
