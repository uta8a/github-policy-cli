# Agent implementation guide

Read README.md, docs/design.md and docs/github-support.md before changing behavior.

Use English for all repository-facing text: README and other documentation, skills, code comments, CLI messages, commit messages, issues, and pull request titles/descriptions. Keep repository artifacts free of Japanese text.

Release versions use CalVer `vYYYY.0M.0D.MICRO`, with dates in Asia/Tokyo (JST). Start at 0 each day and increment for additional releases that day. Use `scripts/calver.py` to suggest and validate tags; never move a published release tag. See docs/versioning.md.

Keep repository-local skills under `.agents/skills/<skill-name>/` so Codex can discover them.

For dependency additions, updates, or checks, always read and apply [dependency-cooldown](.agents/skills/dependency-cooldown/SKILL.md). The cooldown is 14 days (336 hours); use the newest compatible stable release whose publication age is verified. Check newly introduced/changed transitive dependencies too. Only a direct instruction from the human user can authorize a cooldown exception; record its version, scope and rationale. Do not silently bypass the cooldown or automatically downgrade existing dependencies. Ordinary builds and tests alone do not trigger a dependency audit.

- Scope: GitHub.com, GitHub Free personal accounts, authenticated user's own public/private repositories. Reject Organization, collaborator-only access and other hosts. Never expand capabilities based on a paid plan.
- Consult GitHub official REST documentation and OpenAPI before adding settings. Record plan, visibility, ownership, GET/WRITE endpoints and fine-grained PAT permissions in docs/github-support.md. Never guess a missing current value.
- With no --config, select the embedded Public/Private personal preset per target. Explicit --config replaces the preset; never merge hidden defaults into an explicit policy. Public defaults must include all private defaults plus public-only protections. Display the manual Issue-permissions gap for both presets.
- Public main and release-tag rulesets only: preserve unrelated rules and main PR parameters/bypass actors. The release-tags profile explicitly replaces its bypass list with the personal owner alone; never preserve broader exceptions in this owner-only profile. Private Free rulesets must fail preflight. Do not delete rulesets or substitute temporary interaction limits for permanent Issue permissions.
- Go, Cobra, go-gh/v2 RESTClient and go-toml/v2; standard library otherwise. No gh api subprocesses, state files, saved plans, prompts or background jobs.
- Unspecified TOML keys MUST remain unchanged. Distinguish false from absent. Reject unknown keys and invalid enums with config path and field. Validate the entire batch before any write.
- Keep planner pure and deterministic. Group changes by API. PATCH only changed fields; PUT must preserve required companion fields and unrelated supported permission fields.
- --all must paginate and independently filter owner type and owner identity. Fine-grained PAT repository selection limits discovery.
- Continue other repositories after execution failures; return nonzero and report failures. Never retry writes blindly or claim transactional rollback.
- Tests must use fakes or httptest; never mutate live GitHub. Do not print tokens or HTTP debug traces.
- Required checks: go test ./..., go vet ./..., go build ./cmd/repo-policy. Add public/private capability, no-write preflight, pagination and idempotency tests for behavioral changes.
- Prefer small concrete packages. Do not introduce provider/plugin frameworks for hypothetical future plans.
