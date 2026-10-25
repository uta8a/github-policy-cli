# CI and release tooling

## Workflows

- `ci.yml`: PRs and pushes to main; calls the reusable check matrix.
- `checks.yml`: independent build, fmt, lint and test jobs on Ubuntu 24.04. Lint uses the Go toolchain's `go vet`. Tests include Go's race detector and Python CalVer tests. Module files are read-only during Go commands.
- `release.yml`: validates the triggering CalVer tag and main ancestry, runs the shared checks, cross-compiles Darwin ARM64 and Linux AMD64, then publishes archives and SHA-256 checksums. See [versioning](versioning.md).

Checks and release builds use the triggering commit SHA, with checkout credentials removed after checkout. Setup-go's cache is disabled to avoid introducing additional cache state or save operations in untrusted PRs. GitHub-hosted Ubuntu supplies Python, Git, curl, tar and the GitHub CLI; these platform tools follow the runner image rather than project dependency manifests.

## Tag permissions

The release workflow's ancestry checks protect against mistakes in trusted workflow code. They cannot constrain an attacker who can tag a modified workflow. Before enabling tag-based publishing, apply the public default's owner-only release-tags ruleset from a trusted checkout. See the [release trust boundary](versioning.md#release-trust-boundary). CI does not modify these settings automatically. This policy controls matching tag operations; it does not independently restrict every way a repository-write workflow could publish a release. Review workflow-write and publication permissions separately, and keep the owner account trusted.

## Cooldown review

Reviewed on 2026-10-25 UTC. No application dependency versions were changed. The JavaScript Actions are used as their bundled, SHA-pinned distributions; CI does not install their development dependencies.

| Tool | Selected version / SHA | Official publication | Decision |
| --- | --- | --- | --- |
| actions/checkout | v7.0.1 / 3d3c42e5aac5ba805825da76410c181273ba90b1 | 2026-07-20 15:10:05 UTC | At least 14 days old |
| actions/setup-go | v7.0.0 / b7ad1dad31e06c5925ef5d2fc7ad053ef454303e | 2026-07-16 02:50:50 UTC | At least 14 days old |
| Go | 1.27.3 | 2026-10-19 | Explicit human-authorized exception |

Sources: [checkout release](https://github.com/actions/checkout/releases/tag/v7.0.1), [setup-go release](https://github.com/actions/setup-go/releases/tag/v7.0.0), [Go release history](https://go.dev/doc/devel/release). Full Actions commit pins were resolved from their official tag references. Both selected Actions run on Node 24, supported by current GitHub-hosted runners.

### Explicit exception: Go 1.27.3

On 2026-10-25, the human user explicitly authorized adopting Go 1.27.3 before the 14-day cooldown ended. This exception applies only to Go 1.27.3 and includes its security fixes; it does not exempt other tools or future Go versions. Only explicit human instructions can authorize cooldown exceptions. Existing dependency-cooldown instructions remain in force.

### Linter compatibility

The initially considered [Staticcheck 2026.2.1](https://github.com/dominikh/go-tools/releases/tag/2026.2.1), published 2026-08-21 20:43:45 UTC, passed the age requirement but failed local verification because it cannot decode Go 1.27.2+ export data. [Staticcheck 2026.2.2](https://github.com/dominikh/go-tools/releases/tag/2026.2.2) adds that support but was published 2026-10-12 14:05:03 UTC, so it is not eligible until 2026-10-26 14:05:03 UTC. It is not installed or used by CI. The lint job uses the pinned Go toolchain's `go vet` instead; no implicit cooldown exception is taken for Staticcheck.
