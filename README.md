# WhichWhy

**Which command gets picked — and why?**

In PowerShell, `where` can be an alias even when `where.exe` is on PATH.
WhichWhy shows the command the loaded session selects and the alternatives it
shadows. This excerpt comes from the built CLI in PowerShell 7.6.5 on Windows:

```text
WhichWhy - where
[ loaded-session discovery ]

  SELECTED
    Alias where -> Where-Object

  Why
  - PowerShell itself reported this match first for the current loaded session.

  Shadowed (loaded session)
  - Application where.exe
     C:\WINDOWS\system32\where.exe
     Source: C:\WINDOWS\system32\where.exe
```

The full report also includes the PowerShell version, scope, limitations, and a
passive-inspection note. Discovery does not prove an alias target will execute;
unloaded-module auto-loading is not modeled.

## What it solves

Multiple installations, PATH order, aliases, and functions can make a familiar
command reach an unexpected target. WhichWhy explains the selected candidate,
the evidence behind it, the alternatives, and what remains uncertain. It also
diagnoses PATH entries and checks whether the running WhichWhy is the one its
process PATH policy would select.

WhichWhy is under active development. The core runs locally without an account,
remote service, or AI model. Inspection does not execute the commands it finds.

## Build and use

The current installation path is a source build with **Go 1.27**. From this
repository:

```text
go build ./cmd/whichwhy
```

On Windows, run `.\whichwhy.exe`. On Linux or macOS,
run `./whichwhy`. You can place the binary in a directory on your PATH yourself;
WhichWhy does not install itself or edit your shell configuration.

## A 60-second quick start

In PowerShell, from the build directory:

```powershell
# Inspect the process-visible external candidates.
.\whichwhy.exe python

# Diagnose PATH and check the running WhichWhy installation.
.\whichwhy.exe path
.\whichwhy.exe doctor

# Optional: load the shell bridge into this session.
.\whichwhy.exe init powershell | Invoke-Expression

# Now inspect loaded-session aliases, functions, cmdlets, and applications.
whichwhy where
whichwhy inspect path
whichwhy where --json
```

Loading the optional bridge defines a `whichwhy` function for this session; it
does not write your profile. Subsequent inspection is passive. `init powershell`
by itself only prints the integration script. Regenerate the bridge after an
upgrade. If doctor reports attention immediately after a local build, the build
directory may simply not be on PATH; read the report before changing anything.

On Linux or macOS, start with `./whichwhy python`, `./whichwhy path`, and
`./whichwhy doctor`. These use the standalone process policy.

## Main commands

| Command | Purpose |
| --- | --- |
| `whichwhy <command>` | Inspect a command identity |
| `whichwhy inspect <literal-command>` | Inspect reserved or wildcard-bearing names literally |
| `whichwhy path` | Summary, problems, and every original PATH entry |
| `whichwhy doctor` | Check this installation's process-policy discovery |
| `whichwhy init powershell` | Print the optional PowerShell session bridge |
| `whichwhy --help` | Usage, options, and examples |
| `whichwhy --version` | Build version |

Add trailing `--json` to either inspection form, `path`, or `doctor`.
`whichwhy inspect path` inspects a command named `path`.
`whichwhy inspect --json` inspects a command named `--json`; add another
`--json` to request its JSON report. Quote names for your shell, for example
`whichwhy inspect 'name[1].cmd'`. Explicit executable paths are not supported.

## Standalone versus shell-aware

| Mode | What the answer means |
| --- | --- |
| Standalone | `process-path-order-v1` selects among process-visible external candidates. The claim is `policy-only`; your shell may select something else. |
| PowerShell bridge (experimental) | PowerShell supplies ordered discovery from the active loaded session. The claim is `shell-observed`, scoped to that session. |

Human reports distinguish:

- **SELECTED** — the completed report selected a candidate within its stated scope.
- **OBSERVED CANDIDATE** — a candidate was selected, but an earlier unresolved attempt makes its precedence uncertain.
- **NOT FOUND** — no candidate was observed within the stated scope; this is not proof of universal unavailability.
- **NO DEFINITIVE RESULT** — no candidate was observed and filesystem inspection was incomplete.

