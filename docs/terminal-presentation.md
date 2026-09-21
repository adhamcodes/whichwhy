# Human presentation

Human output and frozen JSON consume the same completed resolution report.
`internal/cli/command_human.go`, `path.go`, and `doctor_human.go` organize the
answers; `human.go` owns text safety, hierarchy, and layout. They do not collect
evidence or recompute precedence. No dependency was added.

## States and information order

Command reports lead with the identity, scoped status, and primary answer.
`SELECTED` uses the completed selection; `OBSERVED CANDIDATE` marks process
precedence uncertainty. A complete process miss or loaded-session miss is
`NOT FOUND` within its stated scope. An incomplete process miss is
`NO DEFINITIVE RESULT`. A later observation failure leaves a definitive selection
intact but adds an immediate incomplete-inspection notice. Shell reports do not
invent process inspection completeness or invocation guarantees.

Why translates the report's established explanations into ordinary language;
unrecognized explanations pass through unchanged. It is followed by alternatives,
resolution, concise scope, and a passivity note. Technical policy identifiers and
the exhaustive generic limitation list remain in JSON and technical documentation.
Scope notes summarize shell differences and execution limits; incomplete reports
still show uncertainty and every observation failure. Only explicitly recognized
generic limitations are summarized; unfamiliar or result-specific limitations
remain visible under Additional limits. PowerShell retains supplied candidate
metadata (including source) and shell version/edition. Ordinary negative and
skipped filesystem attempts remain in JSON; human output lists every incomplete
observation, with its original attempt and PATH index and diagnostic.

PATH leads with summary counters and problem entries before the full ordered
entry list. The redundant whole raw PATH is available in JSON, not appended to the
human report. The header counts affected entries once; summary counts
can overlap. Empty/duplicate entries can also have filesystem problems. Raw and
effective spelling are shown together when different. Empty PATH is explicitly
identified; unset PATH retains its stderr/exit-1 behavior.

Doctor maps its existing successful report to healthy or attention and suggests
manual next steps. Fatal collection failures are reported as failure on stderr,
with exit 2 and no partial report. Red is reserved for actual entry errors;
attention/uncertainty uses yellow. No status depends on color alone.

## Capability policy

Detection examines the actual destination writer, not stdin or the presumed
parent shell. Unknown writers, pipes, files, and `TERM=dumb` receive plain text
without ANSI, tree characters, or borders. Human identity data remains UTF-8.
The locked help tagline remains UTF-8 even with ASCII decoration.

- Windows reads `GetConsoleMode`, visible window width, and output code page.
  ANSI requires virtual terminal processing already enabled; Unicode decoration
  requires UTF-8 output (65001). No console mode or encoding is changed.
- Linux/macOS read window size with `TIOCGWINSZ`; ANSI also requires a nonempty
  non-dumb `TERM`. Unicode decoration requires a UTF-8 locale, using `LC_ALL`,
  `LC_CTYPE`, then `LANG` precedence. Other platforms conservatively stay plain.
- Nonempty `NO_COLOR` disables all SGR styling, including dim text. No force-color
  variable overrides pipe detection. Width is observed, not trusted from an
  environment override. No terminal queries, cursor motion, animation, or probes
  execute inspected commands.

The palette uses portable ANSI amber, green, yellow, red, and dim. Prose wraps at
spaces when a useful width is known. Identities and paths remain contiguous for
copy/paste and use terminal soft wrapping. Borders are omitted for narrow windows,
long answers, or non-ASCII answer text whose exact cell width is uncertain. Text
is never truncated. Very small or unknown widths use an open layout. Width
estimation for prose is conservative; it is not a full Unicode grapheme engine.
The open layout uses a small ASCII `>` marker and whitespace to distinguish the
primary answer, including in ANSI-only terminals and fully plain output. It does
not require a console encoding change or a relaxation of capability detection.

PowerShell's existing bridge explicitly redirects the CLI response and decodes it
as strict UTF-8 before forwarding success/error lines. It therefore receives the
plain layout. It does not opt into ANSI based on the existence of a parent console,
which would contaminate pipelines, nor modify caller encoding or output settings.

## Untrusted text policy

Every external human-visible value goes through `safeText` at the final formatting
boundary. This includes command identities, candidates, paths, alias targets,
sources, shell metadata, versions, reasons, limitations, and error messages.

- C0/C1 controls and DEL become visible escapes. LF, CR, and tab use `\n`, `\r`,
  and `\t`; other single-byte control values use `\xNN`.
- Unicode format controls (category Cf, including bidi overrides/isolates, BOM,
  and zero-width joiners) and line/paragraph separators use `\uNNNN` escapes
  (`\UNNNNNNNN` for supplementary code points).
- Invalid UTF-8 bytes use `\xNN`. Ordinary Unicode, including combining marks,
  is preserved without normalization.
- Only renderer-owned newlines and SGR codes can affect terminal layout. ESC/OSC,
  CSI, clipboard/title sequences, carriage returns, and injected line breaks from
  data cannot reach the terminal as controls.

Literal backslashes are preserved, so an escaped control can resemble an existing
literal escape spelling. Human text is not a reversible machine encoding; consult
JSON for original identity. Confusable printable glyphs are not normalized.

Human sanitization never mutates the report. JSON uses its existing encoder and
escaping, preserves the original valid Unicode strings, and has no styling path.
Machine-error diagnostics remain unstyled `whichwhy:` lines on stderr; those human
diagnostics are sanitized too. `init powershell` is generated shell code, not a
human report, and intentionally bypasses this layer.

## Regression coverage

Capability tests inject terminal facts independently of the environment. Real
file/pipe tests prove redirection detection. Tests exercise all four result states,
later failures, alias selection and metadata, overlapping PATH diagnostics,
doctor states, deterministic output, long identities, narrow widths, Unicode
fallback, NO_COLOR, and hostile strings. JSON contract tests remain authoritative;
contamination tests additionally exercise every JSON route. Native shell suites
keep their evidence/passivity assertions and only update human heading/label
expectations.
