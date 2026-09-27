# Resolution claims

## Command identity and public syntax

Ordinary inspection remains `whichwhy git` or `whichwhy git --json`. Product
commands remain `path`, `doctor`, `init powershell`, help and version.
Use explicit inspection when the identity conflicts with product syntax or
contains PowerShell wildcard characters:

```text
whichwhy inspect <literal-command>
whichwhy inspect <literal-command> --json
```

The first argument after `inspect` is always the command identity. The only
permitted subsequent argument is `--json`. Thus `whichwhy inspect --json`
inspects a command named `--json`, and `whichwhy inspect --json --json` emits its
report as JSON. `whichwhy inspect path` inspects `path` instead of diagnosing
PATH; `whichwhy inspect inspect` inspects `inspect`. Missing/empty identities or
extra arguments are usage errors (exit 2). There is no `-- <name>` grammar.
Shell quoting remains the caller's responsibility, including a literal identity
that the shell itself treats specially.

In PowerShell, examples include `whichwhy inspect 'ww*f9'` for a literal
session command and `whichwhy inspect 'ww[f9].cmd' --json` for a literal external
filename. For wildcard-bearing external identities, supply the complete filename:
exact filtering does not invent extension inference from a pattern-sensitive
name. Discovery is not proof of successful invocation.

Explicit standalone inspection uses the unchanged process policy below and stays
`policy-only`. Only the active PowerShell bridge can supply `shell-observed`
loaded-session evidence. Explicit inspection changes routing, not standalone
PATH, suffix, eligibility, or completeness rules.

Standalone inspection reports a **policy-selected candidate**, not the command
that an unobserved shell will execute. The policy name is
`process-path-order-v1`; its scope is `process-external` and its claim strength
is `policy-only`. These identifiers are intentionally tested as literal values.

## Exact standalone policy

This policy names the implemented algorithm, including the focused R2 PATH
evidence, R3 dotted-name, R4 Unix eligibility and R5 observation-retention
corrections; it does not establish shell equivalence.

1. Accept one nonempty command name. Reject names for which Go's platform-native
   `filepath.Base(name) != name`; explicit command paths remain unsupported.
2. Read the process's `PATH` and `PATHEXT` using `os.Getenv`. Parse PATH once into
   `internal/processpath.Path`, matching Go's platform-native `filepath.SplitList`
   semantics while retaining raw segments. An entirely empty or unset PATH
   produces no entries. On Windows the delimiter is a semicolon and SplitList
   removes double quotes while respecting quoted delimiters. On Unix the
   delimiter is a colon. This is process parsing, not shell parsing.
3. Visit directories in that order. An empty entry becomes `.`; relative entries
   are relative to the inspected process's current directory. There is no extra
   implicit current-directory search before PATH.
4. On Windows, split PATHEXT on semicolons, trim whitespace, discard empty values,
   and prepend a dot to extensions that lack one. Preserve spelling and order,
   including duplicate entries at this parsing stage. If no extensions remain,
   use `.COM`, `.EXE`, `.BAT`, `.CMD`. Compare `filepath.Ext(command)` (the final
   dot and everything after it) with each resulting extension, case-insensitively.
   If it matches, check the literal command name alone. Otherwise, if that suffix
   is nonempty, check the literal name first, then append each PATHEXT extension
   to the **whole command name** in extension order. With an empty suffix, check
   only the appended names; do not introduce a literal extensionless candidate.
   Remove case-insensitive duplicate generated names, keeping the first spelling
   and position. Directory order takes precedence over this name order.
5. Use `os.Stat` (following symlinks). On Windows accept any successfully observed
   non-directory candidate. On Unix accept successfully observed non-directory
   candidates eligible under the effective-process-identity mode-class rule below.
   Neither filter proves that the invoking user can successfully execute the
   candidate.
6. Return absolute candidate paths when `filepath.Abs` succeeds, otherwise cleaned
   paths. Remove duplicate cleaned path strings, case-insensitively on Windows,
   keeping the first occurrence and its original PATH index. Do not resolve
   symlink identity for this deduplication.
7. Select the first remaining observed candidate, if any. Preserve the others in
   order as alternatives **under this policy**. An unresolved Stat attempt before
   that candidate makes its precedence uncertain; selection is then only among
   observed candidates. A failure after it leaves its observed precedence intact.
   No selection means no observed candidate under this policy, not proof that a
   shell cannot find the command. See the F7 completeness rules below.

Every candidate-search attempt now retains its outcome, including Stat failures
and skipped operands. The report discloses inspection completeness separately
from claim strength. It also discloses unobserved shell-local state and possible
differences in shell external search rules, including current-directory,
empty-entry, and quoted-PATH behavior.

## Correlated process PATH evidence (R2 / F2)

`processpath.Path` retains the complete raw value and an ordered, unfiltered
sequence of entries. Each entry carries its one-based index, raw segment, and
interpreted value. One scanner determines both raw and interpreted boundaries;
the resolver and PATH diagnostics do not maintain separate parsers. Tests compare
the native parser with `filepath.SplitList`, including malformed quote sequences.

