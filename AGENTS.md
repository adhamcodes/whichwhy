# Engineering Contract

This repository is a correctness-first command-resolution debugger. Treat every claim about what a shell will execute as something that must be supported by evidence.

Before implementation work, read `docs/product-contract.md` and treat it as the product-level source of truth. The repository name `whichwhy` is currently a working codename; do not perform naming or branding changes unless explicitly assigned.

## Non-negotiable rules

- Correctness over cleverness. If the evidence is incomplete, preserve uncertainty instead of guessing.
- Inspection must be passive. Never execute an arbitrary inspected command merely to learn about it, and do not mutate the caller's shell/session while collecting evidence.
- The shell is the oracle for shell-specific behavior. Claimed support must be backed by tests that compare WhichWhy behavior with the real shell.
- Preserve observation failures, skipped candidates, and limitations instead of silently discarding them when they affect confidence.
- Keep evidence collection, resolution, diagnostics, and presentation separate. Human and JSON output should consume the same resolved result rather than re-implementing precedence independently.
- Do not broaden scope while fixing a focused issue. Prefer small, reviewable changes with targeted regression tests.
- Do not add dependencies unless they are clearly justified.
- Do not rewrite working architecture without evidence that a rewrite is necessary.
- Do not merge pull requests. Leave merge decisions to Mission Control.

## Current safety boundary

The generic command inspector must not run candidate executables. Known-tool execution probes are out of scope unless explicitly approved and allow-listed in a dedicated change.

PowerShell discovery must not auto-import an unloaded module merely because a command is inspected. If safe inspection cannot establish a result, report the limitation rather than causing module initialization side effects.

## Verification expectations

For behavior changes, add regression tests that would have failed before the fix. Where shell semantics are involved, include fresh-session oracle coverage when practical.

Before handing work back:

- run formatting checks;
- run `go test ./...`;
- run `go vet ./...`;
- build the CLI;
- run relevant PowerShell oracle coverage on supported versions when the environment permits;
- inspect the diff for accidental scope expansion;
- report remaining uncertainty or unverified platform assumptions explicitly.

## Git discipline

Use focused commits that describe engineering intent (`fix:`, `test:`, `refactor:`, `docs:`, `ci:`, `chore:`). Do not merge, force-push shared history, or make unrelated cleanup changes while solving a scoped issue.
