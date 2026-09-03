# Security policy

Report vulnerabilities privately to the maintainers (see git remotes for
contact). Do not open public issues with exploit details.

Scope notes:

- AutoDoc never records login flows (except fixture-based login tutorials with
  masked password inputs) and never uses your everyday browser profile.
- `compile`/`record` fail on secret sentinels; `doctor` redacts secrets.
- Never publish `.autodoc/_work/`, profiles, or `storage-state.json`.
- Supported versions: latest `main` / latest release. EOL releases get no fixes.
