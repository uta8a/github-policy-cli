package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestDisabledFeature404RequiresAdministrationRead(t *testing.T) {
	for _, feature := range []string{"immutable-releases", "automated-security-fixes"} {
		for _, status := range []int{200, 403, 404} {
			t.Run(fmt.Sprintf("%s/%d", feature, status), func(t *testing.T) {
				calls := 0
				c := mock(t, func(w http.ResponseWriter, r *http.Request) {
					calls++
					if strings.HasSuffix(r.URL.Path, "/actions/permissions") {
						w.WriteHeader(status)
						fmt.Fprint(w, `{"enabled":true}`)
						return
					}
					w.WriteHeader(404)
					fmt.Fprint(w, `{"message":"Not Found"}`)
				})
				got, err := c.readFeature(context.Background(), "me/repo", feature)
				if status == 200 {
					if err != nil || got["enabled"] != false {
						t.Fatalf("got=%v err=%v", got, err)
					}
				} else if err == nil {
					t.Fatal("ambiguous 404 accepted without Administration read")
				}
				if calls != 2 {
					t.Fatalf("calls=%d", calls)
				}
			})
		}
	}
}
func TestRulesetPaginationAndDetail(t *testing.T) {
	calls := 0
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Query().Get("includes_parents") != "false" {
			t.Errorf("query=%s", r.URL.RawQuery)
		}
		if strings.HasSuffix(r.URL.Path, "/rulesets/42") {
			fmt.Fprint(w, `{"id":42,"name":"main","target":"branch","source_type":"Repository","source":"me/repo"}`)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<https://example.invalid>; rel="next"`)
			fmt.Fprint(w, `[{"id":1,"name":"other"}]`)
			return
		}
		fmt.Fprint(w, `[{"id":42,"name":"main","source_type":"Repository","source":"me/repo"}]`)
	})
	got, err := c.namedRuleset(context.Background(), "me/repo", "main", "rulesets.main_protection")
	if err != nil || got["id"] != float64(42) || calls != 3 {
		t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
	}
}
func TestRulesetDiscoveryFailsClosed(t *testing.T) {
	for _, body := range []string{`null`, `[null]`, `[{}]`, `[{"id":1,"name":"main","source_type":"Organization","source":"org"}]`, `[{"id":1,"name":"main","source_type":"Repository","source":"me/repo"},{"id":2,"name":"main","source_type":"Repository","source":"me/repo"}]`} {
		c := mock(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		if _, err := c.namedRuleset(context.Background(), "me/repo", "main", "rulesets.main_protection"); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestReleaseTagRulesetDiscovery(t *testing.T) {
	calls := 0
	c := mock(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if strings.HasSuffix(r.URL.Path, "/rulesets/7") {
			fmt.Fprint(w, `{"id":7,"name":"release-tags","target":"tag","source_type":"Repository","source":"me/repo"}`)
			return
		}
		if r.URL.Query().Get("page") == "1" {
			w.Header().Set("Link", `<https://ignored.invalid>; rel="next"`)
			fmt.Fprint(w, `[{"id":1,"name":"main"}]`)
			return
		}
		fmt.Fprint(w, `[{"id":7,"name":"release-tags","source_type":"Repository","source":"me/repo"}]`)
	})
	got, err := c.namedRuleset(context.Background(), "me/repo", "release-tags", "rulesets.release_tag_protection")
	if err != nil || got["id"] != float64(7) || calls != 3 {
		t.Fatalf("got=%v err=%v calls=%d", got, err, calls)
	}
}
