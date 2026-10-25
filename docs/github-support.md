# GitHub Free support investigation

Checked 2026-10-25. Sources are GitHub.com documentation and GitHub's official OpenAPI, not GHES documentation. The CLI applies a fixed Free capability model; it does not infer permission from an account's paid subscription.

## Setting matrix

All supported rows require authenticated ownership (`owner.type=User` and owner login matching GET /user). Public and private refer to GitHub Free personal repositories.

| TOML field | Public | Private | Read / write |
| --- | --- | --- | --- |
| repository.has_projects / allow_update_branch | Yes | Yes | Repository GET / PATCH |
| releases.immutable | Yes | Yes | immutable-releases GET / PUT / DELETE |
| security.dependabot_security_updates | Yes | Yes | automated-security-fixes GET / PUT / DELETE |
| actions.sha_pinning_required | Yes | Yes | Actions permissions GET / PUT |
| actions.fork_pr_approval_policy | Yes | No (public-fork policy) | fork-pr-contributor-approval GET / PUT |
| rulesets.release_tag_protection | Yes | No (Free private restriction) | Rulesets list / detail / POST / PUT |
| rulesets.main_protection | Yes | No (Free private restriction) | Rulesets list / detail / POST / PUT |
| repository.has_issues | Yes | Yes | Repository GET / PATCH |
| repository.has_wiki | Yes | **No, including false** | Repository GET / PATCH |
| repository.delete_branch_on_merge | Yes | Yes | Repository GET / PATCH |
| repository.allow_squash_merge | Yes | Yes | Repository GET / PATCH |
| repository.allow_merge_commit | Yes | Yes | Repository GET / PATCH |
| repository.allow_rebase_merge | Yes | Yes | Repository GET / PATCH |
| pull_requests.creation_policy | all / collaborators_only | all / collaborators_only | Repository GET / PATCH pull_request_creation_policy |
| actions.enabled | Yes | Yes | Actions permissions GET / PUT |
| actions.allowed_actions | all / local_only / selected | Same | Actions permissions GET / PUT |
| actions.workflow.default_workflow_permissions | read / write | Same | Workflow permissions GET / PUT |
| actions.workflow.can_approve_pull_request_reviews | Yes | Yes | Workflow permissions GET / PUT |

