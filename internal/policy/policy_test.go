package policy

import (
	"reflect"
	"strings"
	"testing"
)

func ptr[T any](v T) *T { return &v }
func target(private bool) Target {
	t := Target{FullName: "me/repo", Private: private, Visibility: "public"}
	if private {
		t.Visibility = "private"
	}
	t.Owner.Login = "me"
	t.Owner.Type = "User"
	return t
}
func state() State {
	return State{
		Repository: map[string]any{"has_issues": true, "has_wiki": false, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": false, "pull_request_creation_policy": "all"},
		Actions:    map[string]any{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": true},
		Workflow:   map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true},
	}
}
func TestParse(t *testing.T) {
	for _, tc := range []struct{ name, body, field string }{
		{"unknown", "[repository]\nsecret=true", "secret"},
		{"empty unknown table", "[unknown]", "unknown"},
		{"unknown table", "[unknown]\nx=true", "unknown"},
		{"invalid PR", "[pull_requests]\ncreation_policy='invalid'", "pull_requests.creation_policy"},
		{"invalid Actions", "[actions]\nallowed_actions='none'", "actions.allowed_actions"},
		{"invalid workflow", "[actions.workflow]\ndefault_workflow_permissions='admin'", "actions.workflow.default_workflow_permissions"},
		{"type", "[repository]\nhas_issues='yes'", "repository.has_issues"},
		{"syntax", "[repository", ""},
		{"disabled contradiction", "[actions]\nenabled=false\nallowed_actions='all'", "actions.enabled"},
		{"merge contradiction", "[repository]\nallow_squash_merge=false\nallow_merge_commit=false\nallow_rebase_merge=false", "repository.allow_*_merge"},
		{"issue creation unsupported", "[issues]\ncreation_policy='collaborators_only'", "issues"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse("test.toml", []byte(tc.body))
			if err == nil || !strings.Contains(err.Error(), "test.toml") || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("error=%v", err)
			}
		})
	}
	c, err := Parse("test.toml", []byte("[repository]\nhas_issues=false"))
	if err != nil {
		t.Fatal(err)
	}
	if c.Repository.HasIssues == nil || *c.Repository.HasIssues || c.Repository.HasWiki != nil {
		t.Fatalf("optional values lost: %+v", c)
	}
	if _, err = Load("/missing/policy.toml"); err == nil {
		t.Fatal("expected read error")
	}
}
func TestCapabilities(t *testing.T) {
	c, err := Parse("policy.toml", []byte(`[repository]
has_issues = true
delete_branch_on_merge = true
allow_squash_merge = true
allow_merge_commit = false
allow_rebase_merge = false
[pull_requests]
creation_policy = "collaborators_only"
[actions]
enabled = true
allowed_actions = "all"
[actions.workflow]
default_workflow_permissions = "read"
can_approve_pull_request_reviews = false
`))
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []bool{false, true} {
		repo := target(private)
		if err := repo.Validate(c, "ME"); err != nil {
			t.Fatal(err)
		}
		if _, err := Build(c, repo, state()); err != nil {
			t.Fatal(err)
		}
		for _, value := range []bool{false, true} {
			wiki := c
			wiki.Repository.HasWiki = ptr(value)
			err := repo.Validate(wiki, "me")
			if private != (err != nil) {
				t.Fatalf("private=%v wiki=%v err=%v", private, value, err)
			}
		}
	}
	for _, tc := range []struct {
		name   string
		mutate func(*Target)
	}{
		{"organization", func(r *Target) { r.Owner.Type = "Organization" }},
		{"other owner", func(r *Target) { r.Owner.Login = "someone" }},
		{"internal", func(r *Target) { r.Visibility = "internal" }},
		{"missing visibility", func(r *Target) { r.Visibility = "" }},
		{"archived", func(r *Target) { r.Archived = true }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := target(false)
			tc.mutate(&r)
			if r.Validate(c, "me") == nil {
				t.Fatal("expected scope error")
			}
		})
	}
}
func TestPlanner(t *testing.T) {
	c := Config{}
	c.Repository.HasIssues = ptr(true)
	p, err := Build(c, target(false), state())
	if err != nil || len(p.Changes) != 0 || len(p.Operations) != 0 {
		t.Fatalf("p=%+v err=%v", p, err)
	}
	c.Repository.HasIssues = ptr(false)
	c.Repository.DeleteBranchOnMerge = ptr(true)
	c.PullRequests.CreationPolicy = ptr("collaborators_only")
	p, err = Build(c, target(false), state())
	if err != nil {
		t.Fatal(err)
	}
	want := []Change{{"pull_requests.creation_policy", "all", "collaborators_only"}, {"repository.delete_branch_on_merge", false, true}, {"repository.has_issues", true, false}}
	if !reflect.DeepEqual(p.Changes, want) {
		t.Fatalf("changes=%+v", p.Changes)
	}
	if len(p.Operations) != 1 || len(p.Operations[0].Body) != 3 || p.Operations[0].Method != "PATCH" {
		t.Fatalf("ops=%+v", p.Operations)
	}
	for range 20 {
		p2, _ := Build(c, target(false), state())
		if !reflect.DeepEqual(p, p2) {
			t.Fatal("nondeterministic")
		}
	}
	s := state()
	delete(s.Repository, "has_issues")
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("missing value treated as no-op")
	}
	s = state()
	s.Repository["has_issues"] = nil
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("null treated as false")
	}
	s = state()
	s.Repository["has_issues"] = "false"
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("invalid type accepted")
	}
}
func TestActionsCompanions(t *testing.T) {
	c := Config{}
	c.Actions.Allowed = ptr("all")
	c.Actions.Workflow.CanApprove = ptr(false)
	p, err := Build(c, target(false), state())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Operations) != 2 {
		t.Fatalf("ops=%+v", p.Operations)
	}
	want := map[string]any{"enabled": true, "allowed_actions": "all", "sha_pinning_required": true}
	if !reflect.DeepEqual(p.Operations[0].Body, want) {
		t.Fatalf("Actions companions lost: %+v", p.Operations[0])
	}
	want = map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": false}
	if !reflect.DeepEqual(p.Operations[1].Body, want) {
		t.Fatalf("workflow companions lost: %+v", p.Operations[1])
	}
	c = Config{}
	c.Actions.Enabled = ptr(false)
	p, err = Build(c, target(false), state())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(p.Operations[0].Body, map[string]any{"enabled": false, "allowed_actions": "selected", "sha_pinning_required": true}) {
		t.Fatal("disable lost allowlist policy")
	}
	c = Config{}
	c.Actions.Allowed = ptr("all")
	s := state()
	s.Actions["enabled"] = false
	if _, err = Build(c, target(false), s); err == nil {
		t.Fatal("allowed policy with disabled Actions accepted")
	}
	c.Actions.Enabled = ptr(true)
	p, err = Build(c, target(false), s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Operations[0].Body["enabled"] != true {
		t.Fatal("enable not applied")
	}
	c = Config{}
	c.Actions.Workflow.CanApprove = ptr(false)
	s = state()
	delete(s.Workflow, "default_workflow_permissions")
	if _, err = Build(c, target(false), s); err == nil {
		t.Fatal("missing workflow companion accepted")
	}
}
func TestResultingMergeValidation(t *testing.T) {
	c := Config{}
	c.Repository.AllowSquashMerge = ptr(false)
	s := state()
	s.Repository["allow_merge_commit"] = false
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("all merge methods disabled")
	}
}
