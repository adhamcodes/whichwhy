# JSON and machine-error contract

This is the public machine interface for the current CLI. Resolution meaning and
support boundaries remain in [resolution-policy.md](resolution-policy.md) and
[oracle-contract.md](oracle-contract.md). A selected candidate never promises
successful execution. Inspection is passive.

## Families and compatibility

| Invocation | Family | `schema_version` | `kind` |
| --- | --- | --- | --- |
| `<command> --json` | command | 2 | absent |
| `inspect <literal-command> --json` | command | 2 | absent |
| Either inspection form through the active PowerShell bridge | command | 2 | absent |
| `path --json` | PATH | 1 | `path` |
| `doctor --json` | doctor | 2 | `doctor` |

Versions are independent positive integers per document family, not a global
version, CLI release number, evidence-policy revision, or confidence level. The
equal command/doctor numbers do not couple them. Command retains its existing
shape without `kind`: the requesting operation identifies the family, and adding
a redundant discriminator does not justify changing that established shape at
this freeze. A consumer storing mixed reports must retain the requested family;
`schema_version` alone cannot identify it.

Within a family/version, required keys, types, presence/null/empty rules, existing
machine identifiers and semantic array order are stable. Removing/renaming fields,
changing those rules or existing meanings requires an affected-family version
bump. New optional fields may be additive without a bump; consumers should ignore
unknown keys. New enum values require a version decision too: the finite semantic
sets below are frozen; PowerShell candidate kinds have the explicitly open mapping
described below. Unknown versions or identifiers must not be interpreted as a
known successful/definitive state. Prose, whitespace, object-key order and the
number/order of limitations may change without a version bump.

F10 retains command 2 / PATH 1 / doctor 2: there is no structural change to reports
produced by the real collectors. Presentation now owns nested wire types and
normalizes nil internal collections to arrays, enforcing the existing empty-list
contract even when a completed model uses nil slices. No resolution policy changes.
Independent serialized-value expectations in `internal/cli/json_contract_test.go`
guard this contract. An intentional additive change still requires updating and
reviewing the compatibility expectations and this document.

## Exit codes, streams and encoding

| Surface | Exit 0 | Exit 1 |
| --- | --- | --- |
| Command, including PowerShell and explicit inspection | An observed candidate is selected | No candidate observed |
| PATH | PATH is set; includes empty PATH and directory warnings/errors | PATH is unset |
| Doctor | Healthy discovery (`status=ok`) | Discovery warning (`status=warning`) |

Command exit 0 includes incomplete inspection and `precedence-uncertain` selection;
exit 1 includes incomplete misses. Inspect the structured claim and completeness,
not just the exit code. PATH diagnostics do not turn individual entry problems into
exit 1. A set empty PATH and an unset PATH are distinct for `path --json`; ordinary
command and doctor collection read both as the same empty process PATH.

For exits 0/1, stdout contains exactly one complete UTF-8 JSON object, without BOM
or human banner, followed by one LF newline; stderr is empty. No whitespace or
object-key ordering is promised. Valid Unicode identity (including combining
sequences) survives decoding without normalization. Quotes, backslashes and
controls use JSON escaping; HTML escaping is disabled (`<`, `>` and `&` remain
literal). U+2028/U+2029 may be escaped. Go's encoding of invalid UTF-8 bytes uses
U+FFFD; arbitrary non-UTF-8 Unix filename bytes are not a lossless string interface.
The hidden command transport rejects invalid UTF-8 before constructing a report.

Exit 2 means usage, malformed internal protocol, operational, serialization or
output failure. Before output delivery starts, stdout is empty and stderr contains
a human diagnostic beginning `whichwhy: ` and ending in a newline. There is no
JSON error envelope and no stable prose error code. Do not parse the diagnostic.
Resolver/eligibility failures and failure to locate the running executable produce
no report, even if some evidence had already been collected. Retained per-attempt
errors instead belong inside a valid report; they do not write stderr.

Serialization completes in memory before the first stdout write. A failed or
short write returns exit 2 when the writer reports control back to the CLI. A pipe,
file or terminal can accept a prefix before failing; already delivered bytes cannot
be retracted. Signals/process termination may also prevent delivery or the normal
exit/diagnostic. Consumers must reject truncated JSON and never use partial output
as a report. This physical delivery limitation is not a second partial-report API.

`--json` is only supported in the invocation forms above. Help/version/init output
is human text or shell code; help/version routing still takes precedence even with
extra arguments. To inspect a reserved identity, use `inspect`. Private PowerShell
endpoints are bridge transport, not public evidence-input syntax; malformed private
requests use the same exit-2 behavior. Bridge discovery/guard/encoding failures that
occur before launching the CLI are PowerShell errors, not CLI JSON reports.

