# WhichWhy

> Know exactly which command will run — and why.

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

- what command or executable will win resolution;
- where that target comes from;
- why it wins over other candidates;
- which other candidates are shadowed;
- whether the resolution state looks suspicious;
- safe next steps when a likely configuration problem is detected.

The first release is intended to cover the common command-resolution rules used by Windows, macOS, and Linux shells, with shell-aware integration where live shell state is required for an accurate answer.

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

## License

WhichWhy is planned to be released under the MIT License before the repository becomes public.
