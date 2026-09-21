Which command gets picked — and why?

- Explain command selection and alternatives within an explicit evidence scope.
- Diagnose original PATH entries, duplicates, missing directories and observation failures.
- Use `doctor` to compare the running binary with process-policy discovery.
- Optionally observe Windows PowerShell 5.1/7 loaded sessions through the experimental bridge.
- Retain incomplete observations, uncertain precedence and limitations in human and JSON reports.
- Use the frozen JSON interface: command/doctor schema 2 and PATH schema 1.
- Inspect locally and passively, without executing discovered commands or auto-importing modules.

Assets: Windows amd64, Linux amd64, macOS amd64 and arm64. Native CI verifies
Windows, Linux and macOS process policies and Windows PowerShell 5.1/7. The
macOS amd64 asset is cross-compiled on the macOS arm64 packaging runner; it has
no native amd64 runtime gate. Each packaging log records whether `--version`
ran natively. Standalone mode does not claim Bash, Zsh, Fish or general cmd.exe
resolution parity, and discovery does not prove successful invocation.

Download the matching archive and SHA256SUMS.txt. Follow the README checksum
instructions before extracting. Archives include the binary, MIT license and
usage reference. There is no installer, automatic PATH editing, self-updater,
package-manager distribution, signing or notarization in this pipeline.

This draft requires artifact smoke tests and Mission Control approval before
publication. See docs/release.md in the tagged source for the release checklist.
