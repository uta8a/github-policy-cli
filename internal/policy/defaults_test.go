package policy

import (
	"reflect"
	"strings"
	"testing"
)

func TestPersonalDefaults(t *testing.T) {
	defaults := PersonalDefaults()
	for _, private := range []bool{false, true} {
		repo := target(private)
		cfg, err := defaults.ForTarget(repo)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.Validate(cfg, "me"); err != nil {
			t.Fatal(err)
		}
		if private {
			want := map[string]any{"repository.has_projects": false, "repository.allow_merge_commit": true, "repository.allow_squash_merge": false, "repository.allow_rebase_merge": false, "repository.allow_update_branch": true, "repository.delete_branch_on_merge": true, "pull_requests.creation_policy": "collaborators_only", "releases.immutable": true, "actions.sha_pinning_required": true, "security.dependabot_security_updates": false}
			if !reflect.DeepEqual(cfg.Desired(), want) || cfg.Rulesets.MainProtection != nil || cfg.Rulesets.ReleaseTagProtection != nil || cfg.Repository.HasWiki != nil {
				t.Fatalf("private defaults=%+v", cfg)
			}
		} else {
			base, err := defaults.ForTarget(target(true))
			if err != nil {
				t.Fatal(err)
			}
			want := base.Desired()
			want["actions.fork_pr_approval_policy"] = "all_external_contributors"
			if !reflect.DeepEqual(cfg.Desired(), want) || cfg.Rulesets.MainProtection == nil || !*cfg.Rulesets.MainProtection || cfg.Rulesets.ReleaseTagProtection == nil || !*cfg.Rulesets.ReleaseTagProtection {
				t.Fatalf("public defaults=%+v", cfg)
			}
		}
	}
	explicit, err := Parse("explicit.toml", []byte("[repository]\nhas_issues=false"))
	if err != nil {
		t.Fatal(err)
	}
	resolved, err := explicit.ForTarget(target(true))
	if err != nil || !reflect.DeepEqual(resolved.Desired(), map[string]any{"repository.has_issues": false}) {
		t.Fatal("explicit config inherited defaults")
	}
}
func TestNewSettingsCapabilities(t *testing.T) {
	rules := Config{}
	rules.Rulesets.MainProtection = ptr(true)
	approval := Config{}
	approval.Actions.ForkPRApproval = ptr("all_external_contributors")
	for _, cfg := range []Config{rules, approval} {
		for _, private := range []bool{false, true} {
			err := target(private).Validate(cfg, "me")
			if private != (err != nil) {
				t.Fatalf("private=%v err=%v", private, err)
			}
		}
	}
	for _, body := range []string{"[rulesets]\nmain_protection=false", "[actions]\nfork_pr_approval_policy='all'", "[issues]\ncreation_policy='collaborators_only'"} {
		if _, err := Parse("config.toml", []byte(body)); err == nil {
			t.Errorf("accepted %s", body)
		}
	}
}
func TestMainRulesetPreservesUnmanagedFields(t *testing.T) {
	c := Config{}
	c.Rulesets.MainProtection = ptr(true)
	s := state()
	s.RulesetsKnown = true
	p, err := Build(c, target(false), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Operations) != 1 || p.Operations[0].Method != "POST" {
		t.Fatalf("plan=%+v", p)
	}
	current := cloneObject(p.Operations[0].Body)
	current["id"] = float64(9)
	s.MainRuleset = current
	p, err = Build(c, target(false), s)
	if err != nil || len(p.Operations) != 0 {
		t.Fatalf("non-idempotent plan=%+v err=%v", p, err)
	}
	current["enforcement"] = "disabled"
	current["bypass_actors"] = []any{map[string]any{"actor_id": float64(5), "actor_type": "RepositoryRole", "bypass_mode": "always"}}
	current["rules"] = append(current["rules"].([]any), map[string]any{"type": "required_linear_history"})
	var pr map[string]any
	for _, r := range current["rules"].([]any) {
		o := r.(map[string]any)
		if o["type"] == "pull_request" {
			pr = o["parameters"].(map[string]any)
		}
	}
	pr["required_approving_review_count"] = float64(2)
	pr["require_code_owner_review"] = true
	snapshot := cloneObject(current)
	p, err = Build(c, target(false), s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Operations) != 1 || p.Operations[0].Endpoint != "repos/me/repo/rulesets/9" {
		t.Fatalf("ops=%+v", p.Operations)
	}
	body := p.Operations[0].Body
	if !reflect.DeepEqual(body["bypass_actors"], current["bypass_actors"]) {
		t.Fatal("bypass actors lost")
	}
	foundLinear, foundPR := false, false
	for _, r := range body["rules"].([]any) {
		o := r.(map[string]any)
		foundLinear = foundLinear || o["type"] == "required_linear_history"
		if o["type"] == "pull_request" {
			v := o["parameters"].(map[string]any)
			foundPR = v["required_approving_review_count"] == 0 && v["require_code_owner_review"] == true
		}
	}
	if !foundLinear || !foundPR || !reflect.DeepEqual(snapshot, current) {
		t.Fatal("unmanaged state lost or planner mutated input")
	}
}
func TestMainRulesetFailsClosed(t *testing.T) {
	c := Config{}
	c.Rulesets.MainProtection = ptr(true)
	s := state()
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("unfetched rulesets accepted")
	}
	s.RulesetsKnown = true
	s.Repository["allow_merge_commit"] = false
	if _, err := Build(c, target(false), s); err == nil {
		t.Fatal("unusable merge-only rule accepted")
	}
	s = state()
	s.RulesetsKnown = true
	p, err := Build(c, target(false), s)
	if err != nil {
		t.Fatal(err)
	}
	current := cloneObject(p.Operations[0].Body)
	current["id"] = float64(1)
	delete(current, "bypass_actors")
	s.MainRuleset = current
	if _, err = Build(c, target(false), s); err == nil || !strings.Contains(err.Error(), "bypass_actors") {
		t.Fatalf("missing bypass accepted: %v", err)
	}
}
func TestFeatureOperationsAndSHAPinning(t *testing.T) {
	c, err := Parse("private.toml", privateDefaults)
	if err != nil {
		t.Fatal(err)
	}
	s := state()
	s.Repository["has_projects"] = true
	s.Repository["allow_update_branch"] = false
	s.Actions["sha_pinning_required"] = false
	s.Releases = map[string]any{"enabled": false, "enforced_by_owner": false}
	s.Security = map[string]any{"enabled": true}
	p, err := Build(c, target(true), s)
	if err != nil {
		t.Fatal(err)
	}
	foundRelease, foundSecurity, foundSHA := false, false, false
	for _, op := range p.Operations {
		switch op.Endpoint {
		case "repos/me/repo/immutable-releases":
			foundRelease = op.Method == "PUT" && op.Body == nil
		case "repos/me/repo/automated-security-fixes":
			foundSecurity = op.Method == "DELETE" && op.Body == nil
		case "repos/me/repo/actions/permissions":
			foundSHA = op.Body["sha_pinning_required"] == true && op.Body["enabled"] == true && op.Body["allowed_actions"] == "selected"
		}
	}
	if !foundRelease || !foundSecurity || !foundSHA {
		t.Fatalf("ops=%+v", p.Operations)
	}
	delete(s.Releases, "enabled")
	if _, err = Build(c, target(true), s); err == nil {
		t.Fatal("missing release flag accepted")
	}
}
