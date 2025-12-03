package namespace

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/util"
)

// SortMode represents how namespaces are sorted
type SortMode int

const (
	SortNameAsc SortMode = iota
	SortNameDesc
	SortStatus
)

// Column width constants for namespace list
const (
	// Column widths
	colCursor = 2  // "▶ " or "  "
	colMinName    = 12
	colStatus     = 10
	colAge        = 5

	// Chrome overhead for window calculation
	chromeLines = 10
)

// Model represents the namespace list pane
type Model struct {
	state       *state.AppState
	cursor      int
	windowStart int // Explicit window start for scrolling
	filter      string
	isFiltering bool
	sortMode    SortMode
	active      bool
	width       int
	height      int
}

// NewModel creates a new namespace pane model
func NewModel(appState *state.AppState) Model {
	return Model{
		state:  appState,
		cursor: 0,
		active: true, // Start with namespace pane active
	}
}

// Init initializes the namespace model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the namespace pane
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Handle filter mode separately
		if m.isFiltering {
			switch msg.String() {
			case "esc":
				// Cancel filtering
				m.isFiltering = false
				m.filter = ""
				m.cursor = 0
			case "enter":
				// Apply filter
				m.isFiltering = false
				m.cursor = 0
				m.windowStart = 0
			case "backspace":
				// Remove last character
				if len(m.filter) > 0 {
					m.filter = m.filter[:len(m.filter)-1]
					m.cursor = 0
				}
			default:
				// Add character to filter
				if len(msg.String()) == 1 {
					m.filter += msg.String()
					m.cursor = 0
				}
			}
			return m, nil
		}

		// Normal mode
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
				m.adjustWindowForCursor(m.getFilteredAndSortedNamespaces())
			}
		case "down", "j":
			namespaces := m.getFilteredAndSortedNamespaces()
			if m.cursor < len(namespaces)-1 {
				m.cursor++
				m.adjustWindowForCursor(namespaces)
			}
		case "pgup":
			pageSize := m.calculatePageSize()
			if m.cursor >= pageSize {
				m.cursor -= pageSize
			} else {
				m.cursor = 0
			}
			m.adjustWindowForCursor(m.getFilteredAndSortedNamespaces())
		case "pgdown":
			namespaces := m.getFilteredAndSortedNamespaces()
			pageSize := m.calculatePageSize()
			if m.cursor+pageSize < len(namespaces) {
				m.cursor += pageSize
			} else if len(namespaces) > 0 {
				m.cursor = len(namespaces) - 1
			}
			m.adjustWindowForCursor(namespaces)
		case "enter":
			// Select namespace and trigger pod loading
			namespaces := m.getFilteredAndSortedNamespaces()
			if m.cursor < len(namespaces) {
				selectedNS := namespaces[m.cursor].Name
				m.state.SelectedNamespace = selectedNS
				// Don't set LoadingPods here - let main model handle it
				m.state.PodsError = ""
				m.state.Pods = []state.PodInfo{} // Clear previous pods
			}
		case "/":
			// Enter filter mode
			m.isFiltering = true
			m.filter = ""
		case "s":
			// Cycle sort mode
			m.sortMode = (m.sortMode + 1) % 3
			m.cursor = 0
			m.windowStart = 0
		case "h":
			// Show namespace health summary
			namespaces := m.getFilteredAndSortedNamespaces()
			if m.cursor < len(namespaces) {
				selectedNS := namespaces[m.cursor].Name

				// Idempotent: only trigger load if not already loading and not for same namespace
				if !m.state.LoadingNamespaceHealth {
					// Check if we need to reload (different namespace or no data)
					needsLoad := m.state.CurrentNamespaceHealth == nil ||
						m.state.SelectedNamespace != selectedNS

					m.state.SelectedNamespace = selectedNS
					m.state.CurrentView = state.ViewNamespaceHealth

					if needsLoad {
						m.state.LoadingNamespaceHealth = true
						m.state.NamespaceHealthError = ""
					}
					// If health already loaded for this namespace, just switch view
				} else {
					// If already loading, just switch view - don't trigger another load
					m.state.SelectedNamespace = selectedNS
					m.state.CurrentView = state.ViewNamespaceHealth
				}
			}
		}
	}

	return m, nil
}