On Windows, each double quote toggles delimiter protection. A semicolon inside
quotes belongs to the same entry. All double quotes are removed from the
interpreted value, including embedded or unmatched quotes; an unmatched quote
protects remaining semicolons to the end of PATH. This is Go-compatible process
policy behavior, not quote validation or a claim about shell behavior. Drive
colons, backslashes, UNC spelling, whitespace, and single quotes are retained.
On Unix, every colon is a delimiter and quotes are literal characters; semicolons
are literal too. Neither platform trims whitespace or expands variables or `~`.

An entirely empty value has zero entries. Leading, repeated, and trailing
delimiters in a nonempty value retain empty entries; Windows `""` also yields an
empty interpreted entry. `Entry.EffectiveValue()` maps only an empty interpreted
value to `.`. Other values retain their spelling for filesystem inspection;
relative and Windows drive-relative paths retain the existing platform-native
`filepath.Join`/`filepath.Abs` behavior in the inspected process. Parsing neither
changes the working directory nor inserts an implicit search directory.

Candidate collection consumes these entries, retaining `entry.Index - 1` as its
internal `DirectoryIndex`. The completed process resolution report retains the
parsed PATH and exposes `DirectoryIndex + 1` as candidate `PathIndex`. Human
command output labels external candidates with `PATH #N`; command JSON keeps its
existing `path_index`. No index is recomputed from successful candidates.
`pathdiag.InspectPath` can consume that same collected PATH without reparsing;
standalone `path` parses its own process value using the same model. Correlation
between separate invocations requires the same PATH, working directory, and
relevant Windows drive context; filesystem observations are not an atomic snapshot.
PowerShell reports have no process PATH attached and retain shell-supplied order.

Diagnostics preserve every parsed entry, including missing directories, existing
non-directories, and Stat errors. Empty entries are now inspected as `.` and can
carry both `empty` and a filesystem state. Duplicate diagnostics compare cleaned
absolute effective directory strings when `filepath.Abs` succeeds, falling back
to cleaned effective strings on failure, case-insensitively on Windows. They
always refer to the first original one-based entry, including empty, quoted, and
relative spellings of that directory. They do not deduplicate the entry list or
resolve symlinks. Directory duplicate labels are lexical directory comparisons;
candidate deduplication remains the existing separate comparison of final file
paths. Neither claims filesystem object identity. Candidate absolute-path failures
are retained by F7; PATH diagnostics' own lexical comparison fallback is unchanged.

This fixes evidence correlation without changing resolver enumeration or selection,
so `process-path-order-v1`, `process-external`, and `policy-only` remain unchanged.
Process parsing does not establish cmd.exe, PowerShell, Bash, Zsh, or Fish truth.

## PowerShell evidence

The passive bridge continues to supply ordered matches from the active loaded
session. `powershell-loaded-session-order-v1` selects the first supplied match,
with scope `powershell-loaded-session` and claim strength `shell-observed`.
No Go code emulates shell precedence. Shell version and edition stay attached.

This claim covers discovery in the observed loaded session, not successful
invocation. Unloaded-module auto-loading remains unmodeled; even a discovered
alias does not prove its target can execute. The F1 collector guard is unchanged.
No discovery probe executes the inspected command.

Before scratch assignments, the bridge checks every scratch binding for inherited
`AllScope` state. PowerShell shares these variable objects across child scopes;
local assignment or replacement cannot safely isolate them. A collision refuses
the call with a PowerShell error and `LASTEXITCODE=2`, before discovery or native
execution. Caller variable identity, value and options remain unchanged. Ordinary
non-AllScope caller variables are shadowed normally. Regenerate the bridge after
upgrading. The fresh-session isolation suite checks every emitted scratch write,
including loop variables and `-ErrorVariable`, on 5.1 and 7.

Before any discovery query, the bridge shadows `PSDefaultParameterValues` with
a fresh function-local empty table. Caller defaults can otherwise inject
`Get-Command:ArgumentList`, execute inspected `dynamicparam` code, or change
discovery parameters. The bridge neither copies/evaluates caller entries nor
changes the caller table's `Disabled` key. Non-AllScope caller variables,
including ReadOnly and Constant bindings, retain their identity, value/content
and options. Inherited AllScope defaults cannot be safely shadowed: inspection
is refused with a PowerShell error and `LASTEXITCODE=2`, before discovery or
autoload-preference changes. Product-command forwarding remains unchanged.
The local table disappears with the function scope, including on failure.
Regenerate the bridge after upgrading to obtain this AUD-001 correction.

`scripts/test-powershell-defaults.ps1` runs fresh-process cases on both supported
shells for absent/null/empty defaults, injected and wildcard defaults,
scriptblock-valued defaults, unrelated defaults, immutable bindings, AllScope,
and inherited/nested scopes. Both grammars and formats must leave independent
dynamic/body/default-callback markers untouched and preserve caller state.
The oracle helper also isolates defaults and is checked against these markers;
oracle agreement alone is not passivity evidence.

