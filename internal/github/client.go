package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/uta8a/github-policy-cli/internal/policy"
)

type Client struct{ rest *api.RESTClient }

var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9-]*/[A-Za-z0-9_.-]+$`)

func ValidName(name string) bool {
	return namePattern.MatchString(name) && !strings.HasSuffix(name, "/.") && !strings.HasSuffix(name, "/..")
}
func New() (*Client, error) {
	if host := os.Getenv("GH_HOST"); host != "" && host != "github.com" {
		return nil, fmt.Errorf("GH_HOST: only github.com is supported")
	}
	rest, err := api.NewRESTClient(api.ClientOptions{
		Host: "github.com", APIHost: "api.github.com", Transport: http.DefaultTransport,
		Timeout: 30 * time.Second, EnableCache: false, LogIgnoreEnv: true,
		Headers:       map[string]string{"Accept": "application/vnd.github+json", "X-GitHub-Api-Version": "2026-03-10"},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
	})
	if err != nil {
		return nil, fmt.Errorf("GitHub authentication unavailable: set GH_TOKEN or run gh auth login for github.com")
	}
	return &Client{rest}, nil
}

// NewWithREST allows HTTP mocks without adding a runtime host override.
func NewWithREST(rest *api.RESTClient) *Client { return &Client{rest} }

func safeError(method, path string, err error) error {
	var h *api.HTTPError
	if errors.As(err, &h) {
		reason := "GitHub rejected request"
		switch h.StatusCode {
		case 401:
			reason = "authentication failed"
		case 403:
			reason = "permission denied, rate limited, or feature unavailable"
		case 404:
			reason = "repository/setting not found or token lacks access"
		case 409:
			reason = "conflict or setting restricted"
		case 422:
			reason = "GitHub validation failed"
		case 429:
			reason = "rate limited"
		}
		// Do not echo API bodies, which can contain arbitrary strings or credentials.
		return fmt.Errorf("%s %s: HTTP %d (%s)", method, path, h.StatusCode, reason)
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("%s %s: canceled", method, path)
	}
	return fmt.Errorf("%s %s: GitHub transport or response error", method, path)
}
func (c *Client) get(ctx context.Context, path string, out any) (http.Header, error) {
	resp, err := c.rest.RequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		return nil, safeError("GET", path, err)
	}
	defer resp.Body.Close()
	if err = json.NewDecoder(resp.Body).Decode(out); err != nil {
		return nil, fmt.Errorf("GET %s: invalid or missing JSON response", path)
	}
	return resp.Header, nil
}
func (c *Client) Identity(ctx context.Context) (string, error) {
	var u struct {
		Login string `json:"login"`
		Type  string `json:"type"`
	}
	if _, err := c.get(ctx, "user", &u); err != nil {
		return "", err
	}
	if u.Type != "User" || u.Login == "" {
		return "", fmt.Errorf("authenticated identity must be a personal account")
	}
	return u.Login, nil
}
func (c *Client) Discover(ctx context.Context, login string) ([]string, error) {
	names := map[string]bool{}
	for page := 1; ; page++ {
		var repos []policy.Target
		path := fmt.Sprintf("user/repos?affiliation=owner&visibility=all&sort=full_name&direction=asc&per_page=100&page=%d", page)
		header, err := c.get(ctx, path, &repos)
		if err != nil {
			return nil, err
		}
		for _, r := range repos {
			if r.OwnedBy(login) && (r.Visibility == "public" || r.Visibility == "private") && ValidName(r.FullName) {
				names[r.FullName] = true
			}
		}
		// Page numbers are generated locally: never send a token to a Link URL.
		if !strings.Contains(header.Get("Link"), `rel="next"`) && len(repos) < 100 {
			break
		}
	}
	out := make([]string, 0, len(names))
	for name := range names {
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}
func (c *Client) Fetch(ctx context.Context, name, login string, cfg policy.Config) (policy.Target, policy.State, error) {
	var t policy.Target
	s := policy.State{}
	if !ValidName(name) {
		return t, s, fmt.Errorf("invalid repository: expected owner/repo")
	}
	if _, err := c.get(ctx, "repos/"+name, &s.Repository); err != nil {
		return t, s, err
	}
	raw, err := json.Marshal(s.Repository)
	if err != nil {
		return t, s, fmt.Errorf("%s: invalid repository metadata", name)
	}
	if err = json.Unmarshal(raw, &t); err != nil {
		return t, s, fmt.Errorf("%s: invalid repository metadata", name)
	}
	if !strings.EqualFold(t.FullName, name) || !strings.EqualFold(strings.Split(name, "/")[0], t.Owner.Login) {
		return t, s, fmt.Errorf("%s: repository identity changed or missing; use its current owner/name", name)
	}
	cfg, err = cfg.ForTarget(t)
	if err != nil {
		return t, s, err
	}
	if err = t.Validate(cfg, login); err != nil {
		return t, s, err
	}
	if cfg.NeedsActions() {
		if _, err = c.get(ctx, "repos/"+name+"/actions/permissions", &s.Actions); err != nil {
			return t, s, err
		}
	}
	if cfg.NeedsWorkflow() {
		if _, err = c.get(ctx, "repos/"+name+"/actions/permissions/workflow", &s.Workflow); err != nil {
			return t, s, err
		}
	}

	if cfg.Actions.ForkPRApproval != nil {
		if _, err = c.get(ctx, "repos/"+name+"/actions/permissions/fork-pr-contributor-approval", &s.ForkPR); err != nil {
			return t, s, err
		}
	}
	if cfg.Releases.Immutable != nil {
		s.Releases, err = c.readFeature(ctx, name, "immutable-releases")
		if err != nil {
			return t, s, err
		}
	}
	if cfg.Security.DependabotUpdates != nil {
		s.Security, err = c.readFeature(ctx, name, "automated-security-fixes")
		if err != nil {
			return t, s, err
		}
	}
	if cfg.Rulesets.MainProtection != nil {
		s.MainRuleset, err = c.mainRuleset(ctx, t.FullName)
		if err != nil {
			return t, s, err
		}
		s.RulesetsKnown = true
	}
	return t, s, nil
}
func (c *Client) Execute(ctx context.Context, op policy.Operation) error {
	var body io.Reader
	if op.Body != nil {
		data, err := json.Marshal(op.Body)
		if err != nil {
			return fmt.Errorf("invalid operation payload")
		}
		body = bytes.NewReader(data)
	}
	resp, err := c.rest.RequestWithContext(ctx, op.Method, op.Endpoint, body)
	if err != nil {
		return safeError(op.Method, op.Endpoint, err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("%s %s: response read failed; update outcome unknown", op.Method, op.Endpoint)
	}
	return nil
}
