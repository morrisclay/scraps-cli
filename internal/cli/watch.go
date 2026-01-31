package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/stream"
)

func newWatchCmd() *cobra.Command {
	var branch, path string

	cmd := &cobra.Command{
		Use:   "watch <store/repo[:branch]>",
		Short: "Watch repository events in real-time",
		Long: `Watch repository events in real-time.

Examples:
  # Watch all events
  scraps watch mystore/myrepo

  # Watch specific branch
  scraps watch mystore/myrepo:main

  # Watch specific file path
  scraps watch mystore/myrepo --path src/auth.ts

  # Watch files matching a glob pattern
  scraps watch mystore/myrepo --path "src/**/*.ts"

  # Combine branch and path filters
  scraps watch mystore/myrepo:main --path "src/**"`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("repository reference required\n\nUsage: scraps watch <store/repo[:branch]>\n\nExample: scraps watch mystore/myrepo")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			store, repo, parsedBranch, err := parseStoreRepoBranch(args[0])
			if err != nil {
				return err
			}

			if parsedBranch != "" {
				branch = parsedBranch
			}

			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			return runWatch(client, store, repo, branch, path)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter to specific branch")
	cmd.Flags().StringVarP(&path, "path", "p", "", "Filter to specific path or glob pattern (e.g., \"src/**/*.ts\")")

	return cmd
}

func runWatch(client *api.Client, store, repo, branch, path string) error {
	info(fmt.Sprintf("Watching %s/%s", store, repo))
	if branch != "" {
		fmt.Printf("Branch: %s\n", branch)
	}
	if path != "" {
		fmt.Printf("Path: %s\n", path)
	}

	// Fetch and display recent historical events
	events, err := client.GetRecentStreamEvents(store, repo, 20)
	if err != nil {
		errorf("Failed to fetch historical events: %v", err)
	} else if len(events) > 0 {
		fmt.Printf("\n--- Recent events (%d) ---\n", len(events))
		for i := len(events) - 1; i >= 0; i-- {
			printEvent(events[i])
		}
		fmt.Println("--- Live events ---")
	} else {
		fmt.Println("(no recent events)")
	}

	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	opts := &api.StreamOptions{Branch: branch, Path: path}
	streamURL := client.BuildStreamURL(store, repo, opts)

	// Auto-reconnect loop
	for {
		streamClient := stream.NewClient(streamURL, client.APIKey())

		streamClient.OnMessage = func(data []byte) {
			var msg map[string]any
			if json.Unmarshal(data, &msg) == nil {
				printEvent(msg)
			} else {
				fmt.Println(string(data))
			}
		}

		streamClient.OnError = func(err error) {
			// Don't print EOF errors, just reconnect silently
			if err.Error() != "EOF" {
				errorf("Stream error: %v", err)
			}
		}

		if err := streamClient.Connect(); err != nil {
			errorf("Connection failed: %v, retrying...", err)
			time.Sleep(2 * time.Second)
			continue
		}

		// Wait for connection to close
		<-streamClient.Done()
		streamClient.Close()

		// Reconnect after a brief pause
		time.Sleep(500 * time.Millisecond)
	}
}

func printEvent(event map[string]any) {
	eventType, _ := event["type"].(string)
	agentID, _ := event["agent_id"].(string)

	// Compact format for common events
	switch eventType {
	case "agent_join":
		role, _ := event["role"].(string)
		fmt.Printf("  [%s] %s joined (%s)\n", eventType, agentID, role)
	case "agent_leave":
		role, _ := event["role"].(string)
		fmt.Printf("  [%s] %s left (%s)\n", eventType, agentID, role)
	case "agent_claim":
		patterns, _ := event["patterns"].([]any)
		fmt.Printf("  [%s] %s claimed %v\n", eventType, agentID, patterns)
	case "agent_release":
		patterns, _ := event["patterns"].([]any)
		fmt.Printf("  [%s] %s released %v\n", eventType, agentID, patterns)
	case "file_write":
		path, _ := event["path"].(string)
		fmt.Printf("  [%s] %s wrote %s\n", eventType, agentID, path)
	case "file_chunk":
		path, _ := event["path"].(string)
		version, _ := event["version"].(float64)
		fmt.Printf("  [%s] %s streaming %s (%d chars)\n", eventType, agentID, path, int(version))
	case "commit":
		sha, _ := event["sha"].(string)
		msg, _ := event["message"].(string)
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Printf("  [%s] %s %s\n", eventType, sha, msg)
	case "error":
		errMsg, _ := event["error"].(string)
		fmt.Printf("  [%s] %s: %s\n", eventType, agentID, errMsg)
	default:
		// Full JSON for unknown events
		formatted, _ := json.MarshalIndent(event, "  ", "  ")
		fmt.Println(string(formatted))
	}
}
