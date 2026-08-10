package page

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/opencode-ai/opencode/internal/app"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/tui/layout"
	"github.com/opencode-ai/opencode/internal/tui/styles"
	"github.com/opencode-ai/opencode/internal/tui/theme"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

var WorkspacePage PageID = "workspace"

type workspaceAction struct {
	Title       string
	Description string
	Prompt      string
}

type workspacePage struct {
	app         *app.App
	width       int
	height      int
	focus       int // 0 = sources, 1 = actions
	cursor      int
	sources     []string
	actions     []workspaceAction
	snapshot    []string
	refreshedAt time.Time
}

type WorkspaceKeyMap struct {
	NextPane key.Binding
	PrevPane key.Binding
	Up       key.Binding
	Down     key.Binding
	Run      key.Binding
	Refresh  key.Binding
	GoChat   key.Binding
}

var workspaceKeyMap = WorkspaceKeyMap{
	NextPane: key.NewBinding(
		key.WithKeys("tab"),
		key.WithHelp("tab", "next pane"),
	),
	PrevPane: key.NewBinding(
		key.WithKeys("shift+tab"),
		key.WithHelp("shift+tab", "prev pane"),
	),
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("up/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("down/j", "move down"),
	),
	Run: key.NewBinding(
		key.WithKeys("enter"),
		key.WithHelp("enter", "run action"),
	),
	Refresh: key.NewBinding(
		key.WithKeys("r"),
		key.WithHelp("r", "refresh workspace"),
	),
	GoChat: key.NewBinding(
		key.WithKeys("c"),
		key.WithHelp("c", "go chat"),
	),
}

func (p *workspacePage) Init() tea.Cmd {
	p.refresh()
	return nil
}

func (p *workspacePage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.width = msg.Width
		p.height = msg.Height
		return p, nil
	case tea.KeyMsg:
		switch {
		case key.Matches(msg, workspaceKeyMap.NextPane):
			p.focus = (p.focus + 1) % 2
			p.cursor = 0
		case key.Matches(msg, workspaceKeyMap.PrevPane):
			p.focus = (p.focus + 1) % 2
			p.cursor = 0
		case key.Matches(msg, workspaceKeyMap.Up):
			if p.cursor > 0 {
				p.cursor--
			}
		case key.Matches(msg, workspaceKeyMap.Down):
			max := len(p.currentItems())
			if max > 0 && p.cursor < max-1 {
				p.cursor++
			}
		case key.Matches(msg, workspaceKeyMap.Refresh):
			p.refresh()
			return p, util.ReportInfo("Workspace snapshot refreshed")
		case key.Matches(msg, workspaceKeyMap.GoChat):
			return p, util.CmdHandler(PageChangeMsg{ID: ChatPage})
		case key.Matches(msg, workspaceKeyMap.Run):
			if p.focus == 1 && p.cursor >= 0 && p.cursor < len(p.actions) {
				action := p.actions[p.cursor]
				if strings.TrimSpace(action.Prompt) == "" {
					return p, util.ReportWarn("This workspace action has no prompt configured")
				}
				return p, util.CmdHandler(WorkspaceRunPromptMsg{
					Prompt:       action.Prompt,
					SwitchToChat: true,
				})
			}
		}
	}

	return p, nil
}

func (p *workspacePage) SetSize(width, height int) tea.Cmd {
	p.width = width
	p.height = height
	return nil
}

func (p *workspacePage) GetSize() (int, int) {
	return p.width, p.height
}

func (p *workspacePage) BindingKeys() []key.Binding {
	return layout.KeyMapToSlice(workspaceKeyMap)
}

func (p *workspacePage) currentItems() []string {
	if p.focus == 0 {
		return p.sources
	}
	items := make([]string, 0, len(p.actions))
	for _, action := range p.actions {
		items = append(items, action.Title)
	}
	return items
}

