package policy

import "fmt"

// Release-tag bypass actors are managed explicitly: only the personal owner may
// create, update or delete v* tags. Unrelated rules in this named ruleset survive.
func planReleaseTagRuleset(p *Plan, t Target, s State) error {
	const field = "rulesets.release_tag_protection"
	if t.Private {
		return fmt.Errorf("%s: unavailable for private repositories on GitHub Free", field)
	}
	if !s.ReleaseTagsKnown {
		return fmt.Errorf("%s: release tag ruleset was not fetched", field)
	}
	if t.Owner.ID <= 0 {
		return fmt.Errorf("%s: missing personal owner's numeric user ID", field)
	}
	body := map[string]any{"name": "release-tags", "target": "tag", "enforcement": "active", "conditions": map[string]any{}, "rules": []any{}, "bypass_actors": []any{}}
	before := any("absent")
	method, endpoint := "POST", "repos/"+t.FullName+"/rulesets"
	if current := s.ReleaseTagRuleset; current != nil {
		id, err := RulesetID(current["id"])
		if err != nil {
			return fmt.Errorf("%s: invalid ruleset ID", field)
		}
		if current["name"] != "release-tags" || current["target"] != "tag" {
			return fmt.Errorf("%s: existing release-tags ruleset must target tags", field)
		}
		if current["enforcement"] != "active" && current["enforcement"] != "disabled" && current["enforcement"] != "evaluate" {
			return fmt.Errorf("%s: missing or invalid enforcement", field)
		}
		copied := cloneObject(current)
		for _, key := range []string{"conditions", "rules", "bypass_actors", "enforcement"} {
			if copied[key] == nil {
				return fmt.Errorf("%s: missing current %s", field, key)
			}
			body[key] = copied[key]
		}
		if _, ok := body["bypass_actors"].([]any); !ok {
			return fmt.Errorf("%s: invalid bypass_actors", field)
		}
		method, endpoint = "PUT", endpoint+"/"+id
	}
	rules, ok := body["rules"].([]any)
	if !ok {
		return fmt.Errorf("%s: invalid current rules", field)
	}
	types := map[string]bool{}
	for _, rule := range rules {
		object, ok := rule.(map[string]any)
		if !ok {
			return fmt.Errorf("%s: invalid current rule", field)
		}
		kind, ok := object["type"].(string)
		if !ok || kind == "" || types[kind] {
			return fmt.Errorf("%s: missing or duplicate current rule type", field)
		}
		types[kind] = true
	}
	ruleOrder(rules)
	if s.ReleaseTagRuleset != nil {
		before = cloneObject(body)
	}
	conditions, ok := body["conditions"].(map[string]any)
	if !ok {
		return fmt.Errorf("%s: invalid conditions", field)
	}
	conditions["ref_name"] = map[string]any{"include": []any{"refs/tags/v*"}, "exclude": []any{}}
	body["enforcement"] = "active"
	// User IDs come from verified repository metadata, never undocumented role IDs.
	body["bypass_actors"] = []any{map[string]any{"actor_type": "User", "actor_id": t.Owner.ID, "bypass_mode": "always"}}
	for _, kind := range []string{"creation", "update", "deletion"} {
		if !types[kind] {
			rules = append(rules, map[string]any{"type": kind})
		}
	}
	ruleOrder(rules)
	body["rules"] = rules
	if sameJSON(before, body) {
		return nil
	}
	p.Changes = append(p.Changes, Change{field, before, body})
	p.Operations = append(p.Operations, Operation{method, endpoint, body})
	return nil
}
