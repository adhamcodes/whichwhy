# Architecture

This document records the initial engineering boundaries for WhichWhy. It is intentionally stricter than an implementation sketch: if the design changes, the reason should be documented.

## Product contract

For a supported shell and command, WhichWhy should answer four core questions:

1. What would the shell resolve this command to?
2. What evidence caused that candidate to win?
3. What other candidates were shadowed or skipped?
4. Does the observed state contain a likely configuration problem?

WhichWhy must distinguish between facts, supported inferences, and unknowns. It must not invent certainty when the current shell state cannot be observed.

## Architecture layers

### 1. Evidence collection

Collect facts that affect command resolution, such as environment search paths, file metadata, operating-system rules, and optional shell-local state.

The standalone binary can inspect process-visible state directly. Shell-local state such as aliases, functions, builtins, or command caches may require a small integration that runs inside the current shell and passes structured evidence to the core.

### 2. Resolution model

Turn collected evidence into an ordered set of candidates and a resolution result. Platform- and shell-specific rules must remain explicit rather than being hidden in presentation code.

The core model should be able to represent:

- the winning candidate;
- shadowed candidates;
- candidate type and source;
- ordering and precedence evidence;
- uncertainty when a rule cannot be evaluated safely.

### 3. Diagnostics

Diagnostics inspect a completed resolution result for suspicious but explainable states. Examples include stale search-path entries, duplicate entries, old installations shadowing newer ones, and supported toolchain mismatches.

Diagnostics must not silently turn heuristics into facts. Each warning should be traceable to evidence.

### 4. Presentation

Human output should explain the result in plain language. JSON output should expose stable structured data for scripts and integrations.

Presentation must not contain resolution logic.

## Safety boundaries

The generic resolver is passive. It must not execute arbitrary candidate programs merely to identify them.

If version probing is added for known tools, probes must be explicitly allow-listed and reviewed for safety. Unknown commands are never executed as part of inspection.

The first public release will not automatically edit shell configuration, PATH values, installations, aliases, or other machine state.

## Correctness strategy

For each supported shell, integration tests should create controlled resolution scenarios and compare WhichWhy's result with the shell's own resolution behavior. The shell is the oracle for that scenario.

Examples of scenario classes include:

- multiple executable candidates;
- aliases and shell functions;
- shell builtins or cmdlets;
- cached or hashed command locations;
- symlinks and shims;
- Windows executable-extension precedence;
- duplicate or missing search-path entries;
- Python and Node toolchain conflicts.

A disagreement between WhichWhy and a supported shell is a product bug, even when the output looks plausible.

## Initial support target

The V1 target is Windows, macOS, and Linux, with focused support for PowerShell, cmd.exe executable resolution, Bash, Zsh, and Fish.

Support should be claimed only after the corresponding oracle tests exist and pass.
