# Versioning

Release tags are the source of truth. Use [CalVer](https://calver.org/) notation `vYYYY.0M.0D.MICRO`:

- `YYYY`: four-digit Gregorian year.
- `0M` and `0D`: zero-padded month and day.
- `MICRO`: a nonnegative integer without leading zeroes, starting at 0 each day.
- The date is the release date in **Asia/Tokyo (JST, UTC+09:00)**.

For example, `v2026.10.18.0` is the first release on October 18, 2026 JST, followed by `v2026.10.18.1` and `v2026.10.18.2`. The next day's first release is `v2026.10.19.0`. Compare counters numerically: `.10` follows `.9`.

## Suggest and validate a tag

The helper requires only Python's standard library and Git. It does not create or push tags, publish releases, or change repository files.

```sh
git fetch origin --tags
python3 scripts/calver.py next
python3 scripts/calver.py validate v2026.10.18.0
```

`next` reads local Git tags and selects one above the largest counter for today's JST date, or 0 when none exists. Fetch tags first to include releases published by others. An explicit `--date YYYY-MM-DD` is available for reproducible checks; normal releases use the current JST date. The helper refuses to backdate a release before a known later-dated release. It ignores unrelated tag formats when suggesting a version. `validate` checks syntax and actual calendar validity; it intentionally does not require the date to be today, so existing tags can be rebuilt.

Before publishing, use a tested commit on `main`, refresh tags, and confirm that the suggested version is still unused. Create an annotated tag on that commit and push that exact tag. Never overwrite or force-push published release tags. If publication fails, retry the same tag without moving it; use a new counter for changed source. Recalculate if another release claims the suggested tag or the JST date changes before tagging.

## Binary version

Inject the validated tag at build time, rather than maintaining a duplicate version in source:

```sh
version=v2026.10.18.0
python3 scripts/calver.py validate "$version"
CGO_ENABLED=0 go build -trimpath -ldflags "-X main.version=$version" -o repo-policy ./cmd/repo-policy
./repo-policy --version
# repo-policy version v2026.10.18.0
```

Builds without this injection report `dev`. Version display does not require a GitHub token or API access. The tag-triggered release workflow validates the tag and injects it into each target binary.

These four-component CalVer tags are application release identifiers, not [Go module semantic versions](https://go.dev/ref/mod#versions). Install released binaries or build from a checkout; do not rely on `go install ...@v2026.10.18.0` to resolve a Go module version.

## Verification

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
```

Tests cover calendar validity, leap days, counter ordering, daily resets, backdating and the JST midnight boundary. The CLI test verifies that `--version` does not construct an authenticated API client.

## Release trust boundary

Before relying on tag-triggered releases in a public repository, apply the public preset's `rulesets.release_tag_protection = true` from a trusted checkout. It creates an active `release-tags` ruleset for `refs/tags/v*`, limiting creation, updates and deletion to the personal owner. Existing bypass actors in that profile are replaced with the owner alone. Review the plan before applying, and keep the owner's credentials trusted. Private GitHub Free repositories cannot use this ruleset.

A tag can point at an unmerged PR commit, and GitHub runs the workflow from that tag. An attacker allowed to push such tags can remove the YAML's main-ancestry check, so the check alone is not a security boundary. Tag rules must be enforced by GitHub outside the workflow. Applying the default is a separate owner action; the release workflow does not bootstrap this protection.

## Automated release

`.github/workflows/release.yml` runs on `v*` tag pushes. A tag push does not carry a target branch, so the workflow checks out the triggering commit with full history and requires it to be an ancestor of `origin/main`. It validates the real CalVer date and counter format, then runs the same build, formatting, lint and test matrix used for PRs. The tag date is not required to equal the rerun date, so an existing tag can be retried later.

After validation, the release job builds these archives with `CGO_ENABLED=0`, an injected version, and stripped binaries:

- `repo-policy_TAG_darwin_arm64.tar.gz`: Apple Silicon macOS.
- `repo-policy_TAG_linux_amd64.tar.gz`: x86-64 Linux.
- `checksums.txt`: SHA-256 hashes of both archives.

Each archive contains `repo-policy`, LICENSE and README.md in a versioned directory. The binary version includes the full tag, for example `v2026.10.18.0`.

The release is first created as a draft, all assets are uploaded, and only then is it published. Existing release assets are never overwritten. If publication leaves a draft behind, inspect and remove that failed draft on GitHub before rerunning the workflow for the unchanged tag. A published release must use a new tag for any changed source. Never move the original tag. No workflow creates tags automatically.

Only the release job has `contents: write`; validation and CI have `contents: read`. Actions are pinned to complete commit SHAs. Release runs for the same tag are serialized and not canceled mid-publication. No scheduled or PR-triggered publication is configured.

To check packaging locally without publishing:

```sh
bash scripts/package-release.sh v2026.10.18.0 /tmp/repo-policy-release
```

The output directory must not already exist. Verify `checksums.txt` with `sha256sum -c checksums.txt` on Linux or `shasum -a 256 -c checksums.txt` on macOS from inside the output directory.
