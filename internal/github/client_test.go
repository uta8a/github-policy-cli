package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/uta8a/github-policy-cli/internal/policy"
)

type transport func(*http.Request) (*http.Response, error)

func (t transport) RoundTrip(r *http.Request) (*http.Response, error) { return t(r) }
func mock(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	server := httptest.NewServer(h)
	t.Cleanup(server.Close)
	rest, err := api.NewRESTClient(api.ClientOptions{Host: "github.com", APIHost: "api.github.com", AuthToken: "test-secret", LogIgnoreEnv: true, Transport: transport(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme = "http"
		r.URL.Host = strings.TrimPrefix(server.URL, "http://")
		return http.DefaultTransport.RoundTrip(r)
	})})
	if err != nil {
		t.Fatal(err)
	}
	return NewWithREST(rest)
}
func TestDiscoveryPagination(t *testing.T) {
	pages := 0
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			t.Errorf("unexpected write")
		}
		if r.URL.Path == "/user" {
			fmt.Fprint(w, `{"login":"me","type":"User"}`)
			return
		}
		pages++
		if r.URL.Query().Get("affiliation") != "owner" || r.URL.Query().Get("visibility") != "all" || r.URL.Query().Get("per_page") != "100" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<https://evil.example/?page=2>; rel="next"`)
			fmt.Fprint(w, `[
   {"full_name":"me/public","visibility":"public","owner":{"type":"User","login":"me"}},
   {"full_name":"org/repo","visibility":"public","owner":{"type":"Organization","login":"org"}},
   {"full_name":"other/repo","visibility":"private","private":true,"owner":{"type":"User","login":"other"}}
  ]`)
		} else {
			fmt.Fprint(w, `[{"full_name":"me/private","visibility":"private","private":true,"owner":{"type":"User","login":"me"}}]`)
		}
	})
	login, err := c.Identity(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	names, err := c.Discover(context.Background(), login)
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || strings.Join(names, ",") != "me/private,me/public" {
		t.Fatalf("pages=%d names=%v", pages, names)
	}
}

func TestHasNextPage(t *testing.T) {
	for _, test := range []struct {
		name string
		link string
		want bool
	}{
		{name: "missing"},
		{name: "last page", link: `<https://api.github.com/resource?page=1>; rel="prev"`},
		{name: "next", link: `<https://api.github.com/resource?page=2>; rel="next", <https://api.github.com/resource?page=4>; rel="last"`, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			header := http.Header{}
			header.Set("Link", test.link)
			if got := hasNextPage(header); got != test.want {
				t.Fatalf("hasNextPage()=%v want=%v", got, test.want)
			}
		})
	}
}

func TestDiscoveryStopsWithoutNextLinkAtFullPage(t *testing.T) {
	requests := 0
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		requests++
		rows := make([]map[string]any, 100)
		for i := range rows {
			rows[i] = map[string]any{
				"full_name":  fmt.Sprintf("me/repo-%d", i),
				"visibility": "public",
				"owner":      map[string]any{"type": "User", "login": "me"},
			}
		}
		json.NewEncoder(w).Encode(rows)
	})
	names, err := c.Discover(context.Background(), "me")
	if err != nil || requests != 1 || len(names) != 100 {
		t.Fatalf("requests=%d names=%d err=%v", requests, len(names), err)
	}
}

func TestSafeErrors(t *testing.T) {
	for _, status := range []int{401, 403, 404, 409, 422, 429, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			c := mock(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(status)
				fmt.Fprint(w, `{"message":"test-secret"}`)
			})
			_, err := c.Identity(context.Background())
			if err == nil || !strings.Contains(err.Error(), fmt.Sprint(status)) || strings.Contains(err.Error(), "test-secret") {
				t.Fatalf("err=%v", err)
			}
		})
	}
}
func TestNonGitHubHost(t *testing.T) {
	t.Setenv("GH_HOST", "enterprise.example")
	if _, err := New(); err == nil {
		t.Fatal("enterprise accepted")
	}
}
func TestFetchPrivateWikiBeforeActionsRead(t *testing.T) {
	calls := 0
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		fmt.Fprint(w, `{"full_name":"me/repo","owner":{"type":"User","login":"me"},"visibility":"private","private":true}`)
	})
	value := false
	cfg := policy.Config{}
	cfg.Repository.HasWiki = &value
	cfg.Actions.Enabled = &value
	_, _, _, err := c.Fetch(context.Background(), "me/repo", "me", cfg)
	if err == nil || !strings.Contains(err.Error(), "has_wiki") || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
func TestExecutePayload(t *testing.T) {
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/repos/me/repo/actions/permissions" {
			t.Errorf("%s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if len(body) != 2 || body["enabled"] != false || body["allowed_actions"] != "selected" {
			t.Errorf("body=%v", body)
		}
		w.WriteHeader(204)
	})
	if err := c.Execute(context.Background(), policy.Operation{Method: "PUT", Endpoint: "repos/me/repo/actions/permissions", Body: map[string]any{"enabled": false, "allowed_actions": "selected"}}); err != nil {
		t.Fatal(err)
	}
}
func TestInvalidName(t *testing.T) {
	for _, name := range []string{"me/repo/extra", "me/..", "me/repo?x=1", "https://evil.example/x", "/repo", "me/"} {
		if ValidName(name) {
			t.Errorf("accepted %s", name)
		}
	}
}
