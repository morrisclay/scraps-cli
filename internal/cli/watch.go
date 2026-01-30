package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/morrisclay/scraps-cli/internal/api"
	"github.com/morrisclay/scraps-cli/internal/config"
	"github.com/morrisclay/scraps-cli/internal/model"
	"github.com/morrisclay/scraps-cli/internal/stream"
	"github.com/morrisclay/scraps-cli/internal/tui"
)

func newWatchCmd() *cobra.Command {
	var branch, lastEvent string
	var claims bool

	cmd := &cobra.Command{
		Use:   "watch <store/repo[:branch]>",
		Short: "Watch repository events in real-time",
		Args:  cobra.ExactArgs(1),
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

			// Interactive TUI mode
			if isInteractive() && config.GetOutputFormat() != "json" {
				return runWatchTUI(client, store, repo, branch, claims)
			}

			// Non-interactive: just stream to stdout
			return runWatchNonInteractive(client, store, repo, branch)
		},
	}

	cmd.Flags().StringVarP(&branch, "branch", "b", "", "Filter to specific branch")
	cmd.Flags().StringVar(&lastEvent, "last-event", "", "Resume from event ID")
	cmd.Flags().BoolVar(&claims, "claims", false, "Show claim/release activity")

	return cmd
}

func runWatchNonInteractive(client *api.Client, store, repo, branch string) error {
	streamURL := client.BuildStreamURL(store, repo)
	streamClient := stream.NewClient(streamURL, client.APIKey())

	streamClient.OnMessage = func(data []byte) {
		// Pretty print JSON
		var msg map[string]any
		if json.Unmarshal(data, &msg) == nil {
			formatted, _ := json.MarshalIndent(msg, "", "  ")
			fmt.Println(string(formatted))
		} else {
			fmt.Println(string(data))
		}
	}

	streamClient.OnError = func(err error) {
		errorf("Stream error: %v", err)
	}

	streamClient.OnClose = func() {
		info("Connection closed")
	}

	if err := streamClient.Connect(); err != nil {
		return fmt.Errorf("failed to connect: %w", err)
	}
	defer streamClient.Close()

	info(fmt.Sprintf("Watching %s/%s", store, repo))
	if branch != "" {
		fmt.Printf("Branch: %s\n", branch)
	}
	fmt.Println("Press Ctrl+C to stop")
	fmt.Println()

	// Wait for connection to close
	<-streamClient.Done()
	return nil
}

// watchModel is the TUI model for watching events.
type watchModel struct {
	client       *api.Client
	streamClient *stream.Client
	store        string
	repo         string
	branch       string
	claims       bool
	connected    bool
	events       []watchEvent
	eventCount   int
	viewport     viewport.Model
	ready        bool
	filter       string
	showClaims   bool
	width        int
	height       int
	err          error
}

type watchEvent struct {
	Time    time.Time
	Type    string
	Summary string
	Details string
}

type streamConnectedMsg struct{}
type streamMessageMsg struct{ data []byte }
type streamErrorMsg struct{ err error }
type streamClosedMsg struct{}

func newWatchModel(client *api.Client, store, repo, branch string, claims bool) watchModel {
	return watchModel{
		client:     client,
		store:      store,
		repo:       repo,
		branch:     branch,
		claims:     claims,
		events:     make([]watchEvent, 0),
		showClaims: true,
	}
}

func (m watchModel) Init() tea.Cmd {
	return m.connect()
}

func (m *watchModel) connect() tea.Cmd {
	return func() tea.Msg {
		streamURL := m.client.BuildStreamURL(m.store, m.repo)
		m.streamClient = stream.NewClient(streamURL, m.client.APIKey())

		if err := m.streamClient.Connect(); err != nil {
			return streamErrorMsg{err: err}
		}

		return streamConnectedMsg{}
	}
}

func (m watchModel) waitForMessage() tea.Cmd {
	if m.streamClient == nil {
		return nil
	}

	msgChan := make(chan []byte, 1)
	errChan := make(chan error, 1)
	closeChan := make(chan struct{}, 1)

	m.streamClient.OnMessage = func(data []byte) {
		select {
		case msgChan <- data:
		default:
		}
	}
	m.streamClient.OnError = func(err error) {
		select {
		case errChan <- err:
		default:
		}
	}
	m.streamClient.OnClose = func() {
		select {
		case closeChan <- struct{}{}:
		default:
		}
	}

	return func() tea.Msg {
		select {
		case data := <-msgChan:
			return streamMessageMsg{data: data}
		case err := <-errChan:
			return streamErrorMsg{err: err}
		case <-closeChan:
			return streamClosedMsg{}
		}
	}
}

