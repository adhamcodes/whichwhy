# Security policy

WhichWhy inspects command-resolution state, so safety and trust are part of the product contract.

## Core safety rules

- Generic inspection must not execute arbitrary candidate commands.
- The first public release must not silently modify shell configuration or machine state.
- Network access and telemetry are not part of the core design.
- Shell integrations should collect only the state required to explain command resolution.

## Reporting a vulnerability

For the public repository, report security vulnerabilities privately through GitHub's
repository security advisory flow using **Security → Advisories → Report a vulnerability**.

Do not publish exploit details in a public issue or discussion.

Private vulnerability reporting must be enabled before the public V1 release is
published. If the private-reporting control is not available, treat that as a release
blocker rather than asking reporters to disclose details publicly.
