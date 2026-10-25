# Versioning

Release tags are the source of truth. Use `vYYYY.MM.DD.N`, corresponding to CalVer `vYYYY.0M.0D.MICRO`:

- `YYYY`: four-digit Gregorian year.
- `MM` and `DD`: zero-padded month and day.
- `N`: a nonnegative integer without leading zeroes, starting at 0 each day.
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

Builds without this injection report `dev`. Version display does not require a GitHub token or API access. Future tag-triggered release automation must validate the tag and inject it into each target binary.

These four-component CalVer tags are application release identifiers, not [Go module semantic versions](https://go.dev/ref/mod#versions). Install released binaries or build from a checkout; do not rely on `go install ...@v2026.10.18.0` to resolve a Go module version.

## Verification

```sh
python3 -m unittest discover -s scripts -p 'test_*.py'
```

Tests cover calendar validity, leap days, counter ordering, daily resets, backdating and the JST midnight boundary. The CLI test verifies that `--version` does not construct an authenticated API client.
