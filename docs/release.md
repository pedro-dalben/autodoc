# Release

## Versioning

Tags use SemVer: `vX.Y.Z` for stable, `vX.Y.Z-rc.N` for release
candidates. The binary reports the tag it was built from
(`autodoc version`); untagged builds report `0.1.0` with commit `dev`.

No stable release exists yet. The current pre-release is `v0.1.0-rc.2`,
published for real-world validation before `v0.1.0`. A pre-release is
never presented as `latest stable`: install it explicitly pinned
(`AUTODOC_VERSION=v0.1.0-rc.2`), see [install.md](install.md).

## Cutting a release candidate

```bash
git tag -a v0.1.0-rc.2 -m "AutoDoc v0.1.0-rc.2"
git push origin v0.1.0-rc.2
```

Never rewrite a published tag. If the RC is broken after publication,
the fix ships as the next candidate (`v0.1.0-rc.2`), never by moving
the old tag.

## Cutting a release

```bash
git tag -a v0.2.0 -m "AutoDoc v0.2.0"
git push origin v0.2.0
```

Pushing the tag runs the `release` workflow: formatting, vet, unit and
integration tests, plus the installer and docs smoke tests. Only then
GoReleaser publishes a non-draft GitHub Release with:

- archives per platform (`autodoc_<version>_<os>_<arch>.tar.gz`, `.zip` on
  Windows): Linux amd64/arm64, macOS amd64/arm64, Windows amd64;
- `checksums.txt` (SHA-256) covering every archive.

Tags matching `vX.Y.Z-rc.N` publish as pre-release (never `latest`
stable); plain `vX.Y.Z` tags publish as stable. The installer
(`install.sh`, `install.ps1`) consumes exactly these two asset types.
Never publish a draft release and expect installs to work.

## Checking the release config locally

```bash
goreleaser check
goreleaser release --snapshot --clean
```

`--snapshot` builds every archive without publishing. Compare the file
names against what `install.sh` requests; they must match.

## Homebrew

No tap exists yet. Until one does, install via the script above or from
source. Do not document a formula that has not been published.
