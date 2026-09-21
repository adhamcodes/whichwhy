# Release procedure

Which command gets picked — and why?

GitHub Releases are the canonical V1 binary distribution surface. This procedure
implements [issue #36](https://github.com/adhamcodes/whichwhy/issues/36) and its
[RC-first sequencing decision](https://github.com/adhamcodes/whichwhy/issues/36#issuecomment-5757102706).
Tag creation, visibility changes and publication are separate Mission Control
actions. Implementing or testing this pipeline does not authorize those actions.

## Architecture and versions

`.github/workflows/release.yml` runs on `v*` tag pushes. `scripts/release` uses the
Go standard library for strict v-prefixed SemVer validation, clean source checks,
builds, archives and checksums. Numeric core/prerelease leading zeroes, whitespace,
empty identifiers and malformed tags are rejected. SemVer build metadata is
accepted and retained in both the embedded version and archive name. Prerelease
classification examines the part before build metadata.

The source default remains `dev`. Release builds use `-trimpath`,
`-buildvcs=true`, `-ldflags=-X main.version=<exact-tag>` and `CGO_ENABLED=0`.
The build tool overrides inherited Go target/build flags and uses baseline CPU
levels. Archive member order, timestamps, permissions and ownership are fixed.
This is reproducibility-minded packaging, not a promise of bit-identical binaries
across Go patch releases or hosts. Record the actual Go version in workflow logs.

Each archive contains the binary, `LICENSE` and a concise `README.md` from
`docs/release-usage.md`. Names omit only the leading `v`:

| Asset | Packaging runner | Runtime version check |
| --- | --- | --- |
| `whichwhy_1.0.0_windows_amd64.zip` | Windows x64 | Native |
| `whichwhy_1.0.0_linux_amd64.tar.gz` | Ubuntu x64 | Native |
| `whichwhy_1.0.0_darwin_arm64.tar.gz` | `macos-15` arm64 | Native `--version` smoke test |
| `whichwhy_1.0.0_darwin_amd64.tar.gz` | `macos-15-intel` x64 | Native `--version` smoke test |

RC names use `1.0.0-rc.1`. The tool compares the actual host OS/architecture with
the target before executing the built CLI's `--version`; logs explicitly report
cross-compilation. Runner labels may evolve: review their actual architecture
and version-check logs before making runtime-support claims.
The Intel packaging gate checks the packaged build's version; it does not add a
full Intel correctness/oracle job or expand shell-parity claims. Standalone
correctness remains governed by the [oracle contract](oracle-contract.md).

## Fail-closed staging

1. Check out the event SHA, validate the tag, and require HEAD and the peeled tag
   to equal that SHA with a clean worktree. Explicitly fetch `refs/heads/main`
   from `$GITHUB_SERVER_URL/$GITHUB_REPOSITORY.git` into
   `refs/remotes/origin/main`, then require the release commit to be an ancestor
   of that fetched commit with `git merge-base --is-ancestor`. An exact main
   commit or an older ancestor passes; a divergent/unmerged commit, missing main
   or failed Git observation fails. A local branch named `main` is never used.
2. Call the ordinary CI workflow **from the same tagged commit**. This reruns
   formatting, uncached `go test ./... -count=1`, vet and CLI builds on Windows,
   Linux and macOS. `WHICHWHY_REQUIRE_ORACLES=1` is retained. Both version-checked
   Windows PowerShell 5.1/7 suites compose passive, transport, literal, isolation
   and response coverage. An earlier branch CI result cannot satisfy this gate.
3. Only after that gate passes, build in fresh checkouts of the event SHA,
   refreshing remote main and rechecking clean source/tag identity and ancestry.
   Upload one archive per target using
   immutable Actions artifacts scoped to this workflow run.
4. Collect all four archives. Reject missing, extra, empty or non-regular entries
   and wrong version/target filenames. Generate `SHA256SUMS.txt` in sorted filename
   order with lowercase SHA-256, two spaces, filename and LF. There is exactly one
   entry per archive; the manifest does not hash itself.
5. On Ubuntu, refresh remote main again before staging.
   `scripts/create-draft-release.sh` rechecks source identity and main ancestry, the
   remote tag's peeled commit, complete assets and the manifest. It requires a
   successful API read and refuses any existing release for that tag, including
   an existing draft. It creates a **draft** with `--verify-tag`, so the CLI cannot
   create an absent tag. RC tags also receive the prerelease flag.
6. Download all staged assets, require exactly four archives plus the manifest,
   compare the manifest and verify all downloaded hashes. Any failure fails the
   run. There is no publish operation. Only this final job has `contents: write`;
   earlier jobs have `contents: read`. All action dependencies are SHA-pinned.

Main can advance after tagging: equality with the current main tip is not required.
The workflow overwrites the remote-tracking ref with an explicit authenticated
fetch from its own GitHub repository before each validation/build/staging job's
source checks; fetch failure stops the job even if a stale local ref exists.
Credentials are passed only to that fetch, masked in logs and not persisted in Git
configuration. The tag remains an immutable input and is not changed by the fetch.
Reachability enforces main lineage, not the human review process itself: protecting
main and ensuring changes there are reviewed remain repository governance duties.

GitHub asset uploads are not transactional: an API/upload failure may leave an
incomplete **draft**. A failed run is never approval to publish. Do not modify a
published release or move/reuse a published tag to repair it. If only transport
failed, Mission Control may inspect and remove the failed unpublished draft
(leaving its tag untouched) and rerun **all jobs** against the same immutable
commit. Existing releases are otherwise refused. If code needs a correction,
review the fix and create a new RC tag, e.g. `v1.0.0-rc.2`. A successful rerun also
requires an intentional draft review/removal first. Do not race human publication
with staging. No workflow should run while a human is publishing that tag.

## Local validation without tags or releases

Run from the repository root with Go and Git installed. The Go helper is portable;
the draft script assumes Ubuntu Bash plus `gh`, `jq`, GNU `sha256sum`, `find`,
`diff`, `cmp`, `mktemp` and standard file utilities. Workflow fetch steps use Bash,
`base64` and `tr`, including Git Bash on Windows; packaging uses the portable helper.

```text
go test ./scripts/release -count=1
go run ./scripts/release validate v1.0.0-rc.1
go run ./scripts/release build v1.0.0-rc.1 windows amd64 dist/rc-test
go run ./scripts/release build v1.0.0-rc.1 linux amd64 dist/rc-test
go run ./scripts/release build v1.0.0-rc.1 darwin amd64 dist/rc-test
go run ./scripts/release build v1.0.0-rc.1 darwin arm64 dist/rc-test
go run ./scripts/release checksums v1.0.0-rc.1 dist/rc-test
```

Use a new empty output directory for each run; existing archives/manifests are
never overwritten. Local `build` permits a dirty tree for packaging development;
it is not release evidence. The workflow always calls `source TAG COMMIT` before
building/staging, after explicitly fetching repository main into the remote ref.
Standalone `source` checks use that fetched snapshot and do not fetch or prove its
freshness themselves; a locally invented remote-tracking ref is not release evidence.
No local example creates a tag or contacts the release API.
Regression tests unpack ZIP/tar.gz, check contents and executable modes, test
stable/prerelease versions from extracted native binaries, reject malformed tags
and incomplete asset sets, and protect archive determinism and overwrite refusal.
Disposable real Git commit graphs test exact/older main ancestry, divergent work,
spoofed local main and missing/unobservable history without creating tags.

Also run formatting, all uncached tests with required native oracles, vet, build,
both Windows shell gates and `git diff --check`. Use actionlint to validate both
workflows and `bash -n scripts/create-draft-release.sh` for shell syntax. Local
Windows cross-builds do not establish Linux/macOS runtime correctness. Only a real
tag run proves Actions orchestration, token permissions and draft upload/download
behavior; local tooling tests cannot substitute for it.

## RC-first sequence and final manual checklist

- [ ] Mission Control reviews and merges the packaging code; no unreviewed/dirty source.
- [ ] Create immutable `v1.0.0-rc.1` from that reviewed commit and let the workflow stage its draft.
- [ ] Require all tag-run gates to pass; inspect platform, oracle and version-check logs.
- [ ] Download all four archives and `SHA256SUMS.txt` from the draft, verify every
  checksum, and inspect names, binary, LICENSE and usage material. See the
  [README](../README.md#github-release-binaries) for user checksum commands.
- [ ] Extract the actual Windows asset to a fresh directory, including a space in
  its path, and invoke it by explicit path from outside the source checkout. Use a
  fresh `-NoProfile` PowerShell session and a controlled session-only PATH. Avoid
  stale WhichWhy binaries/functions and profile-based configuration; do not edit
  the user's persistent PATH. Record the full executable path and downloaded hash.
- [ ] Check `--version` equals `whichwhy v1.0.0-rc.1`, and `--help` renders correctly.
- [ ] Inspect a normal known command such as `where.exe` without executing it.
- [ ] Run `path` and `doctor`; review the evidence and exit codes. Doctor may need
  attention if the chosen directory is absent from the process PATH.
- [ ] Check inspection, `path` and `doctor` JSON with `ConvertFrom-Json`, their scope,
  version, exit and stream behavior against the frozen JSON contract.
- [ ] Load the bridge from the **downloaded binary** with `init powershell |
  Invoke-Expression` in separate fresh 5.1 and 7 sessions. Inspect `where`,
  `inspect path`, and JSON. Confirm loaded-session scope and no profile changes.
- [ ] Complete the final RC audit. Require native `--version` records from both
  macOS packaging jobs; record any further manual architecture smoke-test evidence
  separately. The configured Intel GitHub-hosted job is unproven until this tag run.
- [ ] Approve RC results before creating immutable `v1.0.0`. Its workflow creates
  another **draft**, and checksums/version must be checked again for those final
  assets; the RC binary is not the final-version binary.
- [ ] Reconcile README, LICENSE, SECURITY, CONTRIBUTING, changelog and final release
  notes. Configure and document private vulnerability reporting before going
  public; update the current private-development wording. Add release dates only
  when confirmed. Review/remove draft-only approval prose from final release notes.
- [ ] Update GitHub repository description to **Which command gets picked — and why?**
- [ ] Make the public-visibility decision, confirm repository/tag protection and
  release permissions, and obtain final Mission Control approval.
- [ ] Deliberately publish the final `v1.0.0` draft only after that approval.

No package-manager publishing, installer, self-updater, automatic PATH editing,
signing or notarization is implemented. The experimental PowerShell bridge and
standalone named process policy keep their existing truth and safety boundaries.
