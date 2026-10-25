package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/uta8a/github-policy-cli/internal/github"
	"github.com/uta8a/github-policy-cli/internal/policy"
)

type Client interface {
	Identity(context.Context) (string, error)
	Discover(context.Context, string) ([]string, error)
	Fetch(context.Context, string, string, policy.Config) (policy.Target, policy.State, error)
	Execute(context.Context, policy.Operation) error
}
type Factory func() (Client, error)

func New(factory Factory) *cobra.Command {
	if factory == nil {
		factory = func() (Client, error) { return github.New() }
	}
	root := &cobra.Command{Version: "dev", Use: "repo-policy", Short: "Plan and apply GitHub Free personal repository settings", SilenceUsage: true, SilenceErrors: true}
	for _, verb := range []string{"plan", "apply"} {
		var configPath string
		var all bool
		command := &cobra.Command{Use: verb + " [owner/repo]", Short: verb + " repository policy", Args: func(cmd *cobra.Command, args []string) error {
			if (all && len(args) != 0) || (!all && len(args) != 1) {
				return fmt.Errorf("specify either one owner/repo or --all")
			}
			if !all && !github.ValidName(args[0]) {
				return fmt.Errorf("invalid repository: expected owner/repo")
			}
			return nil
		}}
		apply := verb == "apply"
		command.Flags().StringVar(&configPath, "config", "", "Explicit TOML policy (otherwise use personal Public/Private defaults)")
		command.Flags().BoolVar(&all, "all", false, "Target own public and private repositories visible to the token")
		command.RunE = func(cmd *cobra.Command, args []string) error {
			cfg := policy.PersonalDefaults()
			path := "built-in personal defaults"
			if cmd.Flags().Changed("config") {
				var err error
				cfg, err = policy.Load(configPath)
				if err != nil {
					return err
				}
				path = configPath
			}
			client, err := factory()
			if err != nil {
				return err
			}
			return run(cmd.Context(), cmd.OutOrStdout(), cmd.ErrOrStderr(), client, cfg, path, args, all, apply)
		}
		root.AddCommand(command)
	}
	return root
}
func render(w io.Writer, p policy.Plan) {
	fmt.Fprintf(w, "Repository: %s\n\n", p.Repository)
	for _, note := range p.Notes {
		fmt.Fprintf(w, "Note: %s\n\n", note)
	}
	if len(p.Changes) == 0 {
		fmt.Fprintln(w, "No changes.")
		return
	}
	for _, c := range p.Changes {
		fmt.Fprintf(w, "%s\n- %v\n+ %v\n\n", c.Path, displayValue(c.Before), displayValue(c.After))
	}
	fmt.Fprintf(w, "Plan: %d to change.\n", len(p.Changes))
}
func run(ctx context.Context, out, errOut io.Writer, client Client, cfg policy.Config, path string, names []string, all, apply bool) error {
	login, err := client.Identity(ctx)
	if err != nil {
		return err
	}
	if all {
		names, err = client.Discover(ctx, login)
		if err != nil {
			return err
		}
	}
	plans := make([]policy.Plan, 0, len(names))
	var preflight []error
	for _, name := range names {
		target, state, fetchErr := client.Fetch(ctx, name, login, cfg)
		if fetchErr == nil {
			resolved, resolveErr := cfg.ForTarget(target)
			if resolveErr != nil {
				fetchErr = resolveErr
			} else {
				fetchErr = target.Validate(resolved, login)
			}
		}
		var p policy.Plan
		if fetchErr == nil {
			p, fetchErr = policy.Build(cfg, target, state)
		}
		if fetchErr != nil {
			preflight = append(preflight, fmt.Errorf("%s: %s: %w", path, name, fetchErr))
			continue
		}
		plans = append(plans, p)
	}
	if len(preflight) > 0 {
		return fmt.Errorf("preflight failed; no changes applied:\n%w", errors.Join(preflight...))
	}
	if len(plans) == 0 {
		fmt.Fprintln(out, "No changes. (No owned repositories visible to this token.)")
	}
	for _, p := range plans {
		render(out, p)
	}
	if !apply {
		return nil
	}
	success, failed, skipped := 0, 0, 0
	for _, p := range plans {
		enabling := false
		for _, change := range p.Changes {
			if change.Path == "actions.enabled" && change.Before == false && change.After == true {
				enabling = true
			}
		}
		enableFailed := false
		mergeFailed := false
		mergeEnabling := false
		for _, change := range p.Changes {
			if change.Path == "repository.allow_merge_commit" && change.Before == false && change.After == true {
				mergeEnabling = true
			}
		}
		for _, op := range p.Operations {
			if mergeFailed && strings.HasPrefix(op.Endpoint, "repos/"+p.Repository+"/rulesets") {
				skipped++
				fmt.Fprintf(errOut, "%s: ruleset update skipped because enabling merge commits failed\n", p.Repository)
				continue
			}
			if enableFailed && (op.Endpoint == "repos/"+p.Repository+"/actions/permissions/workflow" || op.Endpoint == "repos/"+p.Repository+"/actions/permissions/fork-pr-contributor-approval") {
				skipped++
				fmt.Fprintf(errOut, "%s: workflow update skipped because enabling Actions failed\n", p.Repository)
				continue
			}
			if err := client.Execute(ctx, op); err != nil {
				failed++
				fmt.Fprintf(errOut, "%s: %v\n", p.Repository, err)
				if mergeEnabling && op.Method == "PATCH" && op.Endpoint == "repos/"+p.Repository {
					mergeFailed = true
				}
				// A failed Actions enable must not be followed by dependent workflow writes.
				if enabling && op.Endpoint == "repos/"+p.Repository+"/actions/permissions" {
					enableFailed = true
				}
				continue
			}
			success++
		}
	}
	fmt.Fprintf(out, "Apply: %d succeeded, %d failed, %d skipped (API operations).\n", success, failed, skipped)
	if failed > 0 {
		return fmt.Errorf("apply failed for %d API operation(s); see errors above", failed)
	}
	return nil
}

func displayValue(value any) string {
	switch value.(type) {
	case map[string]any, []any:
		data, _ := json.MarshalIndent(value, "", "  ")
		return string(data)
	}
	return fmt.Sprint(value)
}