### Explicit PowerShell inspection

The explicit collector runs entirely inside the existing F1 global autoload guard.
Qualified `Get-Command -Name '*' -All -ListImported` supplies broad observations;
in-memory `OrdinalIgnoreCase` comparison selects exact command names, or exact
`Source\Name` module-qualified identities for shell-local commands. The original
requested string is never rewritten; shell name matching and F6 transport
identity are separate. Candidate metadata comes from returned `CommandInfo`
objects, including shadowed module exports.

For identities containing `*`, `?`, `[` or `]`, filtering the broad list retains
the shell's order within the requested full name. It excludes unrelated pattern
matches and handles even strings that are invalid wildcard expressions. This
includes literal external filenames with brackets. For other identities, broad
discovery is restricted to shell-local types. Its exact local matches precede
the unmodified ordered external observations from a separate guarded, qualified
`Get-Command -Name <identity> -CommandType Application,ExternalScript -All
-ListImported` query. No list is sorted and no external suffix rules are
implemented in the bridge. Non-miss discovery errors abort the request; state
restoration still runs in `finally`.

The split is necessary because broad discovery groups applications by filename,
whereas ordinary exact-name discovery uses PATH and extension order. Both native
oracle gates compare the combined result's complete metadata/order/selection
against ordinary guarded `Get-Command`, including competing `.cmd`/`.bat` files
in two PATH directories. Literal fixtures independently specify identity, metadata
and order, including alias/function/application collisions and qualified exports.

`CommandInvocationIntrinsics.GetCommands(name, All, false)` is insufficient:
both tested shells miss literal bracket-containing external filenames, and its
unqualified lookup can omit shadowed function exports retained by `Get-Command
-All`. Wildcard escaping is not a substitute: 5.1 and 7 escape backticks
differently and escaped external queries can miss real filenames. Neither is
used by the explicit collector.

Explicit inspection may enumerate substantially more commands than ordinary
inspection, especially PATH files for wildcard-bearing identities. Ordinary
`whichwhy git` retains its existing narrow collector. These are Windows 5.1/7
discovery guarantees, not a new shell/platform support claim.

### Internal command transport (R6 / F6)

Both `__powershell` and `__powershell-json` accept the same positional arguments:
`<base64-command> <version> <edition> [base64-record ...]`. The first argument is
canonical, padded standard Base64 of the command's UTF-8 bytes. There is no raw
command fallback or format autodetection. Regenerate the session bridge with
`whichwhy init powershell` after upgrading from the previous internal protocol.

PowerShell 5.1 and 7 both encode with
`[Convert]::ToBase64String([Text.UTF8Encoding]::new($false, $true).GetBytes($command))`
after passive discovery and restoration of the autoload preference. This emits
no BOM or line wrapping and rejects unpaired UTF-16 surrogates instead of silently
replacing them. Base64 contains no whitespace, quotes, or backslashes that could
change legacy native argument boundaries. Only the command is newly encoded:
the shell-provided version's numeric/dotted or prerelease spelling and the known
edition values `Desktop`/`Core` are quoting-safe. No broader envelope is needed.

`powershell.DecodeEvidence` decodes the command exactly once, before constructing
typed evidence. It rejects invalid Base64, noncanonical padding bits, CR/LF,
invalid UTF-8, and an empty decoded command. Both hidden endpoints return exit 2
with a `whichwhy:` error and no report on malformed requests, including incomplete
arguments. No partial decode is accepted. Valid command bytes are not trimmed,
case-folded, Unicode-normalized, or decoded again by resolution or presentation.
Whitespace-only strings are nonempty at the codec boundary. Invalid UTF-16 in the
PowerShell caller fails with an encoding exception before a native request exists.

The evidence records retain their previous Base64 encoding, five-field ordering
(type, name, source, path, alias target), separator, and semantics. Resolution
ordering, scope, policy, claim strength, and human/JSON schema are unchanged.
Explicit inspection uses this same transport, including reserved names and
wildcard characters. Public parsing never reinterprets an `inspect` operand as
an internal endpoint. The bridge refuses user attempts to forward private requests.
The native private wire shape remains a caller-supplied evidence interface, not
an authentication mechanism: naming an endpoint alone is insufficient, the full
wire request must pass the existing decoder. No new public evidence input exists.

