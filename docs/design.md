# Initial implementation

## Execution contract

Load strict TOML → validate values → authenticate GET /user → discover targets → fetch repository metadata → validate capabilities → fetch requested settings → create deterministic plans → render or execute.

Every invocation reads fresh state. No persistent state or saved plan. The entire target set is preflighted before apply makes any change: any ownership, capability, unreadable-field or configuration error aborts all writes. Preflight visits remaining targets to report errors. Once execution starts, API failures do not stop independent API groups or other repositories; the CLI reports successful/failed API operations and returns nonzero. No rollback or transaction is promised. Concurrent GitHub edits can still race the read/write interval.

## Packages

- cmd/repo-policy: process exit status.
- internal/cli: Cobra, orchestration, deterministic text rendering and summary.
- internal/policy: optional TOML model, strict validation, scope/capability checks and pure planner.
- internal/github: go-gh RESTClient, authentication, pagination, reads, writes and safe error classification.

Config defaults to policy.toml. Commands accept exactly one owner/repo or --all, never both. Errors include the configuration path for configuration/capability/current-value failures. No changes returns exit 0 and `No changes.`; differences also return 0 in plan.

## Requests

Only requested API groups are read. Missing/null requested values fail planning, rather than defaulting to false. Repository metadata is always fetched to establish ownership and visibility; archived repositories are rejected as read-only.

Repository PATCH contains changed fields only. Actions PUT requires enabled: when only allowed_actions changes, preserve enabled from GET. Preserve existing allowed_actions and sha_pinning_required whenever returned, including when disabling Actions. Workflow PUT sends both known workflow fields, combining current and desired values. Never follow selected_actions_url or alter the separate selected-action allowlist. allowed_actions=selected uses the existing allowlist; defining patterns is outside the schema.

Merged state must retain at least one PR merge method. Actions allowed_actions/workflow settings cannot be managed while the resulting enabled value is false (enable Actions explicitly if necessary). Enabling runs before workflow updates; if enabling fails, dependent workflow changes are skipped and reported. There is no automatic unarchive or automatic enabling of Actions.

## Validation

Unit tests cover strict config, omission, capability matrices and pure plans. Mock HTTP tests cover endpoint methods/payloads, private ownership, pagination, fresh state, partial failure, all-target preflight and idempotency. No real token or repository is required.
