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
	repos    map[string]map[string]any
	actions  map[string]any
	workflow map[string]any
	requests []string
	payloads []map[string]any
	fail     string
}

func setup(t *testing.T) (*fixture, Factory) {
	t.Helper()
	f := &fixture{repos: map[string]map[string]any{}, actions: map[string]any{"enabled": true, "allowed_actions": "selected", "sha_pinning_required": true}, workflow: map[string]any{"default_workflow_permissions": "write", "can_approve_pull_request_reviews": true}}
	for _, name := range []string{"me/a", "me/b"} {
		f.repos[name] = map[string]any{"full_name": name, "owner": map[string]any{"type": "User", "login": "me"}, "visibility": "public", "private": false, "has_issues": true, "has_wiki": false, "delete_branch_on_merge": false, "allow_squash_merge": true, "allow_merge_commit": true, "allow_rebase_merge": false, "pull_request_creation_policy": "all"}
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
		var current map[string]any
		switch {
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