`scripts/test-powershell-transport.ps1` tests the actual generated bridge and native
CLI in fresh Windows PowerShell 5.1 and PowerShell 7 processes. It is also called
by the existing oracle suite in Windows CI. Tests compare command names and human
headings using ordinal equality, and candidates against passive `Get-Command`.
Real session aliases cover normal names, spaces, apostrophes, quotes, punctuation,
Unicode (including combining and supplementary characters), and a Base64-looking
name that detects double decoding. Backslash and mixed quote/backslash cases use
missing qualified names: PowerShell interprets backslashes as path/module
qualification, so those cases do not claim exact-name alias discovery. Markers
must remain absent. Malformed native requests and unpaired UTF-16 are also tested.
The request-transport harness temporarily selects UTF-8 stdout decoding and
restores it. Separate response regressions start with the untouched caller
encoding and also exercise CP437, CP1252 and UTF-8. The bridge explicitly reads
the known CLI's stdout and stderr as strict UTF-8 via redirected process streams;
it never assigns console input/output encoding or PowerShell `OutputEncoding`.
Both pipes drain asynchronously, then stdout lines enter the success stream and
stderr lines enter the error stream. Native exit status is retained before error
delivery, including when the caller treats an error as terminating. Only the
existing quoting-safe private request tokens use this adapter; public forwarding,
strict command encoding and candidate ordering remain unchanged.

