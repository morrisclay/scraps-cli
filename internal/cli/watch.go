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

// streamState tracks in-progress file streaming for cursor updates
type streamState struct {
	lastChunkAgent string
	lastChunkFile  string
	hasChunkLine   bool
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
			printEvent(events[i], nil) // No cursor updates for historical
		}
		fmt.Println("--- Live events ---")
	} else {
		fmt.Println("(no recent events)")
	}

	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	opts := &api.StreamOptions{Branch: branch, Path: path}
	streamURL := client.BuildStreamURL(store, repo, opts)

	state := &streamState{}

	// Auto-reconnect loop
	for {
		streamClient := stream.NewClient(streamURL, client.APIKey())

		streamClient.OnMessage = func(data []byte) {
			var msg map[string]any
			if json.Unmarshal(data, &msg) == nil {
				printEvent(msg, state)
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

// ANSI color codes
const (
	colorReset   = "\033[0m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorBlue    = "\033[34m"
	colorMagenta = "\033[35m"
	colorCyan    = "\033[36m"
	colorRed     = "\033[31m"
	colorGray    = "\033[90m"
)

func coloredType(eventType string) string {
	var color string
	switch eventType {
	case "agent_join":
		color = colorGreen
	case "agent_leave":
		color = colorYellow
	case "agent_claim":
		color = colorMagenta
	case "agent_release":
		color = colorCyan
	case "file_write":
		color = colorBlue
	case "file_chunk":
		color = colorGray
	case "commit":
		color = colorGreen
	case "error":
		color = colorRed
	default:
		color = colorReset
	}
	return fmt.Sprintf("%s[%s]%s", color, eventType, colorReset)
}

func printEvent(event map[string]any, state *streamState) {
	eventType, _ := event["type"].(string)
	agentID, _ := event["agent_id"].(string)
	tag := coloredType(eventType)

	// Handle file_chunk specially for cursor updates
	if eventType == "file_chunk" {
		path, _ := event["path"].(string)
		version, _ := event["version"].(float64)

		if state != nil {
			// Check if this is a continuation of the same stream
			sameStream := state.lastChunkAgent == agentID && state.lastChunkFile == path

			if sameStream && state.hasChunkLine {
				// Update in place with carriage return
				fmt.Printf("\r  %s %s streaming %s (%d chars)    ", tag, agentID, path, int(version))
			} else {
				// New stream or first chunk - finish previous line if any
				if state.hasChunkLine {
					fmt.Println() // Commit previous line
				}
				fmt.Printf("  %s %s streaming %s (%d chars)", tag, agentID, path, int(version))
			}

			state.lastChunkAgent = agentID
			state.lastChunkFile = path
			state.hasChunkLine = true
		} else {
			// No state (historical) - just print normally
			fmt.Printf("  %s %s streaming %s (%d chars)\n", tag, agentID, path, int(version))
		}
		return
	}

	// For non-chunk events, commit any pending chunk line first
	if state != nil && state.hasChunkLine {
		fmt.Println() // Finish the chunk line
		state.hasChunkLine = false
		state.lastChunkAgent = ""
		state.lastChunkFile = ""
	}

	// Compact format for common events
	switch eventType {
	case "agent_join":
		role, _ := event["role"].(string)
		fmt.Printf("  %s %s joined (%s)\n", tag, agentID, role)
	case "agent_leave":
		role, _ := event["role"].(string)
		fmt.Printf("  %s %s left (%s)\n", tag, agentID, role)
	case "agent_claim":
		patterns, _ := event["patterns"].([]any)
		fmt.Printf("  %s %s claimed %v\n", tag, agentID, patterns)
	case "agent_release":
		patterns, _ := event["patterns"].([]any)
		fmt.Printf("  %s %s released %v\n", tag, agentID, patterns)
	case "file_write":
		path, _ := event["path"].(string)
		fmt.Printf("  %s %s wrote %s\n", tag, agentID, path)
	case "commit":
		sha, _ := event["sha"].(string)
		msg, _ := event["message"].(string)
		if len(sha) > 7 {
			sha = sha[:7]
		}
		fmt.Printf("  %s %s %s\n", tag, sha, msg)
	case "error":
		errMsg, _ := event["error"].(string)
		fmt.Printf("  %s %s: %s\n", tag, agentID, errMsg)
	default:
		// Full JSON for unknown events
		formatted, _ := json.MarshalIndent(event, "  ", "  ")
		fmt.Println(string(formatted))
	}
}
