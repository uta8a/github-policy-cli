package policy

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type Target struct {
	FullName string `json:"full_name"`
	Owner    struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	} `json:"owner"`
	Private    bool   `json:"private"`
	Visibility string `json:"visibility"`
	Archived   bool   `json:"archived"`
}

func (t Target) OwnedBy(login string) bool {
	return t.Owner.Type == "User" && strings.EqualFold(t.Owner.Login, login)
}
func (t Target) Validate(c Config, login string) error {
	if !t.OwnedBy(login) {
		return fmt.Errorf("%s: only authenticated user's personally owned repositories are supported", t.FullName)
	}
	if t.Visibility != "public" && t.Visibility != "private" {
		return fmt.Errorf("%s: unsupported or missing visibility %q", t.FullName, t.Visibility)
	}
	if (t.Visibility == "private") != t.Private {
		return fmt.Errorf("%s: inconsistent repository visibility", t.FullName)
	}
	if t.Archived {
		return fmt.Errorf("%s: archived repositories are read-only", t.FullName)
	}
	if t.Private && c.Rulesets.MainProtection != nil {
		return fmt.Errorf("%s: rulesets.main_protection: rulesets are unavailable for private repositories on GitHub Free", t.FullName)
	}
	if t.Private && c.Actions.ForkPRApproval != nil {
		return fmt.Errorf("%s: actions.fork_pr_approval_policy: public-fork contributor approval is only supported for public repositories", t.FullName)
	}
	if t.Private && c.Repository.HasWiki != nil {
		return fmt.Errorf("%s: repository.has_wiki: unavailable for private repositories on GitHub Free (omit this field)", t.FullName)
	}
	return nil
}

type State struct {
	Releases      map[string]any
	Security      map[string]any
	ForkPR        map[string]any
	MainRuleset   map[string]any
	RulesetsKnown bool
	Repository    map[string]any
	Actions       map[string]any
	Workflow      map[string]any
}
type Change struct {
	Path          string
	Before, After any
}
type Operation struct {
	Method, Endpoint string
	Body             map[string]any
}
type Plan struct {
	Repository string
	Changes    []Change
	Operations []Operation
	Notes      []string
}

func actual(s State, path string) (any, bool) {
	switch {
	case path == "releases.immutable":
		v, ok := s.Releases["enabled"]
		return v, ok
	case path == "security.dependabot_security_updates":
		v, ok := s.Security["enabled"]
		return v, ok
	case path == "actions.fork_pr_approval_policy":
		v, ok := s.ForkPR["approval_policy"]
		return v, ok
	case path == "pull_requests.creation_policy":
		v, ok := s.Repository["pull_request_creation_policy"]
		return v, ok
	case strings.HasPrefix(path, "repository."):
		v, ok := s.Repository[strings.TrimPrefix(path, "repository.")]
		return v, ok
	case strings.HasPrefix(path, "actions.workflow."):
		v, ok := s.Workflow[strings.TrimPrefix(path, "actions.workflow.")]
		return v, ok
	default:
		v, ok := s.Actions[strings.TrimPrefix(path, "actions.")]
		return v, ok
	}
}
func required(m map[string]any, k string, kind any) (any, error) {
	v, ok := m[k]
	if !ok || v == nil || reflect.TypeOf(v) != reflect.TypeOf(kind) {
		return nil, fmt.Errorf("%s: missing or invalid current value", k)
	}
	return v, nil
}