## Command fields

All listed fields are required unless a presence condition is stated. JSON strings
are strings even when empty; indices and counters are integers; flags are booleans.
There are no nullable fields except command `selected`.

| Field | Shape / presence |
| --- | --- |
| `schema_version`, `command` | Integer 2; original requested string, separate from candidate identity |
| `resolution_scope`, `policy`, `claim_strength` | Stable strings in the claim table below |
| `selected` | Candidate object, or explicit `null` for no selection; never omitted |
| `candidates` | Always an array, including `[]` on a miss |
| `selection_reason` | Nonempty prose when selected; otherwise omitted |
| `no_candidate_reason` | Nonempty prose when unselected; otherwise omitted |
| `limitations` | Always an array of prose strings, including `[]` if none |
| `shell` | PowerShell only: object with required `name` (`PowerShell`), `version` strings; `edition` omitted when empty, otherwise observed `Desktop`/`Core` |
| `process_path`, `inspection`, `selection_status` | Required for process reports, omitted for PowerShell reports; never null |

| `resolution_scope` | `policy` | `claim_strength` |
| --- | --- | --- |
| `process-external` | `process-path-order-v1` | `policy-only` |
| `powershell-loaded-session` | `powershell-loaded-session-order-v1` | `shell-observed` |

The PowerShell claim covers passive discovery in the active loaded session, not
unloaded-module behavior or invocation success. Standalone inspection does not
observe the invoking shell. Explicit inspection changes routing and preserves the
literal request; it uses the same family and evidence limits as ordinary inspection.

Candidate objects always have `kind`. Nonempty `name`, `path`, `source`, and
`alias_target` are included; empty values are omitted, never null. `path_index` is
included only when nonzero: a one-based original process PATH entry index. Shell
candidates omit it. Metadata presence depends on the observed CommandInfo, not
on guessing a module, target, path or command identity.

| Candidate kind | Metadata |
| --- | --- |
| `external` (process) | `path`, `path_index`; no `name`, `source`, `alias_target` |
| `alias` | Observed `name`, optional `source`, `alias_target` if nonempty |
| `function`, `filter`, `configuration` | Observed `name`, optional `source` |
| `cmdlet` | Observed `name`, optional `source` (module) |
| `application`, `external-script` | Observed `name`, `path`, `source` when supplied |

PowerShell kind normalization is open: `ExternalScript` becomes `external-script`;
other supplied type names are lowercased. The listed values are stable known
mappings, not a promise to reject future shell command types. Consumers should
retain unknown kinds without inventing execution semantics. Paths/source/alias
targets are identity data, not machine enums. Candidate names may differ in case,
qualification or suffix from the original `command`.

## Process evidence (command and doctor)

`process_path` has exactly these current fields: `raw_value` (complete process PATH
string) and `entries` (always an array). Each entry has required `index` (one-based),
`raw` (original segment), and `value` (parsed segment). Empty `value` remains `""`
here; its filesystem operand is `.`. No skipped/duplicate entries are removed.

`inspection` has required `completeness` (`complete` or `incomplete`) and
`observations` (always an array, including `[]` for zero attempts). Each observation
has required integer `attempt`, integer `path_index`, strings `name`, `path`,
`status`, and optional `error` object. Both indices are one-based. `path` is the
actual attempted operand, which may be relative, while candidate paths normally
use absolute spelling. `error` is omitted when absent, never null; when present
it requires strings `operation`, `category`, and `message`.

| Stable field | Values / meaning |
| --- | --- |
| `selection_status` | `definitive` (within policy), `precedence-uncertain` (earlier unresolved attempt), `no-candidate` |
| Observation `status` | `candidate`, `not-found`, `directory`, `not-directory`, `mode-ineligible`, `error` |
| Error `operation` | `stat`, `absolute-path` |
| Error `category` | `not-found`, `not-directory`, `permission-denied`, `other` |

Ordinary Stat negatives retain `error` objects with their category/message, despite
not reducing completeness. Directories and mode-ineligible files have no error.
A candidate can carry an `absolute-path` error and retain its cleaned path fallback.
An `error` status or candidate normalization error makes inspection incomplete;
only an unresolved earlier attempt makes selected precedence uncertain. No-candidate
reports require completeness to distinguish ordinary misses from failed inspection.
`message`, reasons and limitations are explanatory prose, never stable identifiers.

## PATH document

Required fields: `schema_version` (1), `kind` (`path`), `scope` (`process-path`),
`policy` (`process-path-order-v1`), `raw_value` (complete string), `entries` (array),
and `summary` (object). Optional top-level `error` is a nonempty prose string for
unset PATH. Consumers distinguish unset with error presence/exit 1, not its wording.

