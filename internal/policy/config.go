package policy

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

type Repository struct {
	HasIssues           *bool `toml:"has_issues"`
	HasWiki             *bool `toml:"has_wiki"`
	DeleteBranchOnMerge *bool `toml:"delete_branch_on_merge"`
	AllowSquashMerge    *bool `toml:"allow_squash_merge"`
	AllowMergeCommit    *bool `toml:"allow_merge_commit"`
	AllowRebaseMerge    *bool `toml:"allow_rebase_merge"`
}
type PullRequests struct {
	CreationPolicy *string `toml:"creation_policy"`
}
type Workflow struct {
	DefaultPermissions *string `toml:"default_workflow_permissions"`
	CanApprove         *bool   `toml:"can_approve_pull_request_reviews"`
}
type Actions struct {
	Enabled  *bool    `toml:"enabled"`
	Allowed  *string  `toml:"allowed_actions"`
	Workflow Workflow `toml:"workflow"`
}
type Config struct {
	Repository   Repository   `toml:"repository"`
	PullRequests PullRequests `toml:"pull_requests"`
	Actions      Actions      `toml:"actions"`
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return Parse(path, data)
}
func Parse(path string, data []byte) (Config, error) {
	var c Config
	d := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		if strict, ok := err.(*toml.StrictMissingError); ok {
			fields := make([]string, 0, len(strict.Errors))
			for _, e := range strict.Errors {
				fields = append(fields, strings.Join(e.Key(), "."))
			}
			return c, fmt.Errorf("%s: unknown field(s): %s", path, strings.Join(fields, ", "))
		}
		if de, ok := err.(*toml.DecodeError); ok {
			row, col := de.Position()
			field := strings.Join(de.Key(), ".")
			if field == "" {
				field = "<syntax>"
			}
			return c, fmt.Errorf("%s: %s (line %d, column %d): %w", path, field, row, col, err)
		}
		return c, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return c, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}
func (c Config) Validate() error {
	for _, v := range []struct {
		path    string
		value   *string
		allowed []string
	}{
		{"pull_requests.creation_policy", c.PullRequests.CreationPolicy, []string{"all", "collaborators_only"}},
		{"actions.allowed_actions", c.Actions.Allowed, []string{"all", "local_only", "selected"}},
		{"actions.workflow.default_workflow_permissions", c.Actions.Workflow.DefaultPermissions, []string{"read", "write"}},
	} {
		if v.value != nil {
			valid := false
			for _, a := range v.allowed {
				valid = valid || *v.value == a
			}
			if !valid {
				return fmt.Errorf("%s: expected one of %v", v.path, v.allowed)
			}
		}
	}
	if c.Actions.Enabled != nil && !*c.Actions.Enabled && (c.Actions.Allowed != nil || c.NeedsWorkflow()) {
		return fmt.Errorf("actions.enabled: false cannot be combined with actions.allowed_actions or actions.workflow settings")
	}
	r := c.Repository
	if r.AllowSquashMerge != nil && r.AllowMergeCommit != nil && r.AllowRebaseMerge != nil && !*r.AllowSquashMerge && !*r.AllowMergeCommit && !*r.AllowRebaseMerge {
		return fmt.Errorf("repository.allow_*_merge: at least one merge method must be enabled")
	}
	return nil
}
func (c Config) NeedsWorkflow() bool {
	return c.Actions.Workflow.DefaultPermissions != nil || c.Actions.Workflow.CanApprove != nil
}
func (c Config) NeedsActions() bool {
	return c.Actions.Enabled != nil || c.Actions.Allowed != nil || c.NeedsWorkflow()
}

// Desired contains API names; CLI-specific nesting stays in Config.
func (c Config) Desired() map[string]any {
	out := map[string]any{}
	for _, f := range []struct {
		k string
		v *bool
	}{
		{"has_issues", c.Repository.HasIssues}, {"has_wiki", c.Repository.HasWiki}, {"delete_branch_on_merge", c.Repository.DeleteBranchOnMerge},
		{"allow_squash_merge", c.Repository.AllowSquashMerge}, {"allow_merge_commit", c.Repository.AllowMergeCommit}, {"allow_rebase_merge", c.Repository.AllowRebaseMerge},
	} {
		if f.v != nil {
			out["repository."+f.k] = *f.v
		}
	}
	if c.PullRequests.CreationPolicy != nil {
		out["pull_requests.creation_policy"] = *c.PullRequests.CreationPolicy
	}
	if c.Actions.Enabled != nil {
		out["actions.enabled"] = *c.Actions.Enabled
	}
	if c.Actions.Allowed != nil {
		out["actions.allowed_actions"] = *c.Actions.Allowed
	}
	if c.Actions.Workflow.DefaultPermissions != nil {
		out["actions.workflow.default_workflow_permissions"] = *c.Actions.Workflow.DefaultPermissions
	}
	if c.Actions.Workflow.CanApprove != nil {
		out["actions.workflow.can_approve_pull_request_reviews"] = *c.Actions.Workflow.CanApprove
	}
	return out
}
