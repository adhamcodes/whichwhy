# Oracle and release verification contract

WhichWhy claims only the evidence classes below. This gate supplements the
[product contract](product-contract.md) and [resolution policy](resolution-policy.md);
it does not expand either. A comparison between two WhichWhy paths is a consistency
test, never an independent shell oracle.

## Claim to evidence to gate

| Claim | Evidence class | Independent oracle / platform fact | Automatic CI gate | A failure means |
| --- | --- | --- | --- | --- |
| `powershell-loaded-session` / `powershell-loaded-session-order-v1` / `shell-observed` | Loaded-session discovery, not successful invocation | Real `Microsoft.PowerShell.Core\Get-Command -All -ListImported` in the same controlled session, with global autoload disabled only during oracle discovery | Windows PowerShell 5.1 and 7: `scripts/test-powershell-oracle.ps1` | Ordered discovery, metadata, selection, missing-result or session/passivity claim regressed |
| PowerShell passive inspection (F1) | Non-mutation and absence of execution | Disposable module initialization/body markers; preference identity/value/options; loaded modules and global variable names | Both oracle steps compose `test-powershell-passive.ps1`, each of its 16 preference cases in a fresh process | Discovery imported a module, ran a body, leaked state, failed restoration or failed to refuse an unsafe guard |
| Exact PowerShell command identity (R6/F6) | Actual generated bridge and native argument boundary | 15 literal identities, real aliases or controlled missing qualified names; ordinal human/JSON identity and passive Get-Command comparison | Both oracle steps compose `test-powershell-transport.ps1` in fresh processes | Quoting, Unicode, encoding, double decoding, candidate metadata or malformed-request rejection regressed |
| Explicit literal inspection (R8/F9) | Exact loaded-session discovery and unchanged standalone policy | Ordinary identities compare the hybrid collector with independent guarded narrow Get-Command; metacharacter identities use fixed provider/file fixtures with unrelated pattern competitors | Both oracle steps compose `test-powershell-literal.ps1`; F1 and F6 also exercise `inspect`; Go parser/routing and standalone fixture tests | Reserved words enter product routing, a pattern expands, ordering differs, transport changes, or public arguments enter private evidence routing |
| Windows `process-external` / `process-path-order-v1` / `policy-only` | Process policy, **no general shell oracle** | Controlled filesystem and literal expected order; selected cmd comparisons classified below | Windows Go tests: `TestWindowsPolicyVersusCmdOracle`, `TestWindowsDottedPATHEXTOracles` | Documented policy changed, controlled shell behavior changed, or output falsely upgraded to shell truth |
| Unix same process identifiers | Effective-user mode-class policy, **no shell oracle** | Real PATH/filesystem/Stat, effective UID/GID/groups, chmod and symlink target facts; direct kernel execution of a disposable owner fixture | Ubuntu and macOS Go tests: `TestUnixOwnerEligibilityOracle`, `TestUnixCandidateTargetModes`, `TestUnixCollectedEffectiveIdentity`, `TestUnixEligibilityPATHCorrelation` | Native collection, class eligibility, target semantics or passivity regressed (or the runner no longer supports the required fixture) |
| Original PATH identity (R2) | Process parsing and correlation, no shell truth | Native `filepath.SplitList`, fixed raw segments, disposable directories, independently specified original indices | All Go jobs: `internal/processpath` tests, `TestProcessPATHEmptyRelativeDuplicateCorrelation`; Windows quoted/dotted correlation tests | Empty/quoted/duplicate entries drifted or skipped entries renumbered candidates |
| Dotted/PATHEXT names (R3) | Windows enumeration policy | Fixed name sequences plus native executable and batch fixtures; cmd invocation and fresh 5.1/7 discovery | Windows dotted oracle and resolver name tests | Unrecognized suffix loses appended alternatives, extension order changes, or recognized-suffix boundary changes |
| Owner/group/other eligibility (R4) | Explicit mode policy, not access/exec guarantee | Native owner and target tests above; all 512 modes for fixed owner/group/supplementary/other/root identities | All jobs: `TestUnixModeEligibility`, `TestUnixModeRejectsLegacyAnyExecuteFalsePositive`; native Unix jobs above | Owner/group denial falls through, identity fails open or metadata is lost |
| Completeness and precedence (R5) | Retained observations, no shell oracle | Real Stat missing/malformed operands and native Unix broken/looping targets/ENOTDIR; deterministic injected permission/IO/absolute-path errors | All jobs: `TestFilesystemObservationSilentLossRegression`, resolver observation tests, `TestObservationCompletenessAndPrecedence`, presentation/doctor tests | A failure becomes absence, a miss hides incompleteness, or uncertainty changes established precedence |
| Shared human/JSON claims | Presentation consistency, **not an oracle** | A supplied completed report, including a deliberately non-first selection | All Go jobs: report and observation presentation tests | A renderer re-resolves selection or drops claim/evidence/limitations |

## PowerShell experiment order

Load harness dependencies and construct controlled fixtures first. Snapshot loaded
modules, global variable names, function definitions, alias definitions/options,
PATH, PSModulePath and location. Run WhichWhy JSON and human inspection and check
the snapshot and execution markers after each. The main suite separately checks
autoload preference identity/value; F1 exercises absence, shadowing, types and
immutable options. Automatic shell bookkeeping (such as LASTEXITCODE and error
history) is not a claim that all session variable values are immutable.

