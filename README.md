# github-policy-cli

A Go CLI for declaring GitHub repository settings, reviewing differences with `repo-policy plan`, and applying changes with `repo-policy apply`.

## Supported scope

Targets **public and private repositories owned by the authenticated personal account on GitHub.com using GitHub Free**. Organization-owned repositories, repositories owned by other users where you are a collaborator, GHES, and Team / Enterprise-specific features are outside the supported scope. Archived repositories are also rejected.

| Setting | Public | Private |
| --- | --- | --- |
| Issues, Projects, merge methods, branch update suggestions and automatic deletion | Supported | Supported |
| Wiki enablement | Supported | Rejected, including false |
| PR creation policy: all / collaborators_only | Supported | Supported |
| Release immutability | Supported | Supported |
| Actions enablement, allowed actions, SHA pinning and workflow permissions | Supported | Supported |
| Actions external contributor approval policy | Supported | Rejected |
| Dependabot security updates | Supported | Supported |
| Main protection ruleset | Supported | Rejected on GitHub Free |
| Permanent Issue creation restrictions | Manual configuration | Manual configuration |

See the [official API investigation and permission matrix](docs/github-support.md). Private [wikis](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis) and [rulesets](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets) are unavailable on GitHub Free. The CLI applies the same Free capability limits even if the account has a paid plan.

## Personal defaults

**Without `--config`, the CLI automatically selects an embedded preset based on the repository's current visibility. It does not automatically read a local `policy.toml`.** This also works with `--all` when public and private repositories are mixed.

[Private preset](internal/policy/defaults/private.toml):

- Release immutability: on, applying to future releases
- Projects: off
- PR permissions: collaborators only
- Merge commits only; squash and rebase merges: off
- Always suggest updating pull request branches: on
- Automatically delete head branches: on
- Require actions to be pinned to a full-length commit SHA: on
- Dependabot security updates: off; alerts, version updates and other Advanced Security settings are unchanged

Collaborator-only Issue permissions are [available in the GitHub UI](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/disabling-issues), but no official REST read/write field was confirmed. **Configure Settings > General > Issues > Collaborators only manually.** The private preset's plan/apply output also identifies this manual step. Explicit `issues.creation_policy` configuration produces an explanatory error. Temporary interaction limits are not used as a substitute.

[Public preset](internal/policy/defaults/public.toml):

- Ruleset named `main`, active, targeting `~DEFAULT_BRANCH` to follow the default branch
- Restrict deletions, block force pushes and require signed commits
- Require a pull request before merging, zero required approvals, and Merge as the only allowed merge method
- Enable repository merge commits as well; preserve squash/rebase settings for other branches
- Actions: require approval for all external contributors

New rulesets have no bypass actors. If a `main` ruleset already exists, the CLI updates the specified settings while preserving additional rules, other PR parameters and bypass actors. Other rulesets are unchanged. Existing rules can make effective restrictions stricter than this preset. Multiple rulesets with the same name, a target other than branches, or unreadable current values cause preflight errors. Review existing exceptions on GitHub if you also want to remove them.

Public repositories do not inherit private settings; for example, their PR creation policy is unchanged. Applying the public approval policy while Actions is disabled produces an error. The CLI does not enable Actions automatically.

## Build and authentication

Requires Go 1.26.7 or later, as required by go-gh.

```sh
go build -o repo-policy ./cmd/repo-policy
export GH_TOKEN='YOUR_FINE_GRAINED_PAT'
./repo-policy plan YOUR_LOGIN/YOUR_REPO
```

For a fine-grained PAT, choose your personal account as the resource owner, select the intended repositories, and grant **Administration: Read and write / Metadata: Read** repository permissions. Read-only planning normally needs Administration: Read, but if an existing ruleset's bypass actors are omitted from the response, permissions that expose the full current ruleset are required.

Authentication uses `GH_TOKEN` or credentials saved by `gh auth login`, through go-gh. Do not store tokens in TOML. `GH_HOST` must be unset or `github.com`. `--all` paginates through your personally owned public/private repositories visible to the token. Private repositories excluded from the PAT's repository selection cannot be discovered.

## Explicit TOML configuration

`--config PATH` replaces the embedded defaults and manages **only the fields specified in that file**. A missing file is an error. Explicit `false` is distinct from omission. See [examples/policy.toml](examples/policy.toml) for a minimal policy. You can also copy and edit the embedded presets.

```sh
./repo-policy plan owner/repo --config policy.toml
./repo-policy apply owner/repo --config policy.toml
```

Examples of additional settings:

```toml
[repository]
has_projects = false
allow_update_branch = true

[releases]
immutable = true

[security]
dependabot_security_updates = false

[actions]
sha_pinning_required = true
# Public only. Also accepts first_time_contributors_new_to_github / first_time_contributors.
# fork_pr_approval_policy = "all_external_contributors"

# Public only. true manages the main protection profile described above.
# Deletion through false is not supported.
# [rulesets]
# main_protection = true
```

Other supported keys are `repository.has_issues`, `has_wiki`, `delete_branch_on_merge`, `allow_merge_commit`, `allow_squash_merge`, `allow_rebase_merge`; `pull_requests.creation_policy` (all / collaborators_only); `actions.enabled`, `allowed_actions` (all / local_only / selected); and `actions.workflow.default_workflow_permissions` (read / write), `can_approve_pull_request_reviews`.

Unknown keys, type mismatches and invalid enum values are errors. `selected` uses the existing GitHub allowlist; editing patterns is unsupported. At least one merge method must remain enabled. SHA pinning can be configured while Actions is disabled. Allowed-actions policies, workflow settings and external contributor approval require Actions to be enabled. The PR creation policy does not enable the PR feature itself.

## Commands and safety

```sh
./repo-policy plan owner/repo
./repo-policy apply owner/repo
./repo-policy plan --all
./repo-policy apply --all
```

`owner/repo` and `--all` are mutually exclusive. Planning is read-only and prints `No changes.` when the desired values already match. Successful planning exits with code 0 whether or not differences exist. Apply fetches fresh state, generates a new plan every time and calls only APIs with changes. There is no confirmation prompt.

All targets are checked for valid configuration, ownership, visibility, capabilities and readable current values before any update. Any preflight error prevents all writes. Execution errors identify the repository and HTTP status; independent operations and remaining repositories continue. Workflow/approval updates are skipped after a failed Actions enable, and ruleset updates are skipped after a failed merge-commit enable. The summary counts successful, failed and skipped API operations; failures produce a nonzero exit code. There is no rollback or protection against concurrent edits between reads and writes.

## Development

[Agent guide](AGENTS.md) / [Design](docs/design.md) / [API investigation](docs/github-support.md)

Use English for repository documentation, code comments, CLI messages, commit messages, issues, and pull request titles/descriptions. For dependency additions, updates or checks, apply the [14-day dependency cooldown skill](.agents/skills/dependency-cooldown/SKILL.md).

```sh
go test ./...
go vet ./...
go build ./cmd/repo-policy
```

Tests use fakes and httptest; they never change live GitHub repositories.
