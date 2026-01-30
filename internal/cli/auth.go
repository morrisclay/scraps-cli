package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/scraps-sh/scraps-cli/internal/api"
	"github.com/scraps-sh/scraps-cli/internal/config"
	"github.com/scraps-sh/scraps-cli/internal/model"
	"github.com/scraps-sh/scraps-cli/internal/tui"
)

// --- Login Command ---

func newLoginCmd() *cobra.Command {
	var key string
	var host string

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Authenticate with your API key",
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				host = config.GetHost()
			}

			// Non-interactive mode
			if key != "" || !isInputInteractive() {
				if key == "" {
					// Read from stdin
					scanner := bufio.NewScanner(os.Stdin)
					if scanner.Scan() {
						key = strings.TrimSpace(scanner.Text())
					}
				}
				if key == "" {
					return fmt.Errorf("API key required")
				}
				return loginWithKey(host, key)
			}

			// Interactive TUI mode
			return runLoginTUI(host)
		},
	}

	cmd.Flags().StringVarP(&key, "key", "k", "", "API key")
	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host")

	return cmd
}

func loginWithKey(host, key string) error {
	client := api.NewClient(host, key)
	user, err := client.GetUser()
	if err != nil {
		return fmt.Errorf("authentication failed: %w", err)
	}

	err = config.SetCredential(host, config.Credential{
		APIKey:   key,
		UserID:   user.ID,
		Username: user.Username,
	})
	if err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	success(fmt.Sprintf("Logged in as %s", user.Username))
	return nil
}

// loginModel is the TUI model for the login command.
type loginModel struct {
	host      string
	input     textinput.Model
	spinner   spinner.Model
	state     string // "input", "loading", "done", "error"
	user      *model.User
	err       error
}

func newLoginModel(host string) loginModel {
	ti := textinput.New()
	ti.Placeholder = "scraps_..."
	ti.Focus()
	ti.CharLimit = 256
	ti.Width = 40
	ti.EchoMode = textinput.EchoPassword
	ti.EchoCharacter = '•'
	ti.PromptStyle = tui.PromptStyle

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = tui.SpinnerStyle

	return loginModel{
		host:    host,
		input:   ti,
		spinner: s,
		state:   "input",
	}
}

func (m loginModel) Init() tea.Cmd {
	return textinput.Blink
}

type loginResultMsg struct {
	user *model.User
	err  error
}

func (m loginModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "enter":
			if m.state == "input" && m.input.Value() != "" {
				m.state = "loading"
				key := m.input.Value()
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						client := api.NewClient(m.host, key)
						user, err := client.GetUser()
						if err != nil {
							return loginResultMsg{err: err}
						}
						// Save credentials
						saveErr := config.SetCredential(m.host, config.Credential{
							APIKey:   key,
							UserID:   user.ID,
							Username: user.Username,
						})
						if saveErr != nil {
							return loginResultMsg{err: saveErr}
						}
						return loginResultMsg{user: user}
					},
				)
			}
		}

	case loginResultMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
		} else {
			m.state = "done"
			m.user = msg.user
		}
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == "loading" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if m.state == "input" {
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m loginModel) View() string {
	switch m.state {
	case "input":
		return fmt.Sprintf(
			"%s\n\n%s\n\n%s",
			tui.TitleStyle.Render("Login to Scraps"),
			"Enter your API key:\n\n"+m.input.View(),
			tui.HelpStyle.Render("enter submit • esc cancel"),
		)
	case "loading":
		return m.spinner.View() + " Authenticating..."
	case "done":
		return tui.SuccessStyle.Render("✓") + fmt.Sprintf(" Logged in as %s", m.user.Username)
	case "error":
		return tui.ErrorStyle.Render("✗") + fmt.Sprintf(" Authentication failed: %v", m.err)
	}
	return ""
}

func runLoginTUI(host string) error {
	m := newLoginModel(host)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if lm, ok := finalModel.(loginModel); ok && lm.err != nil {
		return lm.err
	}
	return nil
}

// --- Logout Command ---

