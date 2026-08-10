package page

type PageID string

// PageChangeMsg is used to change the current page
type PageChangeMsg struct {
	ID PageID
}

type WorkspaceRunPromptMsg struct {
	Prompt       string
	SwitchToChat bool
}
