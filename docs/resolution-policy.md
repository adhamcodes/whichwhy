# Resolution claims

Standalone inspection reports a **policy-selected candidate**, not the command
that an unobserved shell will execute. The policy name is
`process-path-order-v1`; its scope is `process-external` and its claim strength
is `policy-only`. These identifiers are intentionally tested as literal values.

## Exact standalone policy

This policy names the implemented algorithm, including the focused R2 PATH
evidence, R3 dotted-name and R4 Unix eligibility corrections; it does not establish
shell equivalence.

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
   order as alternatives **under this policy**. No selection means no observed
   candidate under this policy, not proof that a shell cannot find the command.

Stat failures and skipped candidates are not fully retained by the existing
collector. The report explicitly discloses that limitation. It also discloses
unobserved shell-local state and possible differences in shell external search
rules, including current-directory, empty-entry, and quoted-PATH behavior.

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
paths. Neither claims filesystem object identity. Failures to make comparison
keys absolute are not newly structured here (F7 remains deferred).

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
alias does not prove its target can execute. The F1 collector guard and transport
are unchanged. No discovery probe executes the inspected command.

## Shared result and presentation

`internal/resolution.Report` contains the completed claim, selected candidate,
ordered candidates, alternatives, reason, shell metadata, and limitations.
Human and JSON renderers receive this report. They do not select the first
candidate or reconstruct reasons independently. Missing results retain scope,
policy, claim strength, and limitations. Claim strength describes the evidence
class, not a probability or a promise of execution.

Exit status 0 means a candidate was selected within the reported scope; 1 means
no candidate was observed; 2 remains an operational/usage error (including Unix
identity or ownership metadata unavailable for eligibility evaluation). Doctor retains
its existing warning/status rules and executable-identity deduplication, but
consumes a process report for selection and reports its claim and limitations.

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

`go test ./internal/cli -run TestWindowsPolicyVersusCmdOracle -v` runs three isolated
cases with disposable, same-named `.cmd` files in the current directory and a
PATH directory. With PATH containing only the latter, the policy selects the
PATH file; fresh `cmd.exe /D /Q /C` executes the current-directory file. With a
leading empty PATH entry, the policy selects the current-directory file too.
Agreement in that second case does not upgrade the process evidence class.
With an entirely empty PATH, the policy observes no candidates and exits 1,
while cmd still executes the current-directory fixture. A null selection must
therefore preserve uncertainty too.

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
an unavailable PowerShell executable is an explicit skipped subtest. Both
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
UID/GID/mode; broken links yield the existing Stat skip. Candidate paths retain
the link spelling. Lexical deduplication, first-producing-entry order, original
PATH indices and raw PATH evidence are unchanged.

Group collection failure aborts with a wrapped operational error, preserving its
cause; it is never interpreted as an empty group list. Missing Unix ownership
metadata likewise aborts with the candidate path in the error. No completed
no-candidate report is produced on either failure. This uses the existing error
channel without introducing F7's structured per-candidate observation model.
Existing Stat failures/skips still have the existing explicit report limitation.

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

## Deferred work

F6 PowerShell 5.1 transport, F7 observation retention, F8 broader
oracles, F9 inspection routing/UX, and F10 final JSON compatibility remain
separate missions. Naming, other shells, and package/version-manager intelligence
are outside this change. Exact shell parity for recognized-suffix fallback,
literal extensionless discovery, and unusual PATHEXT parsing is also outside F4;
the controlled differences above remain explicit process-policy limitations.
