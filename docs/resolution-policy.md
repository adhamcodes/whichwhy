# Resolution claims

Standalone inspection reports a **policy-selected candidate**, not the command
that an unobserved shell will execute. The policy name is
`process-path-order-v1`; its scope is `process-external` and its claim strength
is `policy-only`. These identifiers are intentionally tested as literal values.

## Exact standalone policy

This policy names the existing algorithm; it does not repair its known gaps.

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
4. On Windows, if `filepath.Ext(command)` is nonempty, check that name alone.
   Otherwise split PATHEXT on semicolons, trim whitespace, discard empty values,
   and prepend a dot to extensions that lack one. If no extensions remain,
   use `.COM`, `.EXE`, `.BAT`, `.CMD`. Generate names in extension order,
   removing case-insensitive duplicate names. Directory order takes precedence
   over extension order. This includes the existing dotted-name/PATHEXT gap.
5. Use `os.Stat` (following symlinks). On Windows accept any successfully observed
   non-directory candidate. On Unix accept successfully observed non-directory
   candidates with any of the `0111` permission bits set. Neither filter proves
   that the invoking user can successfully execute the candidate.
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
no candidate was observed; 2 remains an operational/usage error. Doctor retains
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

## Deferred work

F4 dotted Windows names, F5 Unix user
permissions, F6 PowerShell 5.1 transport, F7 observation retention, F8 broader
oracles, F9 inspection routing/UX, and F10 final JSON compatibility remain
separate missions. Naming, other shells, and package/version-manager intelligence
are outside this change.