Against pre-F6 commit `be55f0eb5970cf589cf9bd3544de60dfbce1d18e`, Windows
PowerShell 5.1.26100.9444 changed `name"with"quote` to `namewithquote` and disrupted
the arguments for `mixed space "quote" \end\` and `mixed \"quote" space\`.
PowerShell 7.6.5's default native argument mode passed those same fixtures.
The encoded bridge passes all 15 identity fixtures on both versions, including
both human and JSON output; the existing oracle and F1 passive suites remain
required. These are Windows native-boundary results, not Unix runtime evidence.

## Shared result and presentation

`internal/resolution.Report` contains the completed claim, selected candidate,
ordered candidates, alternatives, reason, shell metadata, and limitations.
Human and JSON renderers receive this report. They do not select the first
candidate or reconstruct reasons independently. Missing results retain scope,
policy, claim strength, and limitations. Claim strength describes the evidence
class, not a probability or a promise of execution.

Exit status 0 means a candidate was selected among observed candidates within the
reported scope, including `precedence-uncertain` process selections; 1 means
no candidate was observed, with completeness reported separately; 2 remains an
operational/usage error (including Unix identity or ownership metadata unavailable
for eligibility evaluation). Doctor retains its warning/status convention and
executable-identity deduplication, extending warnings to incomplete inspection.
It consumes a process report for selection, claim and limitations.

The public compatibility rules are frozen in [json-contract.md](json-contract.md).
Command inspection JSON uses schema version 2:

- `resolution_scope`, `policy`, and `claim_strength` identify the claim;
- `selected` is an object or explicit null, replacing `winner` for both scopes;
- `selection_reason` replaces `winner_reason`;
- `no_candidate_reason` explains a null selection;
- `candidates`, `shell`, and `limitations` retain their roles and candidate order.

Doctor also uses version 2, renames `command_discovery.path_winner` to
`path_selected`, and includes scope, policy, and claim strength in
`command_discovery`. Its top-level limitations come from the same report.
PATH diagnostic JSON remains version 1 with focused additive F2 fields:

- `policy` names `process-path-order-v1` as the parsing context;
- `raw_value` retains the complete original PATH;
- each entry's `value` retains raw segment text, and new `effective_value` gives
  the filesystem operand after quote removal and empty-to-dot interpretation.

Existing indices now follow the shared parsed sequence: quoted Windows semicolons
no longer create phantom entries. An entirely empty PATH now has zero entries;
an unset PATH still returns the existing error/exit status with an empty list.
`empty` means an empty interpreted value, independently of filesystem status.
Duplicate counts now include equivalent empty/relative/quoted directory operands.
Human PATH output uses the same report, shows raw PATH plus changed raw/effective
entry pairs, combines EMPTY with filesystem status, and names the process parsing
context and shell limitation. This is a focused correction of diagnostic evidence,
not the final F10 compatibility contract. Command and doctor JSON remain version 2;
PowerShell JSON is unchanged.

## Controlled Windows evidence

`go test ./internal/cli -run TestWindowsPolicyVersusCmdOracle -v` runs five isolated
cases with disposable, same-named `.cmd` files in the current directory and a
PATH directory. With PATH containing only the latter, the policy selects the
PATH file; fresh `cmd.exe /D /Q /C` executes the current-directory file. With a
leading empty PATH entry, the policy selects the current-directory file too.
Agreement in that second case does not upgrade the process evidence class.
With an entirely empty PATH, the policy observes no candidates and exits 1,
while cmd still executes the current-directory fixture. A null selection must
therefore preserve uncertainty too.

The release gate also checks normal PATH-only order without a cwd competitor,
and duplicate PATH entries retaining the first producer's index. Cmd agrees on
selection in those two cases; full candidate correlation remains a filesystem
policy assertion. See the [oracle contract](oracle-contract.md) for the complete
claim-to-oracle matrix and required native CI capabilities.

Both human and JSON inspection run before the oracle and must leave an execution
marker absent. Only the oracle executes the known fixture. `/D` disables cmd
AutoRun and the child environment omits `NoDefaultCurrentDirectoryInExePath` so
the tested default search behavior is controlled. Tests never invoke an
arbitrary inspected command. This fixture demonstrates the need for scoping;
it does not establish general cmd.exe support.

`TestWindowsQuotedPATHCorrelation` constructs `".../quoted;directory";.../last`:
the old raw delimiter split yields three entries while the R1 resolver sees two.
It was run against the pre-R2 implementation and failed with two candidates versus
three diagnostic entries. After R2, both candidates correlate with their actual
directory entries #1 and #2, with the original quote text retained. Further
Windows coverage combines quoted relative directories, quoted and literal empty
entries, absolute duplicates, and trailing empties. Cross-platform coverage checks
empty PATH, repeated empty segments, relative/absolute duplicate order, raw
reconstruction, human/JSON agreement, and passive inspection. These tests execute
no inspected fixtures; the existing controlled cmd oracle remains separate.

The existing PowerShell 5.1 and 7 oracle scripts still compare the complete
ordered list against the real shell and run the passive-inspection suite.

## Dotted Windows names (R3 / F4)

A dot in a name is not evidence that its suffix belongs to PATHEXT. With
`.EXE;.CMD`, the exact policy name sequences are:

| Command | Names checked within each PATH directory |
| --- | --- |
| `wwprobe` | `wwprobe.EXE`, `wwprobe.CMD` |
| `wwprobe.v1` | `wwprobe.v1`, `wwprobe.v1.EXE`, `wwprobe.v1.CMD` |
| `wwprobe.alpha.v1` | `wwprobe.alpha.v1`, `wwprobe.alpha.v1.EXE`, `wwprobe.alpha.v1.CMD` |
| `wwprobe.cmd` | `wwprobe.cmd` |
| `wwprobe.CmD` | `wwprobe.CmD` |
| `wwprobe.` | `wwprobe.`, `wwprobe..EXE`, `wwprobe..CMD` |

Recognition uses the parsed PATHEXT (or its default), not a hardcoded list of
suffixes: `.V1;.CMD` makes `wwprobe.v1` literal-only; `.EXE` makes `.cmd` an
unrecognized suffix. Existing parsing remains unchanged, including trimming,
dot insertion, and fallback. These normalization choices are process policy,
not assertions about every shell's treatment of malformed PATHEXT. No new
validation of unusual or multi-dot PATHEXT entries is introduced.

`go test ./internal/cli -run TestWindowsDottedPATHEXTOracles -count=1 -v`
creates disposable native executables and marker-writing `.cmd` fixtures. It
compares standalone human/JSON inspection with fresh `cmd.exe /D /Q /C`
invocation and fresh PowerShell `Get-Command -All -ListImported` discovery with
module auto-loading disabled. It runs under the existing Windows Go CI job;
an unavailable PowerShell executable fails required CI and may explicitly skip on
constrained local hosts. Both
PowerShell versions were available locally: 5.1.26100.9444 and 7.6.5.

The controlled observations on that Windows host were:

- With only appended files present, cmd selects and both PowerShell versions
  discover extensionless, `.v1`, and multi-dot names in PATHEXT order. Reversing
  `.EXE;.CMD` reverses selection/order; a repeated `.cmd` adds no candidate.
- With a native literal `wwprobe.v1` also present, cmd executes it and both
  PowerShell versions list it before its appended `.exe` and `.cmd` alternatives.
  Keeping the literal dotted candidate preserves previous process enumeration
  and this observed ordering, while adding the formerly missed alternatives.
- For explicit `.cmd` and mixed-case `.CmD`, cmd selects the literal when present.
  Both PowerShell versions also list appended `.cmd.exe` and `.cmd.cmd` files.
  When the literal is absent, cmd executes `.cmd.exe` and PowerShell discovers
  the appended alternatives. **The process policy deliberately retains its
  literal-only recognized-suffix rule**, so it does not enumerate that fallback.
  A policy miss in this case is not a shell-unavailability claim.
- PowerShell also discovers a native literal extensionless file after the
  appended candidates, or alone. Cmd does not execute the extensionless-only
  fixture through this PATH search. The process policy retains its existing
  exclusion of literal extensionless files.
- For `wwprobe.` with `wwprobe.cmd` and `wwprobe..cmd` present, cmd executes and
  both PowerShell versions discover `wwprobe..cmd`. The rule appends without
  stripping the trailing dot. Other Windows trailing-dot filesystem aliases
  remain subject to the existing `os.Stat` behavior; no general claim is made.

These fixtures establish the focused defect and ordering decisions, not universal
Windows-shell equivalence. R1 scope/policy/claim strength remain
`process-external` / `process-path-order-v1` / `policy-only`. R2 still supplies
the original PATH entry indices and raw evidence. `TestWindowsDottedPATHCorrelation`
checks missing and quoted entries, duplicate directories, extension ordering,
and human/JSON correlation at PATH #2 and #4. Name-level tests check ordering
before filesystem/path dedup can hide a defect. The `.v1.cmd` regression and
unrecognized-suffix name tests were observed failing against the pre-F4 code.

Human/JSON inspection and PowerShell discovery must leave the execution marker
absent. Only the cmd oracle invokes the explicitly constructed harmless fixtures,
and it verifies their identifying output and marker. Generic inspection gained
no execution probe, shell invocation, or presentation-side precedence logic.

## Unix user-aware eligibility (R4 / F5)

This is a **mode-class policy**, not an OS access check or execution probe. Before
enumerating a nonempty parsed PATH, collect `os.Geteuid()`, `os.Getegid()` and
`os.Getgroups()` once for this resolution. These are the WhichWhy process's
effective UID, effective primary GID, and supplementary numeric group IDs. They
are not the real IDs, login name, environment variables, account-database group
list, or an unobserved parent shell's credentials. No identity is changed. With
set-ID invocation, the effective identity is intentional. Empty PATH needs no
identity collection because it supplies no candidate operands.

For each successfully statted non-directory operand, use the target's numeric
owner UID, group GID and permission mode from that same `os.Stat` result:

1. Effective UID 0: accept if any owner, group or other execute bit is set. With
   no execute bit, reject even for root. This is the traditional superuser mode
   rule for regular executables, not proof of kernel superuser privileges.
2. Otherwise, if effective UID equals file UID, only `0100` governs eligibility.
   A missing owner execute bit denies; never fall through to group or other.
3. Otherwise, if file GID equals effective GID **or any** ID in `os.Getgroups()`,
   only `0010` governs. A missing group execute bit denies; never fall through to
   other. Duplicate groups do not change the result, and effective GID is checked
   separately whether or not `Getgroups` includes it.
4. Only when neither owner nor group class applies does `0001` govern.

Read/write, setuid/setgid and sticky bits do not grant execute eligibility.
The existing non-directory filter remains; this is not a new file-format or
regular-file validity check. `os.Stat` still follows symlinks, including for
UID/GID/mode; missing targets yield a retained `not-found` observation. Candidate
paths retain the link spelling. Lexical deduplication, first-producing-entry order, original
PATH indices and raw PATH evidence are unchanged.

Group collection failure aborts with a wrapped operational error, preserving its
cause; it is never interpreted as an empty group list. Missing Unix ownership
metadata likewise aborts with the candidate path in the error. No completed
no-candidate report is produced on either failure. This uses the existing error
channel; F7 does not downgrade those failures into per-attempt observations.

### What this proves, and alternatives considered

It proves eligibility under the recorded policy using observed identity and mode
facts. It does **not** prove kernel access or successful `exec`:

- ACLs can make actual access differ in either direction; in particular, POSIX
  ACL group bits may represent an ACL mask rather than the owning group's grant.
- A `noexec` mount can deny execution despite eligible mode bits. No mount table
  or ancestor permission analysis is added; Stat success is not execution access.
- Linux filesystem UID/GID can differ from effective IDs, and capabilities can
  grant or remove the privileges this traditional UID-0 policy assumes. macOS
  extended/directory-service group membership and other platform protections are
  not modeled by the numeric process group list. These are disclosed limits.
- Identity collection, Stat calls and later invocation are not atomic. Changes
  to credentials, groups, permissions, symlink targets or files can invalidate
  the observations. Shebang/interpreter availability, interpreter read access,
  executable format and other kernel restrictions can also prevent execution.

`access(X_OK)` was rejected because it uses real identity semantics; Apple's
archived documentation also cautions that privileged X_OK success need not imply
execute bits. POSIX `faccessat(..., AT_EACCESS)` specifies effective-ID checks,
but actual availability and wrapper behavior matter. Linux `faccessat2` supports
flags in-kernel; older faccessat implementations may emulate them with mode bits
and miss ACLs. The installed Go 1.27.1 `syscall.Faccessat` has a Linux fallback,
and no corresponding Darwin function. Adding `golang.org/x/sys/unix` or cgo would
require platform-specific handling and probe-error semantics without eliminating
races or execution failures. An explicitly limited, dependency-free mode policy
is the smaller truthful F5 change. `syscall.Stat_t` is used only to read UID/GID
already supplied by `os.Stat`; no candidate is opened for execution.

Semantic references checked before implementation:

- [Linux pathname permission and privilege rules](https://man7.org/linux/man-pages/man7/path_resolution.7.html).
- [POSIX access/faccessat identity semantics](https://pubs.opengroup.org/onlinepubs/9799919799/functions/access.html)
  and [Linux access/faccessat caveats](https://man7.org/linux/man-pages/man2/access.2.html).
- [Apple access documentation](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/access.2.html)
  and [XNU vnode_authorize_posix / vnode authorization](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/vfs/vfs_subr.c),
  including the regular-file root execute-bit requirement.

### Focused verification boundary

Portable tests enumerate all 512 permission modes for owner, effective group,
supplementary group, duplicate groups, other and root identities, including
special bits and explicit pre-F5 false positives. Unix-native tests collect real
effective credentials and test exact modes `0100`, `0010`, `0001`, `0011`, `0111`
and `0000` through both direct and symlink candidates. Failure tests ensure missing
groups/ownership never become definitive ineligibility.

`go test ./internal/cli -run TestUnixOwnerEligibilityOracle -count=1 -v` creates
a disposable, current-user-owned `/bin/sh` script. Each execute-bit pattern above
is combined with owner read (`0400`) so interpreter read access does not obscure
execute permission. Human and JSON inspection must leave its marker absent;
only explicit test-oracle execution may write it. A successful `0700` control
establishes basic fixture execution first. Owner group/other-only modes pass the
literal pre-F5 predicate but must be rejected by inspection and fail direct
controlled execution with EACCES. Root skips this unprivileged assertion, and
an unusable control or different fixture owner produces an explanatory skip.
Unexpected oracle disagreement is a failure, not silently skipped.

PATH/report tests combine rejected entries, duplicates and a symlink and retain
one-based correlation, policy/scope/claim strength and shared human/JSON limits.
The existing Ubuntu/macOS CI `go test ./...` jobs automatically include all these
Unix tests. The Windows development host runs the pure model and Windows suites;
cross-compilation is **not native Unix runtime verification**. Linux/macOS native
execution is **PENDING REMOTE CI** for this mission. The `unix` build constraint
uses APIs shared by Linux and Darwin; other Unix targets gain no runtime support
claim, and non-Unix/non-Windows targets return an explicit unsupported error.

R1/R2/R3 identifiers and Windows behavior remain unchanged. These fixtures do not
establish Bash, Zsh or Fish equivalence; the evidence remains process-policy only.

## Retained candidate observations (R5 / F7)

The collectors formerly discarded Stat errors, directories and mode-ineligible
files through `continue`. The same clean-looking miss could mean either observed
absence or failed inspection. A later candidate also lacked evidence explaining
whether an unobserved earlier operand could precede it.

`resolver.Result.Observations` now retains every attempted candidate operand.
Windows supplies the unchanged R3 name sequence; Unix supplies one name and the
unchanged R4 effective-identity eligibility function. A small shared collector
owns Stat, outcome retention and candidate path normalization. Identity collection
remains Unix-specific and happens once before a nonempty search.

Each observation carries a one-based `attempt`, original one-based `path_index`,
generated `name`, actual Stat operand `path`, semantic `status`, and optional
`error` containing `operation`, `category` and diagnostic `message`. Raw error
text is supplementary, never the machine-readable classification. `path_index`
correlates with the same R2 parsed entry used by candidates, including empty,
relative, quoted and repeated entries. The Stat operand can be relative; the
candidate path still uses absolute spelling when available.

| Status | Meaning | Reduces completeness? |
| --- | --- | --- |
| `candidate` | Observed non-directory eligible under the platform filter | Only if absolute-path conversion failed |
| `not-found` | Stat reported file/path/target absent | No |
| `directory` | Stat succeeded and the operand is a directory | No |
| `not-directory` | Unix Stat returned ENOTDIR for a path component | No |
| `mode-ineligible` | Stat succeeded; R4 mode-class eligibility rejected the file | No |
| `error` | Stat could not establish an eligible candidate or ordinary negative | Yes |

Stat error categories are `not-found`, `not-directory`, `permission-denied` and
`other`. ENOENT / `os.ErrNotExist` is ordinary negative evidence on Unix; EACCES
and EPERM are `permission-denied`, while symlink loops, I/O errors, invalid
operands, unavailable network paths and unexpected errors remain `other` failures.
Wrapped errors retain the same classification. No retry, ACL manipulation,
candidate execution, or additional filesystem traversal is introduced.

Windows file/path-not-found codes are ordinary negatives. Go's Windows
`syscall.ENOTDIR` aliases `ERROR_PATH_NOT_FOUND`, so that code cannot establish
a known non-directory component. `ERROR_DIRECTORY` means an invalid directory
name and is conservatively an `other` failure. Go also considers
`ERROR_BAD_NETPATH` an `os.ErrNotExist` match; F7 deliberately preserves it as an
`other` failure rather than treating an unavailable network path as candidate
absence. Classification uses the error actually supplied by Stat, not guesses
from path spelling. Some malformed or overlong paths can return path-not-found
on Windows; those observations retain that platform report without inventing a
more specific diagnosis.

These distinctions follow [Go's error matching](https://go.dev/src/os/error.go)
and [Windows error definitions](https://learn.microsoft.com/en-us/windows/win32/debug/system-error-codes--0-499-),
with the installed Go platform mappings checked and regression-tested. Stat
follows symlinks. A broken link whose target lookup returns ENOENT is `not-found`;
this says the target lookup failed with absence, not that the link itself is
absent. Without Lstat the collector cannot distinguish a missing link from a
missing target, and does not claim to. A denied or looping target is an `error`.

Absolute-path conversion is attempted only for eligible candidates, as before.
Its failure retains `status=candidate`, an `absolute-path` / `other` error, and
the existing cleaned candidate-path fallback. Inspection becomes incomplete;
this failure alone does not create an unknown earlier eligible operand. Candidate
identity/dedup uses the documented fallback spelling and remains a lexical claim.

Missing identity/groups or missing ownership metadata abort the entire collection
with the existing wrapped operational error. No completed partial report is
returned: the eligibility policy itself could not be evaluated truthfully.

### Ordering, duplicates and completed claims

Observation order is exactly PATH entry order, then generated-name order within
each entry. The attempt number indexes that sequence; it is independent of
candidate deduplication. Repeated PATH entries retain every actual attempt, even
if their eligible candidates later collapse to the first visible path. Generated
Windows names are still deduplicated by R3 before any filesystem attempt. F7
does not invent records for names that were never attempted.

`resolution.ProcessExternal` produces `inspection.completeness=complete` or
`incomplete` and process `selection_status`:

- `definitive`: the first observed eligible candidate has no preceding unresolved
  Stat attempt. This is definitive only within the observed process policy.
- `precedence-uncertain`: a preceding unresolved attempt could have produced an
  earlier candidate. `selected` remains the first **observed** candidate, with a
  reason explicitly qualifying its precedence. Alternatives remain ordered.
- `no-candidate`: no eligible candidate was observed. A complete miss and an
  incomplete miss have different completeness and explanation, both with
  `selected=null`. Empty PATH has zero attempts and is complete under this policy.

Every retained failure makes collection incomplete. Failures exclusively after
the first observed candidate do not invalidate that candidate's established
precedence; they may conceal alternatives. A failure before a repeated later
success remains unresolved even if the visible path repeats: observations are
not an atomic snapshot. No completeness state promises later execution success.
Scope, policy and claim strength remain `process-external`,
`process-path-order-v1`, and `policy-only`; there is no probability score or
enumeration-policy version bump.

Exit codes do not change: 0 for a selected observed candidate (check
`selection_status` for definitive precedence), 1 for no observed candidate (check
completeness for unresolved attempts), and 2 for usage/operational errors. An
incomplete inspection is retained evidence, not automatically a fatal error.

### Presentation and doctor

Human output shows completeness and selection status. Only incomplete attempts
are expanded by default, with attempt number, PATH index, quoted operand,
operation, category and diagnostic message. Normal negative attempts stay in the
model/JSON to avoid a trace dump. Uncertain selection has an explicit heading
and qualified reason. Both renderers consume the same completed report.

Process command JSON version 2 gains `process_path` (raw value and original
entries with `index`, `raw`, `value`), `inspection` (completeness and full ordered
observations), and `selection_status`. Empty observation arrays remain arrays.
Existing candidate `path_index` is unchanged. These are focused schema additions,
not the F10 final compatibility freeze. PowerShell reports have no process
inspection fields; their schema and shell selection remain unchanged.

Doctor's `command_discovery` carries the same three additions plus the shared
selection/no-candidate reason. Its discovery state becomes `uncertain` for an
uncertain selection or incomplete miss, and its existing warning exit is 1.
A later failure preserves `current`/`different` discovery but produces a warning
for incomplete inspection. Existing executable-identity deduplication and lexical
fallback remain unchanged; it does not remove observation history. The standalone
`path` command's directory diagnostics and schema are unchanged.

### Focused evidence

`TestFilesystemObservationSilentLossRegression` uses only pre-F7 APIs. It runs
real filesystem inspection for a complete miss, a malformed/overlong PATH operand,
failure before a candidate, and failure after one. Both presentations must agree
and leave a marker absent. Against parent `b883d841af906283718fa30906c5a1030f606a71`,
all four cases failed because JSON had no retained observations; the two misses
had identical no-candidate reasons, and the earlier failure still produced an
unqualified first-observed selection reason. The same test passes after F7.

Injected collector tests cover permission failures without flaky Windows ACL
fixtures, Windows error mappings, PATH/name ordering, repeated entries, eligible
and skipped files, wrapped errors, fatal eligibility and absolute-path conversion
failure. Native Unix tests retain exact R4 target-mode outcomes, broken links,
looping links, ENOTDIR, effective credentials and fatal ownership failures.
The existing Windows/cmd/PowerShell oracle suites remain unchanged. Ubuntu/macOS
native test jobs remain required; Windows-host cross-compilation proves only
buildability, never native Unix runtime success.

## Deferred work

F8 verification is specified in the [oracle contract](oracle-contract.md).
F10 final JSON compatibility is specified in [json-contract.md](json-contract.md).
Naming, other shells, and package/version-manager intelligence
are outside this change. Exact shell parity for recognized-suffix fallback,
literal extensionless discovery, and unusual PATHEXT parsing is also outside F4;
the controlled differences above remain explicit process-policy limitations.
