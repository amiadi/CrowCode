package dialog

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

type SetupWizardSubmitMsg struct {
	Options config.ConnectionSetupOptions
}

type CloseSetupWizardMsg struct{}

type setupWizardKeyMap struct {
	Up     key.Binding
	Down   key.Binding
	Left   key.Binding
	Right  key.Binding
	Tab    key.Binding
	Enter  key.Binding
	Escape key.Binding
	Toggle key.Binding
	Probe  key.Binding
}

var setupWizardKeys = setupWizardKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "move down"),
	),
	Left: key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("left/h", "prev provider"),
	),
	Right: key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("right/l", "next provider"),
	),
	Tab: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next field"),
	),
	Enter: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "apply"),
	),
	Escape: key.NewBinding(
		key.WithKeys("esc"),
		key.WithHelp("esc", "close"),
	),
	Toggle: key.NewBinding(
		key.WithKeys(" "),
		key.WithHelp("space", "toggle localOnly"),
	),
	Probe: key.NewBinding(
		key.WithKeys("p"),
		key.WithHelp("p", "probe + apply"),
	),
}

type SetupWizardDialog interface {
	tea.Model
	SetSize(width, height int)
	BindingKeys() []key.Binding
}

type setupWizardDialogCmp struct {
	width, height  int
	focus          int
	providerIdx    int
	providers      []config.ConnectionProvider
	endpointInput  textinput.Model
	localOnly      bool
	preferEmbedder bool
}

func NewSetupWizardDialogCmp() SetupWizardDialog {
	t := theme.CurrentTheme()
	endpointInput := textinput.New()
	endpointInput.Placeholder = "http://127.0.0.1:11434"
	endpointInput.Prompt = ""
	endpointInput.Width = 44
	endpointInput.TextStyle = endpointInput.TextStyle.Foreground(t.Primary())
	endpointInput.PlaceholderStyle = endpointInput.PlaceholderStyle.Foreground(t.TextMuted())
	endpointInput.Focus()

	cfg := config.Get()
	endpoint := ""
	localOnly := false
	if cfg != nil {
		endpoint = strings.TrimSpace(cfg.LocalEndpoint)
		localOnly = cfg.LocalOnly
	}
	if endpoint != "" {
		endpointInput.SetValue(endpoint)
	}

	return &setupWizardDialogCmp{
		focus:          1,
		providerIdx:    0,
		providers:      []config.ConnectionProvider{config.ConnectionProviderOllama, config.ConnectionProviderVLLM, config.ConnectionProviderSGLang},
		endpointInput:  endpointInput,
		localOnly:      localOnly,
		preferEmbedder: true,
	}
}

func (m *setupWizardDialogCmp) Init() tea.Cmd {
	return textinput.Blink
}

func (m *setupWizardDialogCmp) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, setupWizardKeys.Escape):
			return m, util.CmdHandler(CloseSetupWizardMsg{})
		case key.Matches(msg, setupWizardKeys.Tab), key.Matches(msg, setupWizardKeys.Down):
			m.focus = (m.focus + 1) % 4
			if m.focus == 1 {
				m.endpointInput.Focus()
			} else {
				m.endpointInput.Blur()
			}
			return m, nil
		case key.Matches(msg, setupWizardKeys.Up):
			m.focus = (m.focus - 1 + 4) % 4
			if m.focus == 1 {
				m.endpointInput.Focus()
			} else {
				m.endpointInput.Blur()
			}
			return m, nil
		case key.Matches(msg, setupWizardKeys.Left):
			if m.focus == 0 {
				m.providerIdx = (m.providerIdx - 1 + len(m.providers)) % len(m.providers)
				m.setProviderDefaultsIfEmpty()
			}
			return m, nil
		case key.Matches(msg, setupWizardKeys.Right):
			if m.focus == 0 {
				m.providerIdx = (m.providerIdx + 1) % len(m.providers)
				m.setProviderDefaultsIfEmpty()
			}
			return m, nil
		case key.Matches(msg, setupWizardKeys.Toggle):
			if m.focus == 2 {
				m.localOnly = !m.localOnly
			}
			if m.focus == 3 {
				m.preferEmbedder = !m.preferEmbedder
			}
			return m, nil
		case key.Matches(msg, setupWizardKeys.Probe), key.Matches(msg, setupWizardKeys.Enter):
			endpoint := config.NormalizeEndpoint(m.endpointInput.Value())
			if endpoint == "" {
				return m, util.ReportWarn("Endpoint is required")
			}
			opts := config.ConnectionSetupOptions{
				Provider:       m.providers[m.providerIdx],
				Endpoint:       endpoint,
				LocalOnly:      m.localOnly,
				PreferEmbedder: m.preferEmbedder,
			}
			return m, util.CmdHandler(SetupWizardSubmitMsg{Options: opts})
		}
	}

	if m.focus == 1 {
		var cmd tea.Cmd
		m.endpointInput, cmd = m.endpointInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *setupWizardDialogCmp) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *setupWizardDialogCmp) BindingKeys() []key.Binding {
	return layoutKeySlice(setupWizardKeys)
}

func (m *setupWizardDialogCmp) View() string {
	t := theme.CurrentTheme()
	base := styles.BaseStyle()

	providerLine := fmt.Sprintf("Provider: %s", string(m.providers[m.providerIdx]))
	if m.focus == 0 {
		providerLine = base.Foreground(t.Primary()).Bold(true).Render(providerLine)
	}

	endpointLine := "Endpoint URL"
	if m.focus == 1 {
		endpointLine = base.Foreground(t.Primary()).Bold(true).Render(endpointLine)
	}

	localOnlyLine := fmt.Sprintf("Local-only mode: %t", m.localOnly)
	if m.focus == 2 {
		localOnlyLine = base.Foreground(t.Primary()).Bold(true).Render(localOnlyLine)
	}

	embedLine := fmt.Sprintf("Prefer ollama embedder: %t", m.preferEmbedder)
	if m.focus == 3 {
		embedLine = base.Foreground(t.Primary()).Bold(true).Render(embedLine)
	}

	title := base.Foreground(t.Primary()).Bold(true).Render("Connection Setup Wizard")
	hint := base.Foreground(t.TextMuted()).Render("Set up Ollama/vLLM/SGLang endpoint and apply to config")
	keys := base.Foreground(t.TextMuted()).Render("tab/up/down focus, left/right provider, space toggle, enter apply, p probe+apply")

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		hint,
		"",
		providerLine,
		endpointLine,
		m.endpointInput.View(),
		localOnlyLine,
		embedLine,
		"",
		keys,
	)

	return base.Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderFocused()).
		Background(t.Background()).
		Render(content)
}

func (m *setupWizardDialogCmp) setProviderDefaultsIfEmpty() {
	if strings.TrimSpace(m.endpointInput.Value()) != "" {
		return
	}
	switch m.providers[m.providerIdx] {
	case config.ConnectionProviderOllama:
		m.endpointInput.SetValue("http://127.0.0.1:11434")
	case config.ConnectionProviderVLLM:
		m.endpointInput.SetValue("http://127.0.0.1:8000")
	case config.ConnectionProviderSGLang:
		m.endpointInput.SetValue("http://127.0.0.1:30000")
	}
}

func layoutKeySlice(km setupWizardKeyMap) []key.Binding {
	return []key.Binding{km.Tab, km.Up, km.Down, km.Left, km.Right, km.Toggle, km.Enter, km.Escape, km.Probe}
}