func (p *workspacePage) refresh() {
	cfg := config.Get()
	workingDir := config.WorkingDirectory()

	entries, _ := os.ReadDir(workingDir)
	folders := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			folders = append(folders, entry.Name())
		}
	}
	sort.Strings(folders)
	if len(folders) > 12 {
		folders = folders[:12]
	}

	sources := make([]string, 0, len(folders)+4)
	sources = append(sources, "workspace://"+workingDir)
	for _, folder := range folders {
		sources = append(sources, filepath.ToSlash(filepath.Join(workingDir, folder)))
	}
	sources = append(sources,
		"connector://directory (recursive)",
		"connector://remote_share (linux/windows)",
		"connector://web and sql",
	)
	p.sources = sources

	pythonTools := 0
	if cfg != nil {
		pythonTools = len(cfg.Python.Tools)
	}
	sessions, _ := p.app.Sessions.List(context.Background())
	currentModel := "unknown"
	if p.app.CoderAgent != nil {
		currentModel = p.app.CoderAgent.Model().Name
	}

	p.snapshot = []string{
		"Workspace: " + workingDir,
		fmt.Sprintf("Sessions: %d", len(sessions)),
		"Model: " + currentModel,
		fmt.Sprintf("RAG enabled: %t", cfg != nil && cfg.RAG.Enabled),
		fmt.Sprintf("Python tools configured: %d", pythonTools),
		fmt.Sprintf("Local only mode: %t", cfg != nil && cfg.LocalOnly),
	}

	p.actions = []workspaceAction{
		{
			Title:       "Index current workspace folder",
			Description: "Ingest this workspace recursively into RAG with safe limits.",
			Prompt:      "Run a RAG index command for the current workspace folder recursively with source-type directory, conservative max-files/max-bytes limits, and summarize what was indexed.",
		},
		{
			Title:       "Index a mounted remote share",
			Description: "Prepare and run a remote_share ingestion flow with path validation.",
			Prompt:      "Help me ingest a mounted remote share into RAG using source-type remote_share. Ask for the share path if missing, then run a safe recursive indexing command.",
		},
		{
			Title:       "RAG search sanity check",
			Description: "Run search and explain relevance quality and gaps.",
			Prompt:      "Run a RAG search sanity check using a query derived from this repository and report top results with citation usefulness.",
		},
		{
			Title:       "Benchmark retrieval latency",
			Description: "Measure RAG retrieval latency and proxy quality.",
			Prompt:      "Run rag benchmark with a practical query and explain p50/p95 latency and proxy quality for this workspace.",
		},
		{
			Title:       "Backfill embeddings",
			Description: "Re-embed existing chunks after model/provider changes.",
			Prompt:      "Run rag backfill-embeddings for all pending chunks and summarize updated count and any issues.",
		},
	}

	p.refreshedAt = time.Now()
	if p.focus < 0 || p.focus > 1 {
		p.focus = 0
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	max := len(p.currentItems())
	if max == 0 {
		p.cursor = 0
	} else if p.cursor >= max {
		p.cursor = max - 1
	}
}

func (p *workspacePage) View() string {
	t := theme.CurrentTheme()

	titleStyle := styles.BaseStyle().
		Bold(true).
		Foreground(t.TextEmphasized())
	subtitleStyle := styles.BaseStyle().
		Foreground(t.TextMuted())

	cardStyle := styles.BaseStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(t.BorderNormal()).
		Padding(0, 1)
	focusedCardStyle := cardStyle.Copy().BorderForeground(t.BorderFocused())

	renderList := func(title string, items []string, focused bool) string {
		body := make([]string, 0, len(items)+1)
		body = append(body, title)
		for i, item := range items {
			prefix := "  "
			if focused && i == p.cursor {
				prefix = "> "
			}
			row := prefix + item
			if focused && i == p.cursor {
				row = styles.BaseStyle().Foreground(t.Primary()).Render(row)
			}
			body = append(body, row)
		}
		rendered := strings.Join(body, "\n")
		if focused {
			return focusedCardStyle.Render(rendered)
		}
		return cardStyle.Render(rendered)
	}

	actionTitles := make([]string, 0, len(p.actions))
	for _, action := range p.actions {
		actionTitles = append(actionTitles, action.Title)
	}

	sourcesPanel := renderList("Sources", p.sources, p.focus == 0)
	actionsPanel := renderList("Actions", actionTitles, p.focus == 1)

	detailLines := []string{
		"Workspace Snapshot",
	}
	detailLines = append(detailLines, p.snapshot...)
	detailLines = append(detailLines, "")
	if p.focus == 1 && p.cursor >= 0 && p.cursor < len(p.actions) {
		detailLines = append(detailLines,
			"Selected Action",
			p.actions[p.cursor].Description,
		)
	} else if p.focus == 0 && p.cursor >= 0 && p.cursor < len(p.sources) {
		detailLines = append(detailLines,
			"Selected Source",
			p.sources[p.cursor],
		)
	}
	detailLines = append(detailLines,
		"",
		"Tips",
		"- Press Enter on Actions to run in chat.",
		"- Press r to refresh workspace snapshot.",
		"- Press c to jump back to chat.",
		fmt.Sprintf("- Last refresh: %s", p.refreshedAt.Format("2006-01-02 15:04:05")),
	)
	detailsPanel := cardStyle.Render(strings.Join(detailLines, "\n"))

	panelWidth := max(20, (p.width-6)/3)
	sourcesPanel = lipgloss.NewStyle().Width(panelWidth).Render(sourcesPanel)
	actionsPanel = lipgloss.NewStyle().Width(panelWidth).Render(actionsPanel)
	detailsPanel = lipgloss.NewStyle().Width(panelWidth).Render(detailsPanel)

	row := lipgloss.JoinHorizontal(lipgloss.Top, sourcesPanel, " ", actionsPanel, " ", detailsPanel)

	header := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("Workspace Studio"),
		subtitleStyle.Render("Notebook-style workspace control center for RAG sources, actions, and operations"),
	)

	view := lipgloss.JoinVertical(lipgloss.Left, header, "", row)
	return styles.BaseStyle().Width(p.width).Height(p.height).Render(view)
}

func NewWorkspacePage(app *app.App) tea.Model {
	return &workspacePage{
		app: app,
	}
}
