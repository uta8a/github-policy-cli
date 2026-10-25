---
name: dependency-cooldown
description: Apply a 14-day release cooldown when adding, updating, or checking project dependencies, including manifest and lockfile changes, dependency audits, and update-bot configuration. Use for dependency work; ordinary builds and tests alone do not require a dependency audit.
---

# Dependency cooldown: 14 days

Use this skill when adding, updating or checking dependencies. Only adopt versions published at least **14 days (336 hours)** ago.

## Eligibility

- Compare the current time and publication time in UTC. A candidate is eligible when `published_at <= now - 14 days`. Exactly 14 days qualifies; future timestamps do not.
- Verify the version-specific publication timestamp in the official package registry or the upstream project's official release metadata. If reliable publication timestamps disagree, use the later timestamp. Record the source and the time of verification.
- Do not substitute commit times, download times or cache modification times for publication times. In particular, a Go module's `.info` `Time` alone does not prove that a tag or release has been public for 14 days. Use an official release's `published_at` or reliable registry evidence that the version was already observable at least 14 days ago.
- Treat candidates with unverifiable publication times as unverified and do not adopt them. Do not guess when network restrictions prevent verification.

## Additions and updates

1. Inspect the existing manifests, lockfiles and affected packages.
2. Select the newest appropriate stable version that meets compatibility requirements and has passed the cooldown. Do not adopt `latest` without verification.
3. Update manifests and lockfiles after selecting eligible candidates. Inspect the resolved diff and apply the same check to newly introduced or changed transitive dependencies. Adjust the dependency graph or defer the affected change if it does not qualify.
4. Run checks appropriate to the change. Report the selected versions, publication timestamps, source URLs and cooldown results.

Do not audit every dependency merely because an ordinary build or test is running without dependency changes. Do not automatically downgrade existing dependencies. Apply the same cooldown when selecting a new toolchain version.

## Checks and audits

For read-only checks, leave files unchanged and classify candidates as eligible, younger than 14 days, or publication time unverified. For candidates still in cooldown, report the UTC time when they become eligible. Report existing versions younger than 14 days without automatically downgrading them.

Do not silently exempt security fixes younger than 14 days. Explain both the vulnerability and the cooldown restriction, and select an eligible patched version if one exists. Only an explicit user instruction can authorize an exception; record its scope and rationale. Passing the cooldown does not establish that a dependency is safe.

When asked to configure an update bot, consult its official documentation and set the existing tool's supported cooldown to 14 days. This skill alone does not authorize installing a new tool or enabling new automated jobs.