func newLogoutCmd() *cobra.Command {
	var host string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear saved credentials",
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				host = config.GetHost()
			}

			if err := config.RemoveCredential(host); err != nil {
				return fmt.Errorf("failed to remove credentials: %w", err)
			}

			success(fmt.Sprintf("Logged out from %s", host))
			return nil
		},
	}

	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host")
	return cmd
}

// --- Signup Command ---

func newSignupCmd() *cobra.Command {
	var username, email, host string

	cmd := &cobra.Command{
		Use:   "signup",
		Short: "Create a new account",
		RunE: func(cmd *cobra.Command, args []string) error {
			if host == "" {
				host = config.GetHost()
			}

			// Non-interactive if both provided
			if username != "" && email != "" {
				return signupNonInteractive(host, username, email)
			}

			// Interactive mode
			if !isInputInteractive() {
				return fmt.Errorf("username and email required in non-interactive mode")
			}

			return runSignupTUI(host, username, email)
		},
	}

	cmd.Flags().StringVarP(&username, "username", "u", "", "Username")
	cmd.Flags().StringVarP(&email, "email", "e", "", "Email address")
	cmd.Flags().StringVarP(&host, "host", "H", "", "Server host")

	return cmd
}

func signupNonInteractive(host, username, email string) error {
	client := api.NewClient(host, "")
	resp, err := client.Signup(username, email)
	if err != nil {
		return fmt.Errorf("signup failed: %w", err)
	}

	// Save credentials
	key := resp.RawKey
	if key == "" {
		key = resp.APIKey
	}

	err = config.SetCredential(host, config.Credential{
		APIKey:   key,
		UserID:   resp.User.ID,
		Username: resp.User.Username,
	})
	if err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}

	success(fmt.Sprintf("Account created! Logged in as %s", resp.User.Username))
	fmt.Printf("\nYour API key: %s\n", key)
	fmt.Println("Save this key - it won't be shown again!")
	return nil
}

// signupModel is the TUI model for signup.
type signupModel struct {
	host       string
	username   textinput.Model
	email      textinput.Model
	spinner    spinner.Model
	focusIndex int
	state      string
	result     *model.SignupResponse
	err        error
}

func newSignupModel(host, username, email string) signupModel {
	usernameInput := textinput.New()
	usernameInput.Placeholder = "username"
	usernameInput.CharLimit = 64
	usernameInput.Width = 30
	usernameInput.PromptStyle = tui.PromptStyle
	if username != "" {
		usernameInput.SetValue(username)
	}

	emailInput := textinput.New()
	emailInput.Placeholder = "email@example.com"
	emailInput.CharLimit = 128
	emailInput.Width = 30
	emailInput.PromptStyle = tui.PromptStyle
	if email != "" {
		emailInput.SetValue(email)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = tui.SpinnerStyle

	m := signupModel{
		host:     host,
		username: usernameInput,
		email:    emailInput,
		spinner:  s,
		state:    "input",
	}

	// Focus first empty field
	if username == "" {
		m.username.Focus()
		m.focusIndex = 0
	} else {
		m.email.Focus()
		m.focusIndex = 1
	}

	return m
}

func (m signupModel) Init() tea.Cmd {
	return textinput.Blink
}

type signupResultMsg struct {
	result *model.SignupResponse
	err    error
}

func (m signupModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit
		case "tab", "down":
			if m.state == "input" {
				m.focusIndex = (m.focusIndex + 1) % 2
				m.updateFocus()
			}
		case "shift+tab", "up":
			if m.state == "input" {
				m.focusIndex = (m.focusIndex + 1) % 2
				m.updateFocus()
			}
		case "enter":
			if m.state == "input" && m.username.Value() != "" && m.email.Value() != "" {
				m.state = "loading"
				username := m.username.Value()
				email := m.email.Value()
				return m, tea.Batch(
					m.spinner.Tick,
					func() tea.Msg {
						client := api.NewClient(m.host, "")
						resp, err := client.Signup(username, email)
						if err != nil {
							return signupResultMsg{err: err}
						}
						// Save credentials
						key := resp.RawKey
						if key == "" {
							key = resp.APIKey
						}
						saveErr := config.SetCredential(m.host, config.Credential{
							APIKey:   key,
							UserID:   resp.User.ID,
							Username: resp.User.Username,
						})
						if saveErr != nil {
							return signupResultMsg{err: saveErr}
						}
						return signupResultMsg{result: resp}
					},
				)
			}
		}

	case signupResultMsg:
		if msg.err != nil {
			m.state = "error"
			m.err = msg.err
		} else {
			m.state = "done"
			m.result = msg.result
		}
		return m, tea.Quit

	case spinner.TickMsg:
		if m.state == "loading" {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
	}

	if m.state == "input" {
		var cmd tea.Cmd
		if m.focusIndex == 0 {
			m.username, cmd = m.username.Update(msg)
		} else {
			m.email, cmd = m.email.Update(msg)
		}
		return m, cmd
	}

	return m, nil
}