func (m watchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		headerHeight := 4
		footerHeight := 2

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-headerHeight-footerHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - headerHeight - footerHeight
		}
		m.updateViewport()

	case tea.KeyMsg:
		switch {
		case key.Matches(msg, key.NewBinding(key.WithKeys("q", "ctrl+c"))):
			if m.streamClient != nil {
				m.streamClient.Close()
			}
			return m, tea.Quit
		case key.Matches(msg, key.NewBinding(key.WithKeys("c"))):
			m.showClaims = !m.showClaims
			m.updateViewport()
		case key.Matches(msg, key.NewBinding(key.WithKeys("/"))):
			// TODO: implement filtering
		}

	case streamConnectedMsg:
		m.connected = true
		return m, m.waitForMessage()

	case streamMessageMsg:
		m.processMessage(msg.data)
		m.updateViewport()
		return m, m.waitForMessage()

	case streamErrorMsg:
		m.err = msg.err
		m.connected = false
		return m, nil

	case streamClosedMsg:
		m.connected = false
		return m, nil
	}

	if m.ready {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *watchModel) processMessage(data []byte) {
	var baseMsg model.WsMessage
	if err := json.Unmarshal(data, &baseMsg); err != nil {
		return
	}

	event := watchEvent{
		Time: time.Now(),
		Type: baseMsg.Type,
	}

	switch baseMsg.Type {
	case "commit":
		var commit model.CommitEvent
		json.Unmarshal(data, &commit)
		event.Summary = truncate(commit.Message, 40)
		if commit.SHA != "" {
			sha := commit.SHA
			if len(sha) > 7 {
				sha = sha[:7]
			}
			event.Summary = sha + " " + event.Summary
		}
		if len(commit.Files) > 0 {
			var details []string
			for _, f := range commit.Files {
				prefix := " "
				switch f.Action {
				case "add":
					prefix = "+"
				case "delete":
					prefix = "-"
				case "modify":
					prefix = "~"
				}
				details = append(details, prefix+" "+f.Path)
			}
			event.Details = strings.Join(details, "\n")
		}

	case "branch:create", "branch:delete", "branch:update", "ref:update":
		var branch model.BranchEvent
		json.Unmarshal(data, &branch)
		branchName := branch.Branch
		if branchName == "" {
			branchName = branch.Name
		}
		if branchName == "" {
			branchName = branch.Ref
		}
		event.Summary = branchName

	case "activity":
		var activity model.ActivityEvent
		json.Unmarshal(data, &activity)
		if activity.Activity.Type == "claim" {
			event.Type = "CLAIM"
			event.Summary = activity.Activity.AgentID
			event.Details = strings.Join(activity.Activity.Patterns, "\n")
		} else if activity.Activity.Type == "release" {
			event.Type = "RELEASE"
			event.Summary = activity.Activity.AgentID
			event.Details = strings.Join(activity.Activity.Patterns, "\n")
		}

	case "agent_claim":
		var claim model.AgentClaimEvent
		json.Unmarshal(data, &claim)
		event.Type = "CLAIM"
		event.Summary = claim.AgentID
		if claim.Claim != "" {
			event.Summary += " - " + truncate(claim.Claim, 30)
		}
		event.Details = strings.Join(claim.Patterns, "\n")

	case "agent_release":
		var release model.AgentClaimEvent
		json.Unmarshal(data, &release)
		event.Type = "RELEASE"
		event.Summary = release.AgentID
		event.Details = strings.Join(release.Patterns, "\n")

	default:
		event.Summary = string(data)
	}

	m.events = append([]watchEvent{event}, m.events...)
	m.eventCount++

	// Limit event history
	if len(m.events) > 100 {
		m.events = m.events[:100]
	}
}

func (m *watchModel) updateViewport() {
	if !m.ready {
		return
	}

	var lines []string
	for _, e := range m.events {
		// Filter claims if disabled
		if !m.showClaims && (e.Type == "CLAIM" || e.Type == "RELEASE") {
			continue
		}

		timeStr := tui.MutedStyle.Render(e.Time.Format("15:04:05"))
		typeStyle := tui.LabelStyle
		switch e.Type {
		case "commit":
			typeStyle = typeStyle.Foreground(tui.ColorSuccess)
		case "CLAIM":
			typeStyle = typeStyle.Foreground(tui.ColorWarning)
		case "RELEASE":
			typeStyle = typeStyle.Foreground(tui.ColorSecondary)
		}

		line := fmt.Sprintf("%s %s %s", timeStr, typeStyle.Render(strings.ToUpper(e.Type)), e.Summary)
		lines = append(lines, line)

		if e.Details != "" {
			for _, detail := range strings.Split(e.Details, "\n") {
				lines = append(lines, "         "+tui.MutedStyle.Render(detail))
			}
		}
	}

	m.viewport.SetContent(strings.Join(lines, "\n"))
}

func (m watchModel) View() string {
	var s strings.Builder

	// Header
	title := fmt.Sprintf("Watching: %s/%s", m.store, m.repo)
	if m.branch != "" {
		title += ":" + m.branch
	}
	s.WriteString(tui.TitleStyle.Render(title))

	// Connection status
	if m.connected {
		s.WriteString("  ")
		s.WriteString(tui.ConnectedStyle.Render("● Connected"))
	} else if m.err != nil {
		s.WriteString("  ")
		s.WriteString(tui.DisconnectedStyle.Render("● Error: " + m.err.Error()))
	} else {
		s.WriteString("  ")
		s.WriteString(tui.MutedStyle.Render("○ Connecting..."))
	}

	s.WriteString(fmt.Sprintf("  %d events", m.eventCount))
	s.WriteString("\n")
	s.WriteString(strings.Repeat("─", m.width))
	s.WriteString("\n")

	// Events viewport
	if m.ready {
		s.WriteString(m.viewport.View())
	}

	// Footer
	s.WriteString("\n")
	helpText := "q quit"
	if m.showClaims {
		helpText += "  c hide claims"
	} else {
		helpText += "  c show claims"
	}
	helpText += "  / filter"
	s.WriteString(tui.HelpStyle.Render(helpText))

	return s.String()
}

func runWatchTUI(client *api.Client, store, repo, branch string, claims bool) error {
	m := newWatchModel(client, store, repo, branch, claims)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}