Only then collect the oracle. Its independent global autoload guard is restored
in `finally`; only CommandNotFoundException is allowed as an ordinary discovery
miss. Check the snapshot again. Compare every candidate and the selected match
against the real shell using ordinal kind/name/source/path/alias-target equality,
including absent optional metadata. Fixture-specific counts and kind order prevent
an empty or malformed oracle from masquerading as agreement.

The main suite establishes alias > function > application, function > application,
applications alone (two distinct PATH locations, including a space), missing,
unloaded bare/qualified module names, and a deliberately loaded module command.
All bodies write disposable markers. F1 also tests aliases
to unloaded commands, qualified core discovery and constrained preference failures.
Transport covers difficult names without duplicating the entire F1 suite.

Both ordinary and explicit inspection run through the main oracle phases. Two
PATH directories each contain `.cmd` and `.bat` alternatives, catching broad
discovery's different filename grouping and any manual reordering. Explicit
literal tests obtain expected alias/function metadata through literal provider
paths, and specify exact external paths and order from controlled fixtures.
They do not call the product's broad-filter helper as their oracle. Brackets,
backticks, invalid wildcard expressions, case, Unicode, reserved/internal-looking
names, qualified module exports, and missing identities are covered. All fixture
bodies remain unexecuted. The existing F1 matrix also exercises explicit lookup
and constrained guard refusal; the F6 identities run through both public grammars.

The explicit wildcard-bearing external contract matches complete filenames;
it does not infer suffixes from wildcard queries. Exact-name matching is
case-insensitive under PowerShell semantics, while command transport and report
headings preserve the requested string ordinally. Broad discovery and the engine
API alone are not substitutes for the ordered narrow oracle; see the
[resolution policy](resolution-policy.md#explicit-powershell-inspection).

## Controlled Windows comparisons

Every cmd case logs one classification. **EXPECTED AGREEMENT** covers normal
PATH-only order, duplicate PATH (selection agrees; full dedup/index behavior is a
filesystem policy assertion), leading empty entry, PATHEXT order/reversal/duplicates,
extensionless names with appended files, unrecognized dotted and multi-dot names,
literal dotted first/alone, trailing dot, and explicit recognized/mixed-case suffix
when its literal exists. Literal-extensionless-only is agreement on a miss.

**EXPECTED DELIBERATE DIFFERENCE** covers implicit current-directory search,
entirely empty PATH, and a missing recognized suffix with appended files available.
Cmd can execute these fixtures while the documented narrower policy selects a PATH
alternative or none. Both presentations must retain `policy-only` and the unobserved
shell limitation. These differences are passing evidence, not requests to clone cmd.
PowerShell additionally lists recognized-suffix appended alternatives and literal
extensionless files that standalone policy excludes; the dotted suite records that
broader discovery separately from cmd selection agreement.

## Unix boundary and uncertainty

Native Ubuntu/macOS tests use the invoking test identity, owned disposable files,
exact modes, multiple PATH entries, empty/relative/duplicate entries and symlinks.
The owner oracle first inspects the 0700 control with its marker absent, intentionally
executes it, checks output and marker, then repeats inspection-before-execution for
owner/group/other bit patterns. Owner denial must produce EACCES without a marker.
Group/other identities that cannot safely be created on CI are fixed-input policy
tests; no sudo, chown or ACL manipulation is required. ACLs, noexec, capabilities,
filesystem IDs, extended groups, interpreter availability and races remain limits.
There is no Bash, Zsh or Fish resolution parity claim.

R5 gates distinguish (1) complete ordinary misses, (2) unresolved attempts before a
later observed selection: incomplete and precedence-uncertain, (3) errors after the
first candidate: incomplete but definitive precedence, and (4) incomplete versus
complete no-candidate reports. Deterministic injection covers permission failures;
real collector regressions also run on every native platform. No fake ACL fixture
is needed and report-only tests are not described as platform evidence.

## Fail-closed release gate

CI runs formatting, uncached `go test ./... -count=1`, vet and CLI build on the
existing Ubuntu/Windows/macOS matrix. Native Go tests already contain the Unix and
cmd gates, so no duplicate wrapper jobs are needed. Windows adds both version-checked
PowerShell suites, composing F1 and F6 automatically. `WHICHWHY_REQUIRE_ORACLES=1`
makes unavailable PowerShell binaries and unsupported Unix execution/ownership
controls fail rather than skip. Constrained local runs may explicitly skip those
capabilities; such a run is not release proof.

Launch errors, unexpected exit codes, setup/write errors and parse failures fail.
Fresh dotted PowerShell discovery must emit a versioned start and completion record.
Cmd expected misses require a launched process with exit 1; launch failure cannot
pass as absence. Intentional execution requires identifying stdout and marker
contents. Inspection must leave the marker absent first. Fixtures are unique-temp
or fresh-session scoped, harmless and cleaned by `finally`/Go test cleanup. No
arbitrary installed command is executed as a test subject.
PowerShell marker checks distinguish ordinary absence from access/IO failures;
`File.Exists` alone cannot establish this because it hides observation errors.

The literal expectations and comparisons above identify the defects each gate
catches. A useful negative check reverses supplied shell candidates before comparison:
kind/order checks must fail, even if the selected candidate is unchanged. Never
commit a mutation. Build/cross-compilation on Windows is not Unix runtime evidence;
Ubuntu/macOS results must come from native CI before release. Required-check/branch
protection configuration lives outside this repository and is not established by
this workflow alone.
