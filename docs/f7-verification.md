# R5 / F7 verification record

Development host: Windows amd64, Go 1.27.1. Branch:
`refactor/observation-evidence`. The initial working tree was clean at
`b883d841af906283718fa30906c5a1030f606a71`. AGENTS.md, the product contract and
resolution policy were read before implementation. Issue #20 was read through
the GitHub connector; local `gh issue view` returned HTTP 401.

The implementation and exact semantics are documented in
[Resolution policy](resolution-policy.md#retained-candidate-observations-r5--f7).
No dependencies, enumeration-policy identifier changes, shell-bridge changes,
PATH diagnostic changes, or execution probes were added.

## Commands and results

Go verification used workspace caches after the default cache encountered an
access-denied error:

```powershell
$env:GOCACHE = 'E:\whichwhy\bin\go-cache'
$env:GOMODCACHE = 'E:\whichwhy\bin\mod-cache'
```

| Command | Result |
| --- | --- |
| `gofmt -l .` | PASS, no output |
| `go test ./...` | PASS on Windows, all packages |
| `go test ./... -count=1` | PASS on Windows, all packages, uncached |
| `go vet ./...` | PASS |
| `go build ./cmd/whichwhy` | PASS |
| `go test ./internal/resolver ./internal/resolution ./internal/cli -run 'Observation\|Filesystem' -count=1 -v` | Focused cases passed in the final combined run below |
| `go test ./internal/resolver ./internal/resolution ./internal/cli ./internal/processpath -run 'Observation\|Filesystem\|PATH\|PathExt\|PATHEXT\|Dotted\|UnixMode\|UnixIdentity\|UnixCollection\|UnixFile\|WindowsPolicyVersusCmdOracle\|StandaloneInspection' -count=1 -v` | PASS; includes F7, R2/R3, portable R4, controlled cmd and both PowerShell dotted-name oracles. The processpath package had no regex matches; its complete suite passed in `go test ./...` |
| `powershell.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File scripts/test-powershell-oracle.ps1 -ExecutablePath .\whichwhy.exe` | PASS: PowerShell 5.1.26100.9444, all three discovery phases and 16 fresh-session passive cases |
| `pwsh.exe -NoLogo -NoProfile -NonInteractive -ExecutionPolicy Bypass -File scripts/test-powershell-oracle.ps1 -ExecutablePath .\whichwhy.exe` | PASS: PowerShell 7.6.5, all three discovery phases and 16 fresh-session passive cases |
| `GOOS=linux GOARCH=amd64 go test -c -o ./bin/linux/ ./...` | PASS, cross-compilation only |
| `GOOS=darwin GOARCH=arm64 go test -c -o ./bin/darwin/ ./...` | PASS, cross-compilation only |
| `git diff --check` | PASS |
| `wsl --list --quiet` | WSL is not installed; no local native Unix runtime |

GOOS/GOARCH above denote per-process PowerShell environment assignments, not a
claim that POSIX shell syntax was run on Windows. Ubuntu/macOS native runtime
verification is **PENDING REMOTE CI**. Existing CI matrix jobs still run native
tests, vet and CLI builds. Cross-compilation never substitutes for that gate.
Independent Mission Control review and merge remain outside this local handoff.

## Changed files

| Area | Files |
| --- | --- |
| Documentation | `docs/architecture.md`, `docs/resolution-policy.md`, `docs/f7-verification.md` |
| Collection | `internal/resolver/resolver.go`, `internal/resolver/observation.go`, `internal/resolver/notfound_windows.go`, `internal/resolver/notfound_other.go`, `internal/resolver/candidates_windows.go`, `internal/resolver/candidates_unix.go`, `internal/resolver/candidates_unsupported.go` |
| Collection tests | `internal/resolver/resolver_test.go`, `internal/resolver/observation_test.go`, `internal/resolver/observation_windows_test.go`, `internal/resolver/candidates_unix_test.go` |
| PATH evidence serialization | `internal/processpath/path.go` |
| Completed resolution | `internal/resolution/report.go`, `internal/resolution/observation_test.go` |
| Presentation and doctor | `internal/cli/cli.go`, `internal/cli/json.go`, `internal/cli/doctor.go` |
| Presentation tests | `internal/cli/report_test.go`, `internal/cli/observation_test.go`, `internal/cli/observation_regression_test.go` |

## Pre-F7 failure proof

The parent commit was exported with `git archive --format=zip
--output=bin/pre-f7.zip HEAD`, expanded to `bin/pre-f7`, and supplied only the new
`internal/cli/observation_regression_test.go` test. This test uses existing APIs,
real filesystem inspection, and a disposable marker-writing command that
inspection must never execute.

`go -C bin/pre-f7 test ./internal/cli -run
TestFilesystemObservationSilentLossRegression -count=1 -v` failed all four
subtests against the parent. Both complete and incomplete misses produced the
same no-candidate explanation; failure before a candidate still produced the
old first-observed selection reason. No case retained structured observations.
The same four cases pass with F7.

The final real fixture uses an invalid `<` path component on Windows and an
overlong component on Unix. Earlier fixture trials were rejected: environment
variables cannot contain NUL, and Windows returned path-not-found for an overlong
component. Classification was not changed to force a fixture assumption to pass.
Deterministic permission and unexpected-error cases use injected Stat at the
collector boundary, without manipulating ACLs or reproducing presentation logic.

Initial runs also exposed Windows ENOTDIR aliasing and a test assertion expecting
an unescaped path inside a quoted error. Both were corrected before passing
verification. Cache access failures were resolved by running from the workspace
root with workspace caches; baseline execution used `go -C` to avoid a nested
working-directory sandbox write restriction.

## Hostile self-review

- Every completed collector attempt appends one observation. No Stat failure or
  eligibility skip can reach the old silent `continue`; remaining `continue`
  statements implement existing name parsing/dedup or candidate dedup.
- Ordinary negatives and failures use distinct statuses. Windows network-path
  errors cannot become ordinary negatives merely through Go's broad IsNotExist
  mapping. ENOTDIR cannot be falsely inferred from Windows path-not-found.
- An unresolved attempt before the first eligible observation forces
  `precedence-uncertain`. Errors after that observation retain completeness loss
  without destroying its precedence. Absolute conversion failure preserves the
  usable cleaned-path candidate and reports incomplete collection.
- Identity/group and ownership failures return an operational error with no
  completed result. Unix owner/group/other and UID-0 rules are unchanged.
- Observations precede candidate dedup and retain repeated attempts. PATH indices
  come directly from R2 entries. Windows names and PATHEXT order are unchanged.
- Human and JSON consume the same completed completeness, selection and reasons;
  tests compare observations and PATH evidence too. Human output omits ordinary
  traces but shows failure operands and indices. Doctor preserves observations
  through identity dedup and warns on incomplete inspection.
- Exit 0 remains selection among observed candidates, explicitly qualified when
  precedence is uncertain; exit 1 remains no observed candidate, explicitly
  incomplete when appropriate. Fatal errors remain exit 2.
- Product collection gained no execution, shell invocation, environment mutation,
  retries, module loading or filesystem modifications. Controlled oracle fixtures
  alone execute their explicitly constructed harmless commands.
- No F6/F8/F9/F10 work, branding, package intelligence, ACL/noexec analysis or
  generic filesystem tracing entered the diff. No unresolved F7 design defect
  was found in this review. Native Unix behavior remains a stated CI uncertainty.

Additional limitations remain deliberate: observations are not atomic; broken
links are classified by target Stat result, without new link-identity discovery;
filesystem error codes can conceal distinctions the OS did not report. Existing
doctor executable-identity comparison fallbacks and standalone PATH diagnostic
comparison fallbacks were inspected and left within their existing scope.
