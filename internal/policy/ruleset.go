package policy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"sort"
)

func RulesetID(value any) (string, error) {
	n, ok := value.(float64)
	if !ok || n < 1 || n > 9007199254740991 || math.Trunc(n) != n {
		return "", fmt.Errorf("rulesets.main_protection: missing or invalid ruleset ID")
	}
	return fmt.Sprintf("%.0f", n), nil
}
func cloneObject(object map[string]any) map[string]any {
	data, _ := json.Marshal(object)
	var result map[string]any
	_ = json.Unmarshal(data, &result)
	return result
}
func ruleOrder(rules []any) {
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].(map[string]any)["type"].(string) < rules[j].(map[string]any)["type"].(string)
	})
}
func sameJSON(a, b any) bool {
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	return bytes.Equal(left, right)
}

// The named main profile manages only the requested rules. Other rules, PR
// parameters and bypass actors in this ruleset are carried forward unchanged.
func planMainRuleset(p *Plan, t Target, s State) error {
	if t.Private {
		return fmt.Errorf("rulesets.main_protection: unavailable for private repositories on GitHub Free")
	}
	if !s.RulesetsKnown {
		return fmt.Errorf("rulesets.main_protection: rulesets were not fetched")
	}
	merge, err := required(s.Repository, "allow_merge_commit", false)
	if err != nil {
		return fmt.Errorf("rulesets.main_protection: %w", err)
	}
	for _, change := range p.Changes {
		if change.Path == "repository.allow_merge_commit" {
			merge = change.After
		}
	}
	if merge != true {
		return fmt.Errorf("rulesets.main_protection: repository.allow_merge_commit must be true for the Merge-only rule")
	}
	current := s.MainRuleset
	body := map[string]any{"name": "main", "target": "branch", "enforcement": "active", "bypass_actors": []any{}, "conditions": map[string]any{}, "rules": []any{}}
	before := any("absent")
	method, endpoint := "POST", "repos/"+t.FullName+"/rulesets"
	if current != nil {
		id, err := RulesetID(current["id"])
		if err != nil {
			return err
		}
		if current["name"] != "main" || current["target"] != "branch" {
			return fmt.Errorf("rulesets.main_protection: existing main ruleset must target branches")
		}
		if current["enforcement"] != "active" && current["enforcement"] != "disabled" && current["enforcement"] != "evaluate" {
			return fmt.Errorf("rulesets.main_protection: missing or invalid enforcement")
		}
		copied := cloneObject(current)
		for _, key := range []string{"name", "target", "enforcement", "conditions", "rules", "bypass_actors"} {
			if copied[key] == nil {
				return fmt.Errorf("rulesets.main_protection: missing current %s (read access to full ruleset required)", key)
			}
			body[key] = copied[key]
		}
		if _, ok := body["bypass_actors"].([]any); !ok {
			return fmt.Errorf("rulesets.main_protection: invalid bypass_actors")
		}
		method, endpoint = "PUT", endpoint+"/"+id
	}
	rules, ok := body["rules"].([]any)
	if !ok {
		return fmt.Errorf("rulesets.main_protection: invalid current rules")
	}
	byType := map[string]map[string]any{}
	for _, rule := range rules {
		object, ok := rule.(map[string]any)
		if !ok {
			return fmt.Errorf("rulesets.main_protection: invalid current rule")
		}
		kind, ok := object["type"].(string)
		if !ok || kind == "" || byType[kind] != nil {
			return fmt.Errorf("rulesets.main_protection: missing or duplicate current rule type")
		}
		byType[kind] = object
	}
	ruleOrder(rules)
	if current != nil {
		before = cloneObject(body)
	}
	body["enforcement"] = "active"
	conditions, ok := body["conditions"].(map[string]any)
	if !ok {
		return fmt.Errorf("rulesets.main_protection: invalid conditions")
	}
	conditions["ref_name"] = map[string]any{"include": []any{"~DEFAULT_BRANCH"}, "exclude": []any{}}
	for _, kind := range []string{"deletion", "non_fast_forward", "required_signatures"} {
		if byType[kind] == nil {
			rules = append(rules, map[string]any{"type": kind})
		}
	}
	pr := byType["pull_request"]
	if pr == nil {
		pr = map[string]any{"type": "pull_request", "parameters": map[string]any{
			"dismiss_stale_reviews_on_push": false, "require_code_owner_review": false, "require_last_push_approval": false, "required_review_thread_resolution": false,
		}}
		rules = append(rules, pr)
	}
	parameters, ok := pr["parameters"].(map[string]any)
	if !ok {
		return fmt.Errorf("rulesets.main_protection: missing current pull_request parameters")
	}
	for _, key := range []string{"dismiss_stale_reviews_on_push", "require_code_owner_review", "require_last_push_approval", "required_review_thread_resolution"} {
		if _, err := required(parameters, key, false); err != nil {
			return fmt.Errorf("rulesets.main_protection: %w", err)
		}
	}
	if byType["pull_request"] != nil {
		count, ok := parameters["required_approving_review_count"].(float64)
		if !ok || count < 0 || count > 10 || math.Trunc(count) != count {
			return fmt.Errorf("rulesets.main_protection: missing or invalid current required_approving_review_count")
		}
		methods, ok := parameters["allowed_merge_methods"].([]any)
		if !ok || len(methods) == 0 {
			return fmt.Errorf("rulesets.main_protection: missing current allowed_merge_methods")
		}
		for _, method := range methods {
			if method != "merge" && method != "squash" && method != "rebase" {
				return fmt.Errorf("rulesets.main_protection: invalid current allowed_merge_methods")
			}
		}
	}
	parameters["required_approving_review_count"] = 0
	parameters["allowed_merge_methods"] = []any{"merge"}
	ruleOrder(rules)
	body["rules"] = rules
	if sameJSON(before, body) {
		return nil
	}
	p.Changes = append(p.Changes, Change{"rulesets.main_protection", before, body})
	p.Operations = append(p.Operations, Operation{method, endpoint, body})
	return nil
}