// View renders the namespace pane
func (m Model) View() string {
	var b strings.Builder

	// Title
	title := theme.TitleStyle.Render("Namespaces")
	b.WriteString(title + "\n\n")

	// Handle different states
	if m.state.LoadingNamespaces {
		b.WriteString(theme.LoadingStyle.Render("⏳ Loading namespaces...\n"))
	} else if m.state.NamespacesError != "" {
		b.WriteString(theme.StatusErrorStyle.Render("✗ Error loading namespaces:\n"))
		b.WriteString(theme.EmptyStyle.Render(fmt.Sprintf("  %s\n", util.Ellipsize(m.state.NamespacesError, 50))))
	} else {
		// Namespace list
		namespaces := m.getFilteredAndSortedNamespaces()
		if len(namespaces) == 0 {
			if m.filter != "" {
				b.WriteString(theme.EmptyStyle.Render("No namespaces match filter '"+m.filter+"'"))
			} else {
				b.WriteString(theme.EmptyStyle.Render("No namespaces found"))
			}
		} else {
			// Show sort mode indicator
			sortModeText := ""
			switch m.sortMode {
			case SortNameAsc:
				sortModeText = "Sort: Name ↑"
			case SortNameDesc:
				sortModeText = "Sort: Name ↓"
			case SortStatus:
				sortModeText = "Sort: Status"
			}
			b.WriteString(theme.HelpStyle.Render(sortModeText) + "\n\n")

			// Calculate column widths
			nameWidth, showAge := m.calculateNameWidth()

			// Windowed rendering: only render visible items
			start, end := m.calculateVisibleWindow(len(namespaces))
			for i := start; i < end; i++ {
				ns := namespaces[i]

				// Cursor indicator
				cursorPrefix := "  "
				if i == m.cursor {
					cursorPrefix = "▶ "
				}

				// NAME column - truncate to fit and pad
				nsName := ns.Name
				if len(nsName) > nameWidth {
					nsName = util.Ellipsize(nsName, nameWidth)
				}
				nsName = util.PadRight(nsName, nameWidth)

				// STATUS column - pad BEFORE styling
				statusText := ns.Status
				if len(statusText) > colStatus {
					statusText = statusText[:colStatus]
				}
				statusText = util.PadRight(statusText, colStatus)

				statusStyle := theme.StatusInfoStyle
				if ns.Status != "Active" {
					statusStyle = theme.StatusWarnStyle
				}
				statusCol := statusStyle.Render(statusText)

				// Build row parts
				var rowParts []string
				rowParts = append(rowParts, cursorPrefix+nsName)
				rowParts = append(rowParts, statusCol)

				// AGE column - only if visible
				if showAge {
					ageText := ns.Age
					if len(ageText) > colAge {
						ageText = ageText[:colAge]
					}
					ageText = util.PadRight(ageText, colAge)
					rowParts = append(rowParts, ageText)
				}

				// Build final line - single spaces guarantee single line
				line := strings.Join(rowParts, " ")

				// Apply selection styling if selected
				if i == m.cursor {
					line = theme.ListItemSelectedStyle.Render(line)
				}

				b.WriteString(line + "\n")
			}
		}
	}

	// Help text - use width-aware formatting
	b.WriteString("\n")
	if m.isFiltering {
		filterPrompt := fmt.Sprintf("Filter: %s_", m.filter)
		b.WriteString(theme.StatusInfoStyle.Render(filterPrompt) + "\n")
		// Filtering mode: simpler help
		help := util.FormatHelpLine(m.width-8, "Enter apply", "Esc cancel")
		b.WriteString(theme.HelpStyle.Render(help))
	} else {
		// Normal mode: full help with priorities
		help := util.FormatHelpLine(m.width-8,
			"↑/↓ move",
			"Enter select",
			"/ filter",
			"s sort",
			"h health")
		b.WriteString(theme.HelpStyle.Render(help))
	}

	// Apply pane styling
	content := b.String()
	style := theme.PaneStyle
	if m.active {
		style = theme.PaneStyleActive
	}

	return style.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(content)
}

