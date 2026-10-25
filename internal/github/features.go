package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/cli/go-gh/v2/pkg/api"
	"github.com/uta8a/github-policy-cli/internal/policy"
)

// Only these documented feature-check endpoints use 404 for disabled. Verify
// Administration read access separately before interpreting that ambiguous status.
func (c *Client) readFeature(ctx context.Context, name, feature string) (map[string]any, error) {
	path := "repos/" + name + "/" + feature
	resp, err := c.rest.RequestWithContext(ctx, "GET", path, nil)
	if err != nil {
		var h *api.HTTPError
		if errors.As(err, &h) && h.StatusCode == 404 {
			var permissions map[string]any
			if _, checkErr := c.get(ctx, "repos/"+name+"/actions/permissions", &permissions); checkErr != nil {
				return nil, fmt.Errorf("%s: disabled status could not be distinguished from missing Administration access: %w", feature, checkErr)
			}
			if _, ok := permissions["enabled"].(bool); !ok {
				return nil, fmt.Errorf("%s: Administration permission check returned an invalid response", feature)
			}
			return map[string]any{"enabled": false}, nil
		}
		return nil, safeError("GET", path, err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if json.NewDecoder(resp.Body).Decode(&result) != nil {
		return nil, fmt.Errorf("GET %s: invalid JSON response", path)
	}
	return result, nil
}
func (c *Client) mainRuleset(ctx context.Context, name string) (map[string]any, error) {
	var found map[string]any
	for page := 1; ; page++ {
		var rows []map[string]any
		path := fmt.Sprintf("repos/%s/rulesets?includes_parents=false&per_page=100&page=%d", name, page)
		headers, err := c.get(ctx, path, &rows)
		if err != nil {
			return nil, err
		}
		if rows == nil {
			return nil, fmt.Errorf("rulesets.main_protection: invalid null ruleset list")
		}
		for _, row := range rows {
			if row["name"] != "main" {
				continue
			}
			if row["source_type"] != "Repository" || row["source"] != name {
				return nil, fmt.Errorf("rulesets.main_protection: main ruleset has an unexpected source")
			}
			if found != nil {
				return nil, fmt.Errorf("rulesets.main_protection: multiple repository rulesets are named main")
			}
			found = row
		}
		if !strings.Contains(headers.Get("Link"), `rel="next"`) && len(rows) < 100 {
			break
		}
	}
	if found == nil {
		return nil, nil
	}
	id, err := policy.RulesetID(found["id"])
	if err != nil {
		return nil, err
	}
	var detail map[string]any
	if _, err = c.get(ctx, "repos/"+name+"/rulesets/"+id+"?includes_parents=false", &detail); err != nil {
		return nil, err
	}
	if detail["id"] != found["id"] || detail["name"] != "main" || detail["source_type"] != "Repository" || detail["source"] != name {
		return nil, fmt.Errorf("rulesets.main_protection: ruleset identity changed or missing")
	}
	return detail, nil
}
