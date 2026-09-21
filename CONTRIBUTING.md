# Contributing to WhichWhy

WhichWhy welcomes focused contributions that preserve its evidence-first, passive command-resolution contract.

## Engineering principles

- Prefer correctness and clear evidence over clever heuristics.
- Keep the core resolver passive; do not execute arbitrary inspected commands.
- Keep platform- and shell-specific behavior explicit and testable.
- Add tests for every command-resolution rule or bug fix.
- Keep user-facing explanations understandable without requiring shell internals knowledge.
- Avoid unrelated features that turn WhichWhy into a general-purpose system toolbox.

## Development

Requirements:

- Go 1.27 or newer within the supported Go 1.27 release line.

Run the standard checks before committing:

```text
gofmt -w .
go vet ./...
go test ./...
go build ./cmd/whichwhy
```

## Commit messages

Commit history is part of the project's documentation. Use short, specific messages that describe engineering intent.

Examples:

```text
feat(resolver): preserve PATH candidate order
fix(powershell): honor alias precedence
test(resolver): cover duplicate executable names
docs: explain shell integration model
```

Use common prefixes such as `feat`, `fix`, `test`, `refactor`, `docs`, `ci`, and `chore` when they make the change easier to understand. Avoid vague messages such as `update`, `changes`, `fix stuff`, or `final`.

## Pull requests

A pull request should explain what behavior changes, why it changes, and how that behavior was verified. Keep unrelated changes in separate pull requests.
