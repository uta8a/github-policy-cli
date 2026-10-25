# GitHub Free support investigation

Checked 2026-10-25. Sources are GitHub.com documentation and GitHub's official OpenAPI, not GHES documentation. The CLI applies a fixed Free capability model; it does not infer permission from an account's paid subscription.

## Setting matrix

All supported rows require authenticated ownership (`owner.type=User` and owner login matching GET /user). Public and private refer to GitHub Free personal repositories.

| TOML field | Public | Private | Read / write |
| --- | --- | --- | --- |
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

Permanent Issue creation restrictions have no confirmed basic repository setting in this investigation. [Interaction limits](https://docs.github.com/en/communities/moderating-comments-and-conversations/limiting-interactions-in-your-repository) are temporary moderation, not a permanent Issue policy. No issue creation-policy schema or automatic refresh is implemented; such keys fail strict parsing.

Organization repositories, other users' repositories, GHES, paid-only features (including private Wiki and private protected-branch features), rulesets, repository creation, allowlist pattern management and visibility changes are outside this implementation. Archived repositories fail preflight. API 401/403/404/422 and rate-limit failures remain errors, never a no-op or automatic fallback.
