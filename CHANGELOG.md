# Changelog

## V1 — unreleased

Planned sequence: `v1.0.0-rc.1`, RC audit, then `v1.0.0`. No release date or
publication is implied by this entry.

- Command-resolution evidence and explanations for selected and alternative candidates.
- PATH diagnostics preserving original entries, duplicates, misses and filesystem failures.
- `doctor` checks for the running WhichWhy installation under the process policy.
- Experimental Windows PowerShell 5.1/7 loaded-session bridge for aliases, functions,
  cmdlets and applications, without importing unloaded modules during discovery.
- Explicit scope, uncertainty, incomplete observations and passive/local inspection.
- Frozen JSON interfaces: command/doctor schema 2, PATH schema 1; documented exits and streams.
- Windows, Linux and macOS native process-policy verification; no general shell parity claim.
- Draft-only GitHub Release pipeline with versioned Windows amd64, Linux amd64,
  macOS amd64 and arm64 archives, MIT license, usage reference and SHA-256 manifest.
  Both macOS architectures have native packaged-binary `--version` smoke gates.
  Release commits must be ancestors of explicitly fetched remote main history.

See [V1 release-note material](docs/release-notes-v1.md) and the
[release checklist](docs/release.md). Packaging does not change runtime behavior,
resolution semantics or JSON contracts.
