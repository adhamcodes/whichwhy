# Product Contract

The product name is WhichWhy; the CLI is `whichwhy`.
The tagline is: Which command gets picked — and why?

## Product identity

This project is an evidence-first command-resolution debugger.

Its core questions are:

1. What would execute?
2. Why would it execute?
3. What evidence supports that claim?
4. What alternatives lost, and why?
5. What remains uncertain?

A result may be uncertain. Guessing is not acceptable.

## Truth model

Shell-specific claims require shell-specific evidence. The shell itself is the oracle for supported shell behavior.

Process-visible external enumeration is useful evidence, but it is not automatically equivalent to what every shell will execute. Standalone results must identify the policy and limitations behind their claims instead of presenting a generic algorithm as universal shell truth.

## Safety

Inspection must be passive. The generic inspector must not execute arbitrary inspected commands, trigger module initialization, mutate caller shell/session state, or silently repair configuration.

If safe inspection cannot establish a result, report the limitation or uncertainty instead of causing side effects.

## Architecture

Keep these responsibilities separate:

- evidence collection: observe candidates, shell-local state, failures, skips, and environment facts;
- resolution: apply an explicit named policy to evidence;
- diagnostics: explain conflicts, shadowing, and limitations without upgrading heuristics into facts;
- presentation: render one completed result to human and JSON forms without reimplementing precedence.

## Support claims

A platform or shell is not considered supported merely because the code compiles there. Claimed support requires oracle-backed behavior tests for the semantics we advertise.

Current remediation work should strengthen correctness and evidence fidelity before expanding to more shells or features.

## Scope discipline

Do not compete by adding the most features. Prefer a narrow, trustworthy answer over broad but weak inference.

Version-manager intelligence, package origins, automatic fixes, and other higher-level features are secondary to trustworthy command-resolution evidence.
