# Release

## Versioning

Tags use SemVer: `vX.Y.Z`. The binary reports the tag it was built from
(`autodoc version`); untagged builds report `0.1.0` with commit `dev`.

## Cutting a release

```bash
git tag v0.2.0
git push origin v0.2.0
```

Pushing the tag runs the `release` workflow: formatting, vet, unit and
integration tests, plus the installer and docs smoke tests. Only then
GoReleaser publishes a non-draft GitHub Release with:

- archives per platform (`autodoc_<version>_<os>_<arch>.tar.gz`, `.zip` on
  Windows): Linux amd64/arm64, macOS amd64/arm64, Windows amd64;
- `checksums.txt` (SHA-256) covering every archive.

The installer (`install.sh`, `install.ps1`) consumes exactly these two
asset types. Never publish a draft release and expect installs to work.

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
