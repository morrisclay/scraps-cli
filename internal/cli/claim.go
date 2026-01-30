package cli

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/spf13/cobra"

	"github.com/scraps-sh/scraps-cli/internal/api"
	"github.com/scraps-sh/scraps-cli/internal/config"
	"github.com/scraps-sh/scraps-cli/internal/model"
)

func newClaimCmd() *cobra.Command {
	var message, agentID string
	var ttl int

	cmd := &cobra.Command{
		Use:   "claim <store/repo:branch> <patterns...>",
		Short: "Claim file patterns for exclusive access",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, branch, err := parseStoreRepoBranch(args[0])
			if err != nil {
				return err
			}

			if branch == "" {
				return fmt.Errorf("branch is required (use store/repo:branch format)")
			}

			patterns := args[1:]

			// Generate agent ID if not provided
			if agentID == "" {
				agentID = "cli-" + uuid.New().String()[:8]
			}

			if message == "" {
				message = "CLI claim"
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			req := model.ClaimRequest{
				AgentID:    agentID,
				Patterns:   patterns,
				Claim:      message,
				TTLSeconds: ttl,
			}

			resp, err := client.Claim(store, repo, branch, req)
			if err != nil {
				return err
			}

			// Check for conflicts
			if resp.Type == "claim_conflict" && len(resp.Conflicts) > 0 {
				errorf("Claim conflict detected!")
				fmt.Println("\nConflicting claims:")
				for _, c := range resp.Conflicts {
					fmt.Printf("  Agent: %s (%s)\n", c.AgentName, c.AgentID)
					fmt.Printf("  Patterns: %v\n", c.Patterns)
					fmt.Printf("  Claim: %s\n\n", c.Claim)
				}
				return fmt.Errorf("cannot claim: patterns conflict with existing claims")
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(map[string]any{
					"agent_id":   agentID,
					"patterns":   patterns,
					"expires_at": resp.ExpiresAt,
				})
			} else {
				success(fmt.Sprintf("Claimed patterns as %s", agentID))
				fmt.Printf("Patterns: %v\n", patterns)
				if resp.ExpiresAt != nil {
					fmt.Printf("Expires: %s\n", *resp.ExpiresAt)
				}
				info(fmt.Sprintf("Use --agent-id %s to release", agentID))
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Claim description")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Agent ID (auto-generated if not provided)")
	cmd.Flags().IntVar(&ttl, "ttl", 300, "Claim TTL in seconds")

	return cmd
}

func newReleaseCmd() *cobra.Command {
	var agentID string

	cmd := &cobra.Command{
		Use:   "release <store/repo:branch> <patterns...>",
		Short: "Release claimed file patterns",
		Args:  cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, branch, err := parseStoreRepoBranch(args[0])
			if err != nil {
				return err
			}

			if branch == "" {
				return fmt.Errorf("branch is required (use store/repo:branch format)")
			}

			if agentID == "" {
				return fmt.Errorf("--agent-id is required")
			}

			patterns := args[1:]

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			req := model.ReleaseRequest{
				AgentID:  agentID,
				Patterns: patterns,
			}

			if err := client.Release(store, repo, branch, req); err != nil {
				return err
			}

			success(fmt.Sprintf("Released patterns as %s", agentID))
			return nil
		},
	}

	cmd.Flags().StringVar(&agentID, "agent-id", "", "Agent ID from the claim")
	cmd.MarkFlagRequired("agent-id")

	return cmd
}
