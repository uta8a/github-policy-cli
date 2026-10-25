package policy

import (
	"reflect"
	"strings"
	"testing"
)

func TestReleaseTagRuleset(t *testing.T) {
	cfg := Config{}
	cfg.Rulesets.ReleaseTagProtection = ptr(true)
	repo := target(false)
	s := state()
	s.ReleaseTagsKnown = true
	p, err := Build(cfg, repo, s)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Operations) != 1 || p.Operations[0].Method != "POST" || len(p.Changes) != 1 || p.Changes[0].Path != "rulesets.release_tag_protection" {
		t.Fatalf("plan=%+v", p)
	}
	body := p.Operations[0].Body
	if body["target"] != "tag" || body["enforcement"] != "active" || body["name"] != "release-tags" {
		t.Fatalf("body=%v", body)
	}
	wantActors := []any{map[string]any{"actor_type": "User", "actor_id": int64(42), "bypass_mode": "always"}}
	if !reflect.DeepEqual(body["bypass_actors"], wantActors) {
		t.Fatalf("actors=%v", body["bypass_actors"])
	}
	if !sameJSON(body["conditions"], map[string]any{"ref_name": map[string]any{"include": []any{"refs/tags/v*"}, "exclude": []any{}}}) {
		t.Fatal("incorrect tag scope")
	}
	if !sameJSON(body["rules"], []any{map[string]any{"type": "creation"}, map[string]any{"type": "deletion"}, map[string]any{"type": "update"}}) {
		t.Fatal("missing tag restriction")
	}
	current := cloneObject(body)
	current["id"] = float64(3)
	s.ReleaseTagRuleset = current
	p, err = Build(cfg, repo, s)
	if err != nil || len(p.Operations) != 0 {
		t.Fatalf("not idempotent: %v %+v", err, p)
	}
	current["enforcement"] = "disabled"
	current["bypass_actors"] = []any{map[string]any{"actor_type": "User", "actor_id": float64(99), "bypass_mode": "always"}, map[string]any{"actor_type": "Integration", "actor_id": float64(7), "bypass_mode": "always"}}
	current["rules"] = append(current["rules"].([]any), map[string]any{"type": "required_signatures"})
	snapshot := cloneObject(current)
	p, err = Build(cfg, repo, s)
	if err != nil {
		t.Fatal(err)
	}
	if p.Operations[0].Method != "PUT" || p.Operations[0].Endpoint != "repos/me/repo/rulesets/3" || !sameJSON(p.Operations[0].Body["bypass_actors"], wantActors) {
		t.Fatalf("broad bypass retained: %+v", p)
	}
	if len(p.Operations[0].Body["rules"].([]any)) != 4 || !reflect.DeepEqual(snapshot, current) {
		t.Fatal("extra rule lost or input mutated")
	}
}
func TestReleaseTagFailures(t *testing.T) {
	cfg := Config{}
	cfg.Rulesets.ReleaseTagProtection = ptr(true)
	for _, private := range []bool{false, true} {
		err := target(private).Validate(cfg, "me")
		if private != (err != nil) {
			t.Fatalf("private=%v err=%v", private, err)
		}
	}
	if _, err := Parse("policy.toml", []byte("[rulesets]\nrelease_tag_protection=false")); err == nil {
		t.Fatal("false accepted")
	}
	repo := target(false)
	s := state()
	if _, err := Build(cfg, repo, s); err == nil {
		t.Fatal("unfetched treated as absent")
	}
	s.ReleaseTagsKnown = true
	repo.Owner.ID = 0
	if _, err := Build(cfg, repo, s); err == nil {
		t.Fatal("missing owner ID accepted")
	}
	repo = target(false)
	p, err := Build(cfg, repo, s)
	if err != nil {
		t.Fatal(err)
	}
	current := cloneObject(p.Operations[0].Body)
	current["id"] = float64(3)
	s.ReleaseTagRuleset = current
	delete(current, "bypass_actors")
	if _, err := Build(cfg, repo, s); err == nil || !strings.Contains(err.Error(), "bypass_actors") {
		t.Fatalf("unreadable bypass accepted: %v", err)
	}
}