// SetActive sets whether this pane is active
func (m *Model) SetActive(active bool) {
	m.active = active
}

// SetSize sets the dimensions of the pane
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// getFilteredAndSortedNamespaces returns namespaces matching the current filter and sorted by sort mode
func (m Model) getFilteredAndSortedNamespaces() []state.NamespaceInfo {
	// Start with all namespaces
	result := make([]state.NamespaceInfo, 0, len(m.state.Namespaces))

	// Apply filter
	for _, ns := range m.state.Namespaces {
		if m.filter == "" || strings.Contains(strings.ToLower(ns.Name), strings.ToLower(m.filter)) {
			result = append(result, ns)
		}
	}

	// Apply sorting
	switch m.sortMode {
	case SortNameAsc:
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	case SortNameDesc:
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name > result[j].Name
		})
	case SortStatus:
		sort.Slice(result, func(i, j int) bool {
			// Active first, then others
			if result[i].Status == "Active" && result[j].Status != "Active" {
				return true
			}
			if result[i].Status != "Active" && result[j].Status == "Active" {
				return false
			}
			return result[i].Name < result[j].Name
		})
	}

	return result
}

// calculateVisibleWindow computes which items should be rendered based on windowStart and available height
func (m Model) calculateVisibleWindow(totalItems int) (start, end int) {
	pageSize := m.calculatePageSize()

	// If all items fit, show them all
	if totalItems <= pageSize {
		return 0, totalItems
	}

	// Use windowStart as the start point
	start = m.windowStart
	end = start + pageSize

	// Clamp to valid bounds
	if start < 0 {
		start = 0
	}
	if end > totalItems {
		end = totalItems
	}

	return start, end
}

// calculateNameWidth computes the width for the name column based on available pane width
func (m Model) calculateNameWidth() (nameWidth int, showAge bool) {
	// Available width for content (excluding borders and padding)
	availWidth := m.width - 6
	if availWidth < 20 {
		availWidth = 20 // Absolute minimum
	}

	// Try full layout: cursor(2) + name + status(10) + age(5) + 3 gaps
	minTotal := colCursor + colMinName + colStatus + colAge + 3

	if availWidth >= minTotal {
		// We can show all columns
		extra := availWidth - minTotal
		return colMinName + extra, true
	}

	// Drop age for narrow panes: cursor(2) + name + status(10) + 2 gaps
	minWithoutAge := colCursor + colMinName + colStatus + 2

	if availWidth >= minWithoutAge {
		extra := availWidth - minWithoutAge
		return colMinName + extra, false
	}

	// Ultra-narrow: give remaining space to name
	return max(8, availWidth-colCursor-colStatus-2), false
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// calculatePageSize returns the number of items that fit in one page
func (m Model) calculatePageSize() int {
	availableHeight := m.height - chromeLines
	if availableHeight < 1 {
		availableHeight = 1
	}
	return availableHeight
}

// adjustWindowForCursor adjusts windowStart to keep cursor visible
func (m *Model) adjustWindowForCursor(namespaces []state.NamespaceInfo) {
	if len(namespaces) == 0 {
		m.windowStart = 0
		return
	}

	pageSize := m.calculatePageSize()

	// If all items fit on screen, no scrolling needed
	if len(namespaces) <= pageSize {
		m.windowStart = 0
		return
	}

	// If cursor is above window, scroll up
	if m.cursor < m.windowStart {
		m.windowStart = m.cursor
	}

	// If cursor is below window, scroll down
	if m.cursor >= m.windowStart+pageSize {
		m.windowStart = m.cursor - pageSize + 1
	}

	// Clamp windowStart to valid range
	if m.windowStart < 0 {
		m.windowStart = 0
	}
	maxStart := len(namespaces) - pageSize
	if m.windowStart > maxStart && maxStart >= 0 {
		m.windowStart = maxStart
	}
}
