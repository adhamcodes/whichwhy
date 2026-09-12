# WhichWhy

> Inspect command-resolution evidence, alternatives, and uncertainty.

WhichWhy is a local, open-source command-resolution debugger for developers. It is being built to answer a simple question that is often surprisingly difficult to diagnose:

**When I type a command, what will my shell actually run, and why did that candidate win?**

## Project status

WhichWhy is currently under active development and is **not ready for public use yet**. The repository will remain private until the first usable release satisfies the project's correctness and quality gates.

## What WhichWhy will explain

Given a command such as:

```text
whichwhy python
```

WhichWhy is designed to explain:

- what command the observed shell selects, or which candidate a named process policy selects;
- where that target comes from;
- why it wins over other candidates;
- which other candidates are shadowed;
- whether the resolution state looks suspicious;
- safe next steps when a likely configuration problem is detected.

Standalone inspection uses `process-path-order-v1`, a process-visible external
search policy. Its selected candidate may differ from what your shell executes.
The experimental PowerShell bridge reports observations from the current loaded
session. Both modes disclose their scope, claim strength, and limitations; see
[Resolution claims](docs/resolution-policy.md) for the exact policy and JSON fields.

## Design principles

### Correctness over cleverness

If WhichWhy cannot determine something safely and accurately, it should say so instead of guessing. For supported shells, behavior will be tested against the shell's own resolution results.

### Explain, do not mutate

The first release will inspect, explain, warn, and suggest. It will not silently edit PATH entries, remove installations, rewrite shell configuration, or perform other destructive fixes.

### Local by default

WhichWhy is designed to work without an account, remote service, AI model, or network connection. No telemetry is planned for the core tool.

### One job, done well

WhichWhy is not a general system cleaner or developer toolbox. Features belong only when they help answer:

> What will run, why will it run, what else could have run, and does the result look wrong?

## Planned command shape

```text
whichwhy <command>           Investigate a command
whichwhy path                Inspect command-search paths
whichwhy doctor              Check the WhichWhy installation
whichwhy init <shell>        Configure optional shell-aware integration
whichwhy <command> --json    Emit machine-readable output
whichwhy --help              Show usage
whichwhy --version           Show the installed version
```

The final command surface may change during development if testing shows a simpler or more accurate design.

## Development

WhichWhy currently targets Go 1.27.

```text
go test ./...
go vet ./...
go build ./cmd/whichwhy
```

## License

MIT
