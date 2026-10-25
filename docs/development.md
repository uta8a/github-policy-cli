# Development guide

[Back to README](../README.md)

## Versioning

Release tags use **`vYYYY.0M.0D.MICRO`**, with the date in **Asia/Tokyo (JST)**. The first release each day ends in `.0`; subsequent releases increment the counter (`v2026.10.18.0`, `v2026.10.18.1`). A new day resets it to `.0`. Tags are the source of truth; there is no separate version file.

Use `python3 scripts/calver.py next` to suggest the next tag after fetching remote tags, and `python3 scripts/calver.py validate TAG` to check its format and calendar date. Released binaries report the injected tag with `repo-policy --version`; development builds report `dev`. See the [versioning and release procedure](versioning.md).

## CI and releases

[CI](../.github/workflows/ci.yml) runs build, gofmt verification, lint (`go vet`) and tests (`go test -race` plus the Python versioning tests) on pull requests and pushes to `main`. The [shared checks workflow](../.github/workflows/checks.yml) is also used before releases.

Pushing a `v*` tag starts the [release workflow](../.github/workflows/release.yml). It validates CalVer and verifies that the tagged commit belongs to `main`, runs all checks, and publishes Darwin ARM64 and Linux AMD64 archives with `checksums.txt`. See [release details](versioning.md#automated-release) and the [CI toolchain policy](ci.md). No repository settings are changed by these workflows.

## Contributing

[Agent guide](../AGENTS.md) / [Design](design.md) / [API investigation](github-support.md)

Use English for repository documentation, code comments, CLI messages, commit messages, issues, and pull request titles/descriptions. For dependency additions, updates or checks, apply the [14-day dependency cooldown skill](../.agents/skills/dependency-cooldown/SKILL.md).

```sh
go test ./...
go vet ./...
go build ./cmd/repo-policy
```

Tests use fakes and httptest; they never change live GitHub repositories.