// Build is pure: it validates current values and constructs requests without I/O.
func Build(c Config, t Target, s State) (Plan, error) {
	resolved, err := c.ForTarget(t)
	if err != nil {
		return Plan{}, err
	}
	isDefault := c.UsesPersonalDefaults()
	c = resolved
	p := Plan{Repository: t.FullName}
	if isDefault && t.Private {
		p.Notes = append(p.Notes, "Manual setting required: Issues > Issue permissions > Collaborators only (no documented REST read/write field). Private rulesets omitted: unavailable on GitHub Free.")
	}
	if t.Private && c.Rulesets.MainProtection != nil {
		return p, fmt.Errorf("rulesets.main_protection: unavailable for private repositories on GitHub Free")
	}
	if err := c.Validate(); err != nil {
		return p, err
	}
	desired := c.Desired()
	keys := make([]string, 0, len(desired))
	for k := range desired {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	repo, actions, workflow := map[string]any{}, map[string]any{}, map[string]any{}
	for _, path := range keys {
		before, ok := actual(s, path)
		after := desired[path]
		if !ok || before == nil || reflect.TypeOf(before) != reflect.TypeOf(after) {
			return p, fmt.Errorf("%s: missing or invalid current value", path)
		}
		if v, ok := before.(string); ok {
			valid := true
			switch path {
			case "pull_requests.creation_policy":
				valid = v == "all" || v == "collaborators_only"
			case "actions.fork_pr_approval_policy":
				valid = v == "first_time_contributors_new_to_github" || v == "first_time_contributors" || v == "all_external_contributors"
			case "actions.allowed_actions":
				valid = v == "all" || v == "local_only" || v == "selected"
			case "actions.workflow.default_workflow_permissions":
				valid = v == "read" || v == "write"
			}
			if !valid {
				return p, fmt.Errorf("%s: unrecognized current value", path)
			}
		}
		if reflect.DeepEqual(before, after) {
			continue
		}
		p.Changes = append(p.Changes, Change{path, before, after})
		switch {
		case path == "releases.immutable" || path == "security.dependabot_security_updates" || path == "actions.fork_pr_approval_policy":
			// Dedicated endpoint operations are built after the grouped requests.
		case path == "pull_requests.creation_policy":
			repo["pull_request_creation_policy"] = after
		case strings.HasPrefix(path, "repository."):
			repo[strings.TrimPrefix(path, "repository.")] = after
		case strings.HasPrefix(path, "actions.workflow."):
			workflow[strings.TrimPrefix(path, "actions.workflow.")] = after
		default:
			actions[strings.TrimPrefix(path, "actions.")] = after
		}
	}
	r := c.Repository
	if r.AllowSquashMerge != nil || r.AllowMergeCommit != nil || r.AllowRebaseMerge != nil {
		enabled := false
		for _, k := range []string{"allow_squash_merge", "allow_merge_commit", "allow_rebase_merge"} {
			v, err := required(s.Repository, k, false)
			if err != nil {
				return p, fmt.Errorf("repository.%w", err)
			}
			if d, ok := desired["repository."+k]; ok {
				v = d
			}
			enabled = enabled || v.(bool)
		}
		if !enabled {
			return p, fmt.Errorf("repository.allow_*_merge: resulting state disables every merge method")
		}
	}
	if c.NeedsActions() {
		v, err := required(s.Actions, "enabled", false)
		if err != nil {
			return p, fmt.Errorf("actions.%w", err)
		}
		enabled := v.(bool)
		if c.Actions.Enabled != nil {
			enabled = *c.Actions.Enabled
		}
		if !enabled && (c.Actions.Allowed != nil || c.Actions.ForkPRApproval != nil || c.NeedsWorkflow()) {
			return p, fmt.Errorf("actions.enabled: enable Actions before managing allowed_actions or workflow settings")
		}
	}
	base := "repos/" + t.FullName
	if len(repo) > 0 {
		p.Operations = append(p.Operations, Operation{"PATCH", base, repo})
	}
	if len(actions) > 0 {
		if _, ok := actions["enabled"]; !ok {
			v, err := required(s.Actions, "enabled", false)
			if err != nil {
				return p, err
			}
			actions["enabled"] = v
		}
		for _, f := range []struct {
			k    string
			kind any
		}{{"allowed_actions", ""}, {"sha_pinning_required", false}} {
			if _, changed := actions[f.k]; changed {
				continue
			}
			if v, ok := s.Actions[f.k]; ok && v != nil {
				if reflect.TypeOf(v) != reflect.TypeOf(f.kind) {
					return p, fmt.Errorf("actions.%s: invalid current value", f.k)
				}
				if f.k == "allowed_actions" && v != "all" && v != "local_only" && v != "selected" {
					return p, fmt.Errorf("actions.allowed_actions: unrecognized current value")
				}
				actions[f.k] = v
			}
		}
		p.Operations = append(p.Operations, Operation{"PUT", base + "/actions/permissions", actions})
	}
	if len(workflow) > 0 {
		for _, f := range []struct {
			k    string
			kind any
		}{{"default_workflow_permissions", ""}, {"can_approve_pull_request_reviews", false}} {
			if _, ok := workflow[f.k]; !ok {
				v, err := required(s.Workflow, f.k, f.kind)
				if err != nil {
					return p, fmt.Errorf("actions.workflow.%w", err)
				}
				if f.k == "default_workflow_permissions" && v != "read" && v != "write" {
					return p, fmt.Errorf("actions.workflow.default_workflow_permissions: unrecognized current value")
				}
				workflow[f.k] = v
			}
		}
		p.Operations = append(p.Operations, Operation{"PUT", base + "/actions/permissions/workflow", workflow})
	}

	for _, change := range p.Changes {
		switch change.Path {
		case "releases.immutable", "security.dependabot_security_updates":
			endpoint := "automated-security-fixes"
			if change.Path == "releases.immutable" {
				endpoint = "immutable-releases"
				if change.After == false {
					enforced, err := required(s.Releases, "enforced_by_owner", false)
					if err != nil {
						return p, fmt.Errorf("releases.immutable: %w", err)
					}
					if enforced == true {
						return p, fmt.Errorf("releases.immutable: enforced by repository owner")
					}
				}
			}
			method := "DELETE"
			if change.After == true {
				method = "PUT"
			}
			p.Operations = append(p.Operations, Operation{Method: method, Endpoint: base + "/" + endpoint})
		case "actions.fork_pr_approval_policy":
			p.Operations = append(p.Operations, Operation{"PUT", base + "/actions/permissions/fork-pr-contributor-approval", map[string]any{"approval_policy": change.After}})
		}
	}
	if c.Rulesets.MainProtection != nil {
		if err := planMainRuleset(&p, t, s); err != nil {
			return p, err
		}
	}
	sort.Slice(p.Changes, func(i, j int) bool { return p.Changes[i].Path < p.Changes[j].Path })
	return p, nil
}