Selection and inspection completeness are separate. A later filesystem failure
can leave selection definitive while inspection remains incomplete. Reports keep
both facts visible. PowerShell does not borrow the process completeness field or
claim successful invocation.

## JSON and automation

```powershell
$report = whichwhy where --json | ConvertFrom-Json
$report.selected
$report.resolution_scope
$report.limitations
```

JSON contains one UTF-8 object with no human layout or ANSI styling. Command and
doctor reports use schema version 2; PATH uses version 1. Full process observations,
including ordinary misses and skipped candidates, remain in JSON.

| Surface | Exit 0 | Exit 1 |
| --- | --- | --- |
| Command inspection | An observed candidate is selected, including uncertain precedence | No candidate observed, including incomplete inspection |
| PATH | PATH is set, including empty PATH or entry problems | PATH is unset |
| Doctor | Healthy process-policy discovery | Attention needed |

Exit 2 means a usage or operational failure. JSON exits 0/1 have empty stderr;
exit-2 diagnostics go to stderr. Check the report's scope, selection status, and
completeness instead of treating exit 0 as proof of execution. See the frozen
[JSON contract](docs/json-contract.md) for fields and compatibility rules.

## Platforms and terminals

The native verification matrix covers Windows, Linux, and macOS. PowerShell
loaded-session discovery has dedicated Windows PowerShell **5.1** and **7** oracle
suites. There is no Bash, Zsh, Fish, or general cmd.exe shell-resolution parity
claim; standalone behavior is a named process policy on every platform.

Suitable direct terminals get restrained amber accents, semantic status colors,
and Unicode tree lines. A compact selected answer can use a small border. Set
`NO_COLOR` to a nonempty value to disable color. Pipes and files receive plain
text. Unknown or unsuitable consoles use ASCII decoration. Prose adapts to the
observed width; long identities remain intact and soft-wrap in the terminal.

The PowerShell bridge deliberately receives and forwards a redirected UTF-8
response, so its reports use the plain layout even in an interactive session.
This preserves Unicode transport and clean PowerShell pipelines without changing
the caller's console settings. [Terminal presentation](docs/terminal-presentation.md)
describes capability detection and the safety policy.

## Guarantees and limits

- Inspected commands are never executed, and inspection never auto-fixes PATH or configuration.
- PowerShell discovery does not auto-import unloaded modules. Unsafe session-state conditions are refused.
- Observation failures, scope, and uncertainty remain visible; presentation never selects a different candidate.
- Human output escapes terminal controls and Unicode formatting controls. JSON retains the underlying evidence.
- PATH entries retain original indices, raw/effective spelling, empty segments, duplicates, and filesystem problems.
- Filesystem observations are not atomic. Eligibility does not prove invocation permission, interpreter availability, or execution success.
- Standalone inspection cannot observe aliases, functions, built-ins, cmdlets, or shell command caches. Unix mode checks do not model ACLs, noexec mounts, or all platform access rules.

## Deeper documentation

- [Product contract](docs/product-contract.md)
- [Resolution policy and evidence boundaries](docs/resolution-policy.md)
- [JSON and machine-error contract](docs/json-contract.md)
- [Oracle and release verification contract](docs/oracle-contract.md)
- [Architecture](docs/architecture.md)
- [Terminal presentation and safety](docs/terminal-presentation.md)

## Development and contributing

```text
gofmt -l .
go test ./... -count=1
go vet ./...
go build ./cmd/whichwhy
git diff --check
```

On Windows, run both complete shell gates against the build:

```powershell
powershell -NoProfile -File scripts/test-powershell-oracle.ps1 -ExecutablePath .\whichwhy.exe -ExpectedMajor 5
pwsh -NoProfile -File scripts/test-powershell-oracle.ps1 -ExecutablePath .\whichwhy.exe -ExpectedMajor 7
```

Each gate also runs passive, transport, literal, isolation, and response coverage.
CI requires native capabilities with `WHICHWHY_REQUIRE_ORACLES=1`; local skips do
not establish release readiness. Linux/macOS runtime verification comes from their
native CI jobs, not Windows cross-compilation.

Read [CONTRIBUTING.md](CONTRIBUTING.md) and [AGENTS.md](AGENTS.md) before changing
resolution behavior. License: [MIT](LICENSE).
