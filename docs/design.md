# Initial implementation

## Execution contract

Select embedded personal presets, or load and validate explicit strict TOML → resolve preset per target visibility → authenticate GET /user → discover targets → fetch repository metadata → validate capabilities → fetch requested settings → create deterministic plans → render or execute.

Every invocation reads fresh state. No persistent state or saved plan. The entire target set is preflighted before apply makes any change: any ownership, capability, unreadable-field or configuration error aborts all writes. Preflight visits remaining targets to report errors. Once execution starts, API failures do not stop independent API groups or other repositories; the CLI reports successful/failed API operations and returns nonzero. No rollback or transaction is promised. Concurrent GitHub edits can still race the read/write interval.

## Packages

- cmd/repo-policy: process exit status.
- internal/cli: Cobra, orchestration, deterministic text rendering and summary.
- internal/policy: optional TOML model, strict validation, scope/capability checks and pure planner.
- internal/github: go-gh RESTClient, authentication, pagination, reads, writes and safe error classification.

Without --config, Config is a small preset-selection marker until repository metadata is fetched. The embedded defaults/private.toml and defaults/public.toml are the single source of truth. The public preset contains all private preset settings plus public ruleset and contributor-approval settings; a subset regression test keeps the shared values aligned. An explicit --config path replaces all defaults, including an explicitly empty file; omitted fields retain their existing values. Commands accept exactly one owner/repo or --all, never both. Errors include the configuration path for configuration/capability/current-value failures. No changes returns exit 0 and `No changes.`; differences also return 0 in plan.

## Requests

Only requested API groups are read. Missing/null requested values fail planning, rather than defaulting to false. Repository metadata is always fetched to establish ownership and visibility; archived repositories are rejected as read-only.

Repository PATCH contains changed fields only. Actions PUT requires enabled: when only allowed_actions changes, preserve enabled from GET. Preserve existing allowed_actions and sha_pinning_required whenever returned, including when disabling Actions. Workflow PUT sends both known workflow fields, combining current and desired values. Never follow selected_actions_url or alter the separate selected-action allowlist. allowed_actions=selected uses the existing allowlist; defining patterns is outside the schema.

Merged state must retain at least one PR merge method. Actions allowed_actions/workflow/fork-approval settings cannot be managed while the resulting enabled value is false (enable Actions explicitly if necessary). Enabling runs before workflow updates; if enabling fails, dependent workflow changes are skipped and reported. There is no automatic unarchive or automatic enabling of Actions.

## Validation

Unit tests cover strict config, omission, capability matrices and pure plans. Mock HTTP tests cover endpoint methods/payloads, private ownership, pagination, fresh state, partial failure, all-target preflight and idempotency. No real token or repository is required.

## Additional endpoints

Release immutability and Dependabot security updates use GET plus bodyless PUT/DELETE. Only these documented status endpoints interpret 404 as disabled, and only after a separate successful Administration-read check through Actions permissions. Other 404s remain errors. Do not treat paused Dependabot updates as disabled. Missing required current values fail planning.

Public fork contributor approval uses its own GET/PUT endpoint and is never inferred from private-fork workflow settings. SHA pinning uses the existing Actions permissions endpoint; it may be configured while Actions are disabled.

The public main_protection profile lists rulesets with includes_parents=false and pagination, finds the sole repository-owned rule named main, and fetches its full details. Absence is established only by a successful complete list; null lists, conflicting names/types, missing bypass actors or required PR parameters fail closed. A new ruleset is POSTed; an existing one is PUT by ID. Branch target, active enforcement, default-branch conditions and the four requested rules are managed; unrelated rules, PR parameters and bypass actors are copied from current state. Rules are sorted by type for comparison; planning never mutates API state. Only the main ruleset request is skipped if the preceding repository update that enables merge commits fails; independent release-tag protection continues. Independent security updates continue even after Actions enable fails.

Issue collaborator-only permissions are a documented UI feature but have no confirmed REST schema. Both presets print a manual-setting note. Explicit issues.creation_policy fails validation. No UI automation or interaction-limit fallback is used.

## Release-tag protection

The public preset also enables rulesets.release_tag_protection. Fetch the sole repository-owned ruleset named release-tags with the same paginated discovery and full-detail checks used for main. Keep a separate fetched-state marker so an unread rule is never treated as absent.

The planner manages target=tag, active enforcement, include=[refs/tags/v*], exclude=[], and creation/update/deletion restrictions. Its sole bypass actor is the personal repository owner, resolved from verified repository metadata as actor_type=User and actor_id=owner.id, with bypass_mode=always. Numeric RepositoryRole mappings are not guessed. Existing release-tags bypass actors are deliberately replaced, rather than preserved, to prevent a pre-existing collaborator/app exception from defeating the owner-only policy. Additional rules are preserved; unreadable bypass state, conflicting names/types or missing owner ID fail planning. The complete change appears in the plan. Private Free targets reject the profile before any writes.

This server-side tag policy is the trust boundary. A tagged workflow can remove its own ancestry check, so that check remains defense against mistakes rather than a defense against a tag-pushing attacker. A trusted owner must apply the policy before relying on tag-triggered publishing. The workflow does not install the ruleset itself, and no settings are applied as part of tests.