[GitHub plans](https://docs.github.com/en/get-started/learning-about-github/githubs-plans) establishes Free personal public/private support. [About wikis](https://docs.github.com/en/communities/documenting-your-project-with-wikis/about-wikis) explicitly excludes private Free wikis. Reject any private has_wiki declaration, even a no-op false, rather than implying the feature is available.

[Repository REST API](https://docs.github.com/en/rest/repos/repos#get-a-repository) documents repository reads and [updates](https://docs.github.com/en/rest/repos/repos#update-a-repository). These basic repository and merge toggles carry no additional paid-plan restriction. At least one merge method must remain enabled.

[Disabling pull requests](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/disabling-pull-requests) explicitly explains collaborators in personal repositories and has no visibility/paid-plan qualification. The [official OpenAPI](https://github.com/github/rest-api-description/blob/main/descriptions/api.github.com/api.github.com.json) defines pull_request_creation_policy (all / collaborators_only) in both full-repository GET response and PATCH input. Thus creation_policy is supported; a missing response field fails closed. It does not enable disabled pull requests.

[Repository Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository) describes personal default workflow permissions. [Actions billing](https://docs.github.com/en/actions/concepts/billing-and-usage) confirms private Free usage has quotas; changing settings does not grant unlimited runtime. No extra plan restriction is documented for these permissions endpoints.

## Endpoints and fine-grained PAT

| Endpoint | Permission |
| --- | --- |
| GET /user | Authenticated personal identity; no additional repository permission |
| GET /user/repos?affiliation=owner&visibility=all&per_page=100&page=N | Metadata: read; token-selected repositories only |
| GET /repos/{owner}/{repo} | Metadata: read |
| PATCH /repos/{owner}/{repo} | Administration: write |
| GET /repos/{owner}/{repo}/actions/permissions | Administration: read |
| PUT /repos/{owner}/{repo}/actions/permissions | Administration: write |
| GET /repos/{owner}/{repo}/actions/permissions/workflow | Administration: read |
| PUT /repos/{owner}/{repo}/actions/permissions/workflow | Administration: write |

Sources: [Repository endpoints](https://docs.github.com/en/rest/repos/repos), [Actions permissions endpoints](https://docs.github.com/en/rest/actions/permissions), [authenticated user](https://docs.github.com/en/rest/users/users#get-the-authenticated-user). For apply, use Administration read/write and Metadata read on every intended repository. No Actions or Workflows write permission is needed for these administration endpoints.

Repository PATCH is partial. Actions permissions PUT requires enabled; allowed_actions and sha_pinning_required are optional. Workflow PUT has optional default_workflow_permissions and can_approve_pull_request_reviews. Documentation does not explicitly promise complete replacement semantics or preservation for every omitted PUT property: preserve the known returned companions, do not manufacture defaults. Separate selected-action allowlists are not updated. Never send fields that are outside the documented request schema.

## Exclusions

Permanent Issue collaborator-only permissions are documented in the [GitHub UI](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/disabling-issues), including personal repositories. However, neither the full-repository GET schema nor the PATCH schema in the official OpenAPI contains issue_creation_policy. No supported REST read/write contract was confirmed. [Interaction limits](https://docs.github.com/en/communities/moderating-comments-and-conversations/limiting-interactions-in-your-repository) are temporary moderation, not a permanent Issue policy. Explicit issues.creation_policy fails validation with a manual-configuration explanation; both presets omit it and print the manual step. No automatic refresh is implemented.

Organization repositories, other users' repositories, GHES, paid-only features (including private Wiki and private protected-branch features), private rulesets, repository creation, allowlist pattern management and visibility changes are outside this implementation. Archived repositories fail preflight. API authentication, permission, validation and rate-limit failures remain errors. Ordinary 404s never become a no-op; the documented disabled-feature status exception and its Administration-read verification are described below.

## Personal defaults: additional investigation

The [release immutability guide](https://docs.github.com/en/code-security/how-tos/secure-your-supply-chain/establish-provenance-and-integrity/prevent-release-changes) documents repository-level enablement, applying to future releases. [Supply-chain feature availability](https://docs.github.com/en/code-security/concepts/supply-chain-security/supply-chain-security) describes immutable releases and Dependabot security updates without a paid-plan or private-repository restriction. Only security updates are disabled; alerts, version updates and other security settings are untouched. Enabling security updates requires dependency graph and Dependabot alerts already enabled; the CLI does not enable prerequisites implicitly.

The [repository REST endpoints](https://docs.github.com/en/rest/repos/repos) and OpenAPI define has_projects, allow_update_branch, immutable-releases and automated-security-fixes. Both feature-check endpoints document 404 as disabled. Because GitHub can also hide resources from insufficiently privileged tokens with 404, the client verifies repository identity first and Administration-read access separately before interpreting this status. Ordinary endpoint 404s never become disabled or no-change values. Feature toggles use empty-body PUT/DELETE. The immutable response's enforced_by_owner is checked before disabling.

[Actions permissions](https://docs.github.com/en/rest/actions/permissions) documents sha_pinning_required on repository permissions and the fork-pr-contributor-approval endpoint with all_external_contributors. [Repository Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository) distinguishes public-fork contributor approval from private-fork workflow policies. This CLI supports contributor approval only for Public targets. The SHA requirement does not pin existing workflows automatically; such workflows may need source edits to run.

[Ruleset availability](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/about-rulesets) limits Free rulesets to public repositories. [Rules REST API](https://docs.github.com/en/rest/repos/rules) and official OpenAPI establish repository branch rules, ~DEFAULT_BRANCH conditions, deletion / non_fast_forward / required_signatures / pull_request and the PR parameters required_approving_review_count=0 and allowed_merge_methods=[merge]. The public preset additionally enables repository merge commits so the rule is usable. It preserves existing unrelated rules and bypass actors; therefore pre-existing bypass actors may still bypass these protections. New main rulesets have no bypass actors.

| Additional endpoint | Fine-grained PAT permission |
| --- | --- |
| GET /repos/{owner}/{repo}/immutable-releases | Administration: read |
| PUT / DELETE /repos/{owner}/{repo}/immutable-releases | Administration: write |
| GET /repos/{owner}/{repo}/automated-security-fixes | Administration: read |
| PUT / DELETE /repos/{owner}/{repo}/automated-security-fixes | Administration: write |
| GET /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval | Administration: read |
| PUT /repos/{owner}/{repo}/actions/permissions/fork-pr-contributor-approval | Administration: write |
| GET /repos/{owner}/{repo}/rulesets and /rulesets/{id} | Metadata: read (bypass_actors only returned with write access to ruleset) |
| POST /repos/{owner}/{repo}/rulesets | Administration: write |
| PUT /repos/{owner}/{repo}/rulesets/{id} | Administration: write |

The Public preset includes all Private preset settings, with public-only ruleset and contributor-approval additions. Missing --config selects these presets; explicit TOML never inherits them. No paid-only private rule is silently attempted, and the unautomated Issue-permissions setting is visible in both documentation and both default plan/apply outputs.

## Owner-only release tags

[Available rules](https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/available-rules-for-rulesets) documents tag creation/update/deletion restrictions and Free availability for public repositories. [Rules REST API](https://docs.github.com/en/rest/repos/rules#create-a-repository-ruleset) documents target=tag, ref_name patterns, the creation/update/deletion rule types, and bypass actors of type User with a numeric actor_id and always mode. The official OpenAPI confirms these request fields. OrganizationAdmin is explicitly inapplicable to personal repositories; undocumented numeric RepositoryRole mappings are not used.

The release-tags profile restricts refs/tags/v* to owner.id from the already verified repository GET response. That lookup needs Metadata read; ruleset listing/details also use Metadata read, and creation/update requires Administration write. A complete current bypass list must be readable before updating. Unlike main protection, this profile explicitly manages the entire bypass list, replacing any collaborator/app exceptions with the owner alone. Extra rules are retained. Private targets fail preflight on GitHub Free.

GitHub's [workflow trigger model](https://docs.github.com/en/actions/concepts/workflows-and-actions/workflows#workflow-triggers) runs the workflow version at the event's associated ref/SHA. Therefore the release workflow's main-ancestry check is not a substitute for server-enforced tag permissions. Arbitrary-tag pushers can otherwise alter the workflow itself. The default protects against non-owner tag pushers after apply; the personal owner and their credentials remain trusted. No live repository protection is changed merely by checking in these defaults.

## Default workflow permissions in personal presets

Both personal presets set `actions.workflow.default_workflow_permissions = "read"`, corresponding to **Read repository contents and packages permissions** in the [repository Actions settings](https://docs.github.com/en/repositories/managing-your-repositorys-settings-and-features/enabling-features-for-your-repository/managing-github-actions-settings-for-a-repository). This setting is available for owned public and private personal repositories on GitHub Free through the existing workflow permissions GET/PUT endpoints listed above (Administration read/write). The companion `can_approve_pull_request_reviews` value is preserved. This selects the default `GITHUB_TOKEN` permissions; workflows can explicitly request additional permissions. Actions must already be enabled, and unreadable workflow permission values fail preflight before any writes. Explicit configuration still replaces the presets.
