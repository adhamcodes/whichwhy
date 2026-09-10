# Security policy

WhichWhy inspects command-resolution state, so safety and trust are part of the product contract.

## Core safety rules

- Generic inspection must not execute arbitrary candidate commands.
- The first public release must not silently modify shell configuration or machine state.
- Network access and telemetry are not part of the core design.
- Shell integrations should collect only the state required to explain command resolution.

## Reporting a vulnerability

While the repository is private, security issues can be reported directly to the maintainer. Before the repository becomes public, a private vulnerability-reporting path will be configured and documented here.

Please do not publish exploit details in a public issue before a private reporting path has been established.