func (m *signupModel) updateFocus() {
	if m.focusIndex == 0 {
		m.username.Focus()
		m.email.Blur()
	} else {
		m.username.Blur()
		m.email.Focus()
	}
}

func (m signupModel) View() string {
	switch m.state {
	case "input":
		return fmt.Sprintf(
			"%s\n\n%s\n%s\n\n%s\n%s\n\n%s",
			tui.TitleStyle.Render("Create Account"),
			tui.LabelStyle.Render("Username"),
			m.username.View(),
			tui.LabelStyle.Render("Email"),
			m.email.View(),
			tui.HelpStyle.Render("tab next field • enter submit • esc cancel"),
		)
	case "loading":
		return m.spinner.View() + " Creating account..."
	case "done":
		key := m.result.RawKey
		if key == "" {
			key = m.result.APIKey
		}
		return fmt.Sprintf(
			"%s Account created! Logged in as %s\n\n%s %s\n%s",
			tui.SuccessStyle.Render("✓"),
			m.result.User.Username,
			tui.LabelStyle.Render("Your API key:"),
			key,
			tui.WarningStyle.Render("Save this key - it won't be shown again!"),
		)
	case "error":
		return tui.ErrorStyle.Render("✗") + fmt.Sprintf(" Signup failed: %v", m.err)
	}
	return ""
}

func runSignupTUI(host, username, email string) error {
	m := newSignupModel(host, username, email)
	p := tea.NewProgram(m)
	finalModel, err := p.Run()
	if err != nil {
		return err
	}

	if sm, ok := finalModel.(signupModel); ok && sm.err != nil {
		return sm.err
	}
	return nil
}

// --- Whoami Command ---

func newWhoamiCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show current user information",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := api.NewClientFromConfig("")
			if err != nil {
				return err
			}

			user, err := client.GetUser()
			if err != nil {
				return err
			}

			if config.GetOutputFormat() == "json" {
				outputJSON(user)
			} else {
				fmt.Printf("Username: %s\n", user.Username)
				fmt.Printf("Email:    %s\n", user.Email)
				fmt.Printf("User ID:  %s\n", user.ID)
				fmt.Printf("Host:     %s\n", client.Host())
			}
			return nil
		},
	}
	return cmd
}

// --- Status Command ---

func newStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show login status and account info",
		RunE: func(cmd *cobra.Command, args []string) error {
			host := config.GetHost()
			cred, err := config.GetCredential(host)

			fmt.Printf("Host: %s\n", host)

			if err != nil || cred == nil {
				fmt.Println("Status: Not logged in")
				return nil
			}

			client := api.NewClient(host, cred.APIKey)
			user, err := client.GetUser()
			if err != nil {
				fmt.Println("Status: Invalid credentials")
				return nil
			}

			fmt.Println("Status: Logged in")
			fmt.Printf("Username: %s\n", user.Username)
			fmt.Printf("Email: %s\n", user.Email)
			fmt.Printf("User ID: %s\n", user.ID)

			// Get store count
			stores, err := client.ListStores()
			if err == nil {
				fmt.Printf("Stores: %d accessible\n", len(stores))
			}

			return nil
		},
	}
	return cmd
}