Every entry requires `index`, `value` (raw segment, unlike process entry `value`),
`effective_value` (parsed filesystem operand, empty becomes `.`), `directory`,
`missing`, and `empty`. False flags remain present. `duplicate_of` is omitted unless
it identifies the first equivalent entry's one-based index. Entry `error` is omitted
unless a filesystem diagnostic is present; it is prose, not the observation error
object. `empty` and duplication are independent of the filesystem state.

Duplicate equivalence uses cleaned absolute effective directory strings where
available, cleaned fallback otherwise; Windows compares case-insensitively. It
does not resolve symlink identity or filter/renumber entries. Summary always
contains integer `entries`, `missing`, `duplicate`, `empty`, `not_directory`,
`errors` counters, including zero values. Counts may overlap (an empty/duplicate
entry can also have a filesystem problem).

Set empty PATH: raw `""`, entries `[]`, all counters zero, no error, exit 0.
Unset PATH: same shape plus top-level `error`, exit 1. Neither is a fatal failure.

## Doctor document

Required fields: `schema_version` (2), `kind` (`doctor`), `status` (`ok`/`warning`),
`version` (WhichWhy build/version string), `platform` (required `os` and `arch`
strings using Go platform names), `running_executable` (string), `command_discovery`
(object), `limitations` (array of prose). Build version and platform data are not
schema revisions; consumers must allow platform values beyond their own host.

Discovery requires `state`, `resolution_scope`, `policy`, `claim_strength`,
`other_candidates` (array of path strings), `process_path`, `inspection`, and
`selection_status`. It has the process claim/evidence shapes above. `path_selected`
is present as a nonempty path string only when selected; otherwise omitted.
Selection/no-candidate reason presence is the same as command reports. Limitations
remain top-level. There is no nullable discovery field.

`state` is `current` (selected path identifies the running executable), `different`,
`missing` (complete miss), or `uncertain` (uncertain selection/incomplete miss).
Doctor warns for different/missing/uncertain discovery, multiple distinct executable
candidates, or incomplete inspection. A later failure can therefore retain `current`
while `status=warning`. Alternatives preserve resolution order after doctor's
existing executable-identity deduplication, which may use filesystem identity and
otherwise lexical comparison. Deduplication does not erase observations.

Full observations intentionally remain inline, including ordinary not-found
PATH/PATHEXT attempts. Payload size scales with attempts and path/error text; long
Windows environments can yield hundreds of observations and hundreds of kilobytes.
This verbosity is acceptable for the evidence report: silently suppressing negatives
would obscure inspected operands, repeats and gaps, and a compact/range encoding
would add a second interpretation contract. Consumers may summarize the report for
display but should retain evidence when explaining uncertainty. There is no size
cap, truncation, omitted trace, or extra retrieval API.

## Ordering and consumption

Candidates retain observed/policy order. Current policies select the first observed
candidate, if any; `selected` is the authoritative completed selection and the
renderer must not recompute it. Process order is original PATH order then generated
name order, with documented candidate deduplication. PowerShell order is supplied
by the shell. PATH entries retain original order, including duplicates and empties.
Observations retain every attempt in order; `attempt` counts that sequence, separately
from candidate deduplication. Doctor alternatives retain resolution order after its
identity deduplication. Limitations have no guaranteed order, count or stable wording.

Examples below are complete documents with intentionally empty example limitations;
real reports include scope-specific limitations. A PowerShell miss (exit 1):

```json
{"schema_version":2,"command":"missing","resolution_scope":"powershell-loaded-session","policy":"powershell-loaded-session-order-v1","claim_strength":"shell-observed","shell":{"name":"PowerShell","version":"7.6.5","edition":"Core"},"selected":null,"candidates":[],"no_candidate_reason":"No command match was found in the current loaded PowerShell session.","limitations":[]}
```

Unset PATH (exit 1):

```json
{"schema_version":1,"kind":"path","scope":"process-path","policy":"process-path-order-v1","raw_value":"","entries":[],"summary":{"entries":0,"missing":0,"duplicate":0,"empty":0,"not_directory":0,"errors":0},"error":"PATH is not set"}
```

Doctor with empty PATH (exit 1; executable/platform are illustrative):

```json
{"schema_version":2,"kind":"doctor","status":"warning","version":"dev","platform":{"os":"linux","arch":"amd64"},"running_executable":"/tools/whichwhy","command_discovery":{"state":"missing","resolution_scope":"process-external","policy":"process-path-order-v1","claim_strength":"policy-only","other_candidates":[],"process_path":{"raw_value":"","entries":[]},"inspection":{"completeness":"complete","observations":[]},"selection_status":"no-candidate","no_candidate_reason":"No external command candidate was observed under this process policy."},"limitations":[]}
```
