package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	gh "github.com/uta8a/github-policy-cli/internal/github"
)

type testTransport func(*http.Request) (*http.Response, error)

func (t testTransport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }

type fixture struct {
	releases map[string]any
	security map[string]any
	forkPR   map[string]any
	ruleset  map[string]any
	repos    map[string]map[string]any
	actions  map[string]any
	workflow map[string]any
	requests []string
	payloads []map[string]any
	fail     string
}

func setup(t *testing.T) (*fixture, Factory) {
	t.Helper()
	f := &fixture{releases: map[string]any{"enabled": false, "enforced_by_owner": false}, security: map[string]any{"enabled": true, "paused": false}, forkPR: map[string]any{"approval_policy": "first_time_contributors"}, repos: map[string]map[string]any{}, actions: map[string]any{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": true}, workflow: map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}}
	for _, name := range []string{"me/a", "me/b"} {
		f.repos[name] = map[string]any{"full_name": name, "owner": map[string]any{"type": "User", "login": "me"}, "visibility": "public", "private": false, "has_projects": true, "allow_update_branch": false, "has_issues": true, "has_wiki": false, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": false, "pull_request_creation_policy": "all"}
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Method + " " + r.URL.Path
		f.requests = append(f.requests, key)
		if key == f.fail {
			w.WriteHeader(403)
			fmt.Fprint(w, `{"message":"secret-token"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/user" {
			fmt.Fprint(w, `{"login":"me","type":"User"}`)
			return
		}
		if r.URL.Path == "/user/repos" {
			json.NewEncoder(w).Encode([]any{f.repos["me/a"], f.repos["me/b"]})
			return
		}

		for _, feature := range []struct {
			suffix string
			value  map[string]any
		}{{"/immutable-releases", f.releases}, {"/automated-security-fixes", f.security}} {
			if strings.HasSuffix(r.URL.Path, feature.suffix) {
				if r.Method == "GET" {
					json.NewEncoder(w).Encode(feature.value)
					return
				}
				data := make([]byte, 1)
				n, _ := r.Body.Read(data)
				if n != 0 {
					t.Errorf("feature toggle must have an empty body")
				}
				feature.value["enabled"] = r.Method == "PUT"
				w.WriteHeader(204)
				return
			}
		}
		if strings.HasSuffix(r.URL.Path, "/rulesets") && r.Method == "GET" {
			rows := []any{}
			if f.ruleset != nil {
				rows = append(rows, f.ruleset)
			}
			json.NewEncoder(w).Encode(rows)
			return
		}
		if strings.Contains(r.URL.Path, "/rulesets") {
			if r.Method == "GET" {
				json.NewEncoder(w).Encode(f.ruleset)
				return
			}
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
				w.WriteHeader(400)
				return
			}
			f.payloads = append(f.payloads, body)
			f.ruleset = body
			f.ruleset["id"] = float64(123)
			f.ruleset["source_type"] = "Repository"
			f.ruleset["source"] = strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/repos/"), "/rulesets")
			if r.Method == "PUT" {
				f.ruleset["source"] = strings.Split(strings.TrimPrefix(r.URL.Path, "/repos/"), "/rulesets/")[0]
			}
			json.NewEncoder(w).Encode(f.ruleset)
			return
		}
		var current map[string]any
		switch {
		case strings.HasSuffix(r.URL.Path, "/actions/permissions/fork-pr-contributor-approval"):
			current = f.forkPR
		case strings.HasSuffix(r.URL.Path, "/actions/permissions/workflow"):
			current = f.workflow
		case strings.HasSuffix(r.URL.Path, "/actions/permissions"):
			current = f.actions
		default:
			current = f.repos[strings.TrimPrefix(r.URL.Path, "/repos/")]
		}
		if current == nil {
			w.WriteHeader(404)
			fmt.Fprint(w, `{}`)
			return
		}
		if r.Method == "GET" {
			json.NewEncoder(w).Encode(current)
			return
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		f.payloads = append(f.payloads, body)
		for k, v := range body {
			current[k] = v
		}
		if r.Method == "PUT" {
			w.WriteHeader(204)
		} else {
			json.NewEncoder(w).Encode(current)
		}
	}))
	t.Cleanup(server.Close)
	factory := func() (Client, error) {
		rest, err := api.NewRESTClient(api.ClientOptions{Host: "github.com", APIHost: "api.github.com", AuthToken: "secret-token", LogIgnoreEnv: true, Transport: testTransport(func(r *http.Request) (*http.Response, error) {
			r.URL.Scheme = "http"
			r.URL.Host = strings.TrimPrefix(server.URL, "http://")
			return http.DefaultTransport.RoundTrip(r)
		})})
		if err != nil {
			return nil, err
		}
		return gh.NewWithREST(rest), nil
	}
	return f, factory
}
func invoke(t *testing.T, f Factory, body string, args ...string) (string, string, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "policy.toml")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := New(f)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append(args, "--config", path))
	err := cmd.Execute()
	return out.String(), stderr.String(), err
}
func writes(requests []string) int {
	n := 0
	for _, r := range requests {
		if !strings.HasPrefix(r, "GET ") {
			n++
		}
	}
	return n
}
func TestPlanApplyIdempotency(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(fmt.Sprint(private), func(t *testing.T) {
			f, factory := setup(t)
			if private {
				f.repos["me/a"]["private"] = true
				f.repos["me/a"]["visibility"] = "private"
			}
			cfg := "[repository]\nhas_issues=false\ndelete_branch_on_merge=true\n[pull_requests]\ncreation_policy='collaborators_only'"
			out, _, err := invoke(t, factory, cfg, "plan", "me/a")
			if err != nil {
				t.Fatal(err)
			}
			if writes(f.requests) != 0 || !strings.Contains(out, "Plan: 3 to change.") {
				t.Fatalf("requests=%v out=%s", f.requests, out)
			}
			out, _, err = invoke(t, factory, cfg, "apply", "me/a")
			if err != nil {
				t.Fatal(err)
			}
			if writes(f.requests) != 1 || len(f.payloads[0]) != 3 || !strings.Contains(out, "1 succeeded, 0 failed") {
				t.Fatalf("requests=%v out=%s", f.requests, out)
			}
			if f.repos["me/a"]["allow_merge_commit"] != true {
				t.Fatal("unspecified setting changed")
			}
			out, _, err = invoke(t, factory, cfg, "apply", "me/a")
			if err != nil || !strings.Contains(out, "No changes.") || writes(f.requests) != 1 {
				t.Fatalf("second apply %s %v", out, err)
			}
			out, _, err = invoke(t, factory, cfg, "plan", "me/a")
			if err != nil || !strings.Contains(out, "No changes.") {
				t.Fatalf("subsequent plan %s %v", out, err)
			}
		})
	}
}
func TestAllPreflight(t *testing.T) {
	for _, verb := range []string{"plan", "apply"} {
		t.Run(verb, func(t *testing.T) {
			f, factory := setup(t)
			f.repos["me/b"]["private"] = true
			f.repos["me/b"]["visibility"] = "private"
			_, _, err := invoke(t, factory, "[repository]\nhas_wiki=true", verb, "--all")
			if err == nil || !strings.Contains(err.Error(), "me/b") || !strings.Contains(err.Error(), "has_wiki") || !strings.Contains(err.Error(), "policy.toml") || writes(f.requests) != 0 {
				t.Fatalf("err=%v requests=%v", err, f.requests)
			}
		})
	}
}
func TestApplyContinuesAfterFailure(t *testing.T) {
	f, factory := setup(t)
	f.fail = "PATCH /repos/me/a"
	out, stderr, err := invoke(t, factory, "[repository]\nhas_issues=false", "apply", "--all")
	if err == nil || f.repos["me/b"]["has_issues"] != false || !strings.Contains(stderr, "HTTP 403") || strings.Contains(stderr, "secret-token") || !strings.Contains(out, "1 succeeded, 1 failed") {
		t.Fatalf("out=%s stderr=%s err=%v", out, stderr, err)
	}
}
func TestReadFailurePreventsAllWrites(t *testing.T) {
	f, factory := setup(t)
	f.fail = "GET /repos/me/b"
	_, _, err := invoke(t, factory, "[repository]\nhas_issues=false", "apply", "--all")
	if err == nil || writes(f.requests) != 0 {
		t.Fatalf("requests=%v err=%v", f.requests, err)
	}
}
func TestValidationBeforeClient(t *testing.T) {
	calls := 0
	factory := func() (Client, error) { calls++; return nil, fmt.Errorf("should not be called") }
	_, _, err := invoke(t, factory, "[repository]\nunknown=true", "apply", "me/a")
	if err == nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
	for _, args := range [][]string{{"plan"}, {"apply", "me/a", "--all"}, {"plan", "me/a", "me/b"}, {"apply", "../me"}} {
		if _, _, err := invoke(t, factory, "", args...); err == nil {
			t.Fatalf("accepted args %v", args)
		}
	}
}
func TestActionsOnlyChangedGroups(t *testing.T) {
	f, factory := setup(t)
	out, _, err := invoke(t, factory, "[actions]\nallowed_actions='all'", "apply", "me/a")
	if err != nil {
		t.Fatal(err)
	}
	if writes(f.requests) != 1 || f.actions["enabled"] != true || f.actions["sha_pinning_required"] != true || strings.Contains(strings.Join(f.requests, ","), "/workflow") {
		t.Fatalf("requests=%v out=%s", f.requests, out)
	}
	_, _, err = invoke(t, factory, "[actions.workflow]\ncan_approve_pull_request_reviews=false", "apply", "me/a")
	if err != nil {
		t.Fatal(err)
	}
	if writes(f.requests) != 2 || f.workflow["default_workflow_permissions"] != "write" {
		t.Fatalf("requests=%v workflow=%v", f.requests, f.workflow)
	}
	_, _, err = invoke(t, factory, "[actions.workflow]\ncan_approve_pull_request_reviews=false", "apply", "me/a")
	if err != nil || writes(f.requests) != 2 {
		t.Fatal("workflow apply not idempotent")
	}
}
func TestMissingPRValueFailsClosed(t *testing.T) {
	f, factory := setup(t)
	delete(f.repos["me/a"], "pull_request_creation_policy")
	_, _, err := invoke(t, factory, "[pull_requests]\ncreation_policy='collaborators_only'", "apply", "me/a")
	if err == nil || !strings.Contains(err.Error(), "pull_requests.creation_policy") || writes(f.requests) != 0 {
		t.Fatalf("err=%v", err)
	}
}

func TestAllSupportedSettingsPublicPrivate(t *testing.T) {
	for _, private := range []bool{false, true} {
		t.Run(fmt.Sprint(private), func(t *testing.T) {
			f, factory := setup(t)
			if private {
				f.repos["me/a"]["private"] = true
				f.repos["me/a"]["visibility"] = "private"
			}
			cfg := `[repository]
has_issues = false
delete_branch_on_merge = true
allow_squash_merge = false
allow_merge_commit = false
allow_rebase_merge = true
[pull_requests]
creation_policy = "collaborators_only"
[actions]
enabled = true
allowed_actions = "local_only"
[actions.workflow]
default_workflow_permissions = "read"
can_approve_pull_request_reviews = false
`
			if !private {
				cfg = strings.Replace(cfg, "[repository]\n", "[repository]\nhas_wiki=true\n", 1)
			}
			out, _, err := invoke(t, factory, cfg, "apply", "me/a")
			if err != nil || writes(f.requests) != 3 || !strings.Contains(out, "3 succeeded") {
				t.Fatalf("out=%s err=%v requests=%v", out, err, f.requests)
			}
			out, _, err = invoke(t, factory, cfg, "plan", "me/a")
			if err != nil || !strings.Contains(out, "No changes.") {
				t.Fatalf("out=%s err=%v", out, err)
			}
		})
	}
}
func TestEnableActionsFailureSkipsWorkflow(t *testing.T) {
	f, factory := setup(t)
	f.actions["enabled"] = false
	f.fail = "PUT /repos/me/a/actions/permissions"
	out, stderr, err := invoke(t, factory, "[actions]\nenabled=true\n[actions.workflow]\ndefault_workflow_permissions='read'", "apply", "me/a")
	if err == nil || !strings.Contains(out, "1 failed, 1 skipped") || !strings.Contains(stderr, "workflow update skipped") || strings.Contains(strings.Join(f.requests, ","), "PUT /repos/me/a/actions/permissions/workflow") {
		t.Fatalf("out=%s stderr=%s err=%v requests=%v", out, stderr, err, f.requests)
	}
}
func TestIndependentAPIsContinue(t *testing.T) {
	f, factory := setup(t)
	f.fail = "PATCH /repos/me/a"
	out, _, err := invoke(t, factory, "[repository]\nhas_issues=false\n[actions.workflow]\ndefault_workflow_permissions='read'", "apply", "me/a")
	if err == nil || f.workflow["default_workflow_permissions"] != "read" || !strings.Contains(out, "1 succeeded, 1 failed") {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
func TestExplicitNonOwnedRepositoryRejected(t *testing.T) {
	for _, kind := range []string{"other", "organization"} {
		t.Run(kind, func(t *testing.T) {
			f, factory := setup(t)
			if kind == "other" {
				f.repos["me/a"]["owner"] = map[string]any{"login": "other", "type": "User"}
			} else {
				f.repos["me/a"]["owner"] = map[string]any{"login": "me", "type": "Organization"}
			}
			_, _, err := invoke(t, factory, "[repository]\nhas_issues=false", "apply", "me/a")
			if err == nil || writes(f.requests) != 0 {
				t.Fatalf("err=%v requests=%v", err, f.requests)
			}
		})
	}
}

func invokeDefault(t *testing.T, f Factory, args ...string) (string, string, error) {
	t.Helper()
	cmd := New(f)
	var out, stderr bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), stderr.String(), err
}
func TestMixedVisibilityDefaults(t *testing.T) {
	f, factory := setup(t)
	f.repos["me/a"]["private"] = true
	f.repos["me/a"]["visibility"] = "private"
	f.actions["sha_pinning_required"] = false
	out, _, err := invokeDefault(t, factory, "plan", "--all")
	if err != nil {
		t.Fatal(err)
	}
	if writes(f.requests) != 0 || !strings.Contains(out, "Manual setting required") || !strings.Contains(out, "rulesets.main_protection") {
		t.Fatalf("out=%s requests=%v", out, f.requests)
	}
	out, stderr, err := invokeDefault(t, factory, "apply", "--all")
	if err != nil {
		t.Fatalf("out=%s stderr=%s err=%v", out, stderr, err)
	}
	if writes(f.requests) != 10 || f.releases["enabled"] != true || f.security["enabled"] != false || f.actions["sha_pinning_required"] != true || f.repos["me/a"]["has_projects"] != false || f.repos["me/a"]["allow_merge_commit"] != true || f.repos["me/a"]["allow_squash_merge"] != false || f.repos["me/a"]["pull_request_creation_policy"] != "collaborators_only" || f.forkPR["approval_policy"] != "all_external_contributors" {
		t.Fatalf("out=%s writes=%d", out, writes(f.requests))
	}
	if f.repos["me/b"]["has_projects"] != false || f.repos["me/b"]["pull_request_creation_policy"] != "collaborators_only" || f.repos["me/b"]["allow_squash_merge"] != false || f.repos["me/b"]["allow_update_branch"] != true {
		t.Fatal("public did not inherit common settings")
	}
	out, _, err = invokeDefault(t, factory, "apply", "--all")
	if err != nil || writes(f.requests) != 10 || strings.Count(out, "No changes.") != 2 {
		t.Fatalf("non-idempotent defaults: out=%s err=%v requests=%v", out, err, f.requests)
	}
}
func TestPrivateRulesetPreflight(t *testing.T) {
	for _, verb := range []string{"plan", "apply"} {
		f, factory := setup(t)
		f.repos["me/a"]["private"] = true
		f.repos["me/a"]["visibility"] = "private"
		_, _, err := invoke(t, factory, "[rulesets]\nmain_protection=true", verb, "--all")
		if err == nil || !strings.Contains(err.Error(), "GitHub Free") || writes(f.requests) != 0 || strings.Contains(strings.Join(f.requests, ","), "/repos/me/a/rulesets") {
			t.Fatalf("err=%v requests=%v", err, f.requests)
		}
	}
}
func TestDefaultPreflightMissingNewValue(t *testing.T) {
	f, factory := setup(t)
	f.repos["me/a"]["private"] = true
	f.repos["me/a"]["visibility"] = "private"
	delete(f.repos["me/a"], "allow_update_branch")
	_, _, err := invokeDefault(t, factory, "apply", "--all")
	if err == nil || writes(f.requests) != 0 {
		t.Fatalf("err=%v requests=%v", err, f.requests)
	}
}
func TestRulesetSkippedAfterMergeEnableFailure(t *testing.T) {
	f, factory := setup(t)
	f.repos["me/a"]["allow_merge_commit"] = false
	f.fail = "PATCH /repos/me/a"
	out, stderr, err := invokeDefault(t, factory, "apply", "me/a")
	if err == nil || !strings.Contains(out, "3 succeeded, 1 failed, 1 skipped") || !strings.Contains(stderr, "ruleset update skipped") {
		t.Fatalf("out=%s stderr=%s err=%v", out, stderr, err)
	}
}

func TestPublicDefaultsIncludeCommonSettings(t *testing.T) {
	f, factory := setup(t)
	f.actions["sha_pinning_required"] = false
	out, _, err := invokeDefault(t, factory, "plan", "me/a")
	if err != nil || writes(f.requests) != 0 || !strings.Contains(out, "Manual setting required") {
		t.Fatalf("out=%s err=%v", out, err)
	}
	out, _, err = invokeDefault(t, factory, "apply", "me/a")
	if err != nil || writes(f.requests) != 6 {
		t.Fatalf("out=%s err=%v requests=%v", out, err, f.requests)
	}
	r := f.repos["me/a"]
	if r["has_projects"] != false || r["allow_update_branch"] != true || r["delete_branch_on_merge"] != true || r["allow_squash_merge"] != false || r["allow_rebase_merge"] != false || r["allow_merge_commit"] != true || r["pull_request_creation_policy"] != "collaborators_only" || f.releases["enabled"] != true || f.security["enabled"] != false || f.actions["sha_pinning_required"] != true || f.forkPR["approval_policy"] != "all_external_contributors" || f.ruleset == nil {
		t.Fatal("public preset omitted a common or public-only setting")
	}
	out, _, err = invokeDefault(t, factory, "apply", "me/a")
	if err != nil || writes(f.requests) != 6 || !strings.Contains(out, "No changes.") {
		t.Fatalf("out=%s err=%v", out, err)
	}
}
