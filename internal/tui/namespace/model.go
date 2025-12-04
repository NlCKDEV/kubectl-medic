package namespace

import (
	"fmt"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/constants"
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
	// Fixed column widths
	colMinName = 12 // Minimum width for namespace name
	colStatus  = 10 // "Terminating" is 11, but we'll truncate if needed
	colAge     = 5  // "100d" format

	// Chrome overhead for window calculation (title, sort indicator, header, help, borders)
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
				m.state.Pods.Data = []state.PodInfo{} // Clear previous pods
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
			// Show namespace health summary (with smart caching)
			namespaces := m.getFilteredAndSortedNamespaces()
			if m.cursor < len(namespaces) {
				selectedNS := namespaces[m.cursor].Name

				// Smart caching: only trigger load if not already loading and cache is stale
				if !m.state.Health.Loading {
					// Check if we need to reload (different namespace or no data)
					// BUGFIX: Check against Health.Data.Namespace, not SelectedNamespace
					needsLoad := m.state.Health.Data == nil ||
						m.state.Health.Data.Namespace != selectedNS

					m.state.SelectedNamespace = selectedNS
					m.state.CurrentView = state.ViewNamespaceHealth

					if needsLoad {
						m.state.Health.Loading = true
						m.state.LoadingNamespaceHealth = true // DEPRECATED
						m.state.NamespaceHealthError = ""     // DEPRECATED
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

	// Content width = pane width minus border and padding overhead
	contentWidth := max(m.width-constants.PaneHorizontalOverhead, 20)

	// Title with inline sort indicator (compact layout)
	sortIndicator := ""
	switch m.sortMode {
	case SortNameAsc:
		sortIndicator = "↑"
	case SortNameDesc:
		sortIndicator = "↓"
	case SortStatus:
		sortIndicator = "st"
	}
	title := theme.TitleStyle.Render(fmt.Sprintf("Namespaces [%s]", sortIndicator))
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
				b.WriteString(theme.EmptyStyle.Render("No namespaces match filter '" + m.filter + "'"))
			} else {
				b.WriteString(theme.EmptyStyle.Render("No namespaces found"))
			}
		} else {
			// Calculate column widths using content width
			nameWidth, showAge := m.calculateColumnWidths(contentWidth)

			// Build header using same buildRow function for consistency
			// Use plain text (no HelpStyle) to avoid italic rendering issues
			headerRow := m.buildTableRow(
				"  ", // Gutter placeholder for header (matches selection gutter width)
				"NAME", nameWidth,
				"STATUS", colStatus,
				"AGE", colAge,
				showAge,
				false, // not selected
				"",    // no status for header
			)
			b.WriteString(headerRow + "\n")

			// Separator line for visual distinction
			// Total row width: gutter(2) + nameWidth + space(1) + colStatus + space(1) + [colAge]
			sepWidth := constants.SelectionGutterWidth + nameWidth + 1 + colStatus
			if showAge {
				sepWidth += 1 + colAge
			}
			b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", sepWidth)) + "\n")

			// Show scroll indicators if content extends beyond window
			start, end := m.calculateVisibleWindow(len(namespaces))
			if start > 0 {
				b.WriteString(theme.HelpStyle.Render("  ↑ more above...\n"))
			}

			// Windowed rendering: only render visible items
			for i := start; i < end; i++ {
				ns := namespaces[i]

				// Selection gutter: "▶ " or "  " (fixed width)
				gutter := "  "
				if i == m.cursor {
					gutter = "▶ "
				}

				// Build row using unified builder
				row := m.buildTableRow(
					gutter,
					ns.Name, nameWidth,
					ns.Status, colStatus,
					ns.Age, colAge,
					showAge,
					i == m.cursor,
					ns.Status,
				)

				b.WriteString(row + "\n")
			}

			// Show "more below" indicator if there are more items
			if end < len(namespaces) {
				b.WriteString(theme.HelpStyle.Render("  ↓ more below...\n"))
			}
		}
	}

	// Help text - use content width for formatting
	b.WriteString("\n")
	if m.isFiltering {
		filterPrompt := fmt.Sprintf("Filter: %s_", m.filter)
		b.WriteString(theme.StatusInfoStyle.Render(filterPrompt) + "\n")
		// Filtering mode: simpler help
		help := util.FormatHelpLine(contentWidth, "Enter apply", "Esc cancel")
		b.WriteString(theme.HelpStyle.Render(help))
	} else {
		// Normal mode: full help with priorities
		help := util.FormatHelpLine(contentWidth,
			"↑/↓ move",
			"Enter select",
			"/ filter",
			"s sort",
			"h health")
		b.WriteString(theme.HelpStyle.Render(help))
	}

	// Apply pane styling with correct width
	content := b.String()
	style := theme.PaneStyle
	if m.active {
		style = theme.PaneStyleActive
	}

	return style.
		Width(m.width-constants.PaneHorizontalOverhead).
		Height(m.height-constants.PaneVerticalOverhead).
		Padding(0, 0).
		Render(content)
}

// buildTableRow constructs a single table row with consistent column alignment.
// This unified builder ensures headers and data rows have identical widths.
// The gutter is SEPARATE from the name column, ensuring selection never shifts content.
func (m Model) buildTableRow(gutter string, name string, nameWidth int, status string, statusWidth int, age string, ageWidth int, showAge bool, isSelected bool, rawStatus string) string {
	var parts []string

	// GUTTER: Fixed width, never changes with selection
	// Already the correct width (SelectionGutterWidth = 2)
	gutterCol := gutter

	// NAME column: truncate then pad to exact width
	nameText := name
	if len(nameText) > nameWidth {
		nameText = util.Ellipsize(nameText, nameWidth)
	}
	nameText = util.PadRight(nameText, nameWidth)

	// First part: gutter + name (combined but gutter is fixed)
	parts = append(parts, gutterCol+nameText)

	// STATUS column: pad BEFORE styling, apply color-only style
	statusText := status
	if len(statusText) > statusWidth {
		statusText = statusText[:statusWidth]
	}
	statusText = util.PadRight(statusText, statusWidth)

	// Apply status color (no padding in style)
	if rawStatus != "" {
		statusStyle := theme.StatusInfoStyle
		if rawStatus != "Active" {
			statusStyle = theme.StatusWarnStyle
		}
		statusText = statusStyle.Render(statusText)
	}
	parts = append(parts, statusText)

	// AGE column: only if visible
	if showAge {
		ageText := age
		if len(ageText) > ageWidth {
			ageText = ageText[:ageWidth]
		}
		ageText = util.PadRight(ageText, ageWidth)
		parts = append(parts, ageText)
	}

	// Join with single space separators
	row := strings.Join(parts, " ")

	// Apply selection styling (color + bold only, NO padding)
	if isSelected {
		row = theme.ListItemSelectedStyle.Render(row)
	}

	return row
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
	result := make([]state.NamespaceInfo, 0, len(m.state.Namespaces.Data))

	// Apply filter
	for _, ns := range m.state.Namespaces.Data {
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

// calculateColumnWidths computes the width for the name column based on content width.
// contentWidth should already have PaneHorizontalOverhead subtracted.
func (m Model) calculateColumnWidths(contentWidth int) (nameWidth int, showAge bool) {
	// Row structure: [gutter][name] [status] [age]
	// Gutter is SelectionGutterWidth (2)
	// Gaps: 1 between gutter+name and status, 1 between status and age (if shown)
	gutter := constants.SelectionGutterWidth

	// Try full layout: gutter(2) + name + status(10) + age(5) + 2 gaps
	minTotal := gutter + colMinName + colStatus + colAge + 2

	if contentWidth >= minTotal {
		// We can show all columns - give extra space to name
		extra := contentWidth - minTotal
		return colMinName + extra, true
	}

	// Drop age for narrow panes: gutter(2) + name + status(10) + 1 gap
	minWithoutAge := gutter + colMinName + colStatus + 1

	if contentWidth >= minWithoutAge {
		extra := contentWidth - minWithoutAge
		return colMinName + extra, false
	}

	// Ultra-narrow: give remaining space to name
	return max(8, contentWidth-gutter-colStatus-1), false
}

// calculatePageSize returns the number of items that fit in one page
func (m Model) calculatePageSize() int {
	return max(m.height-chromeLines, 1)
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
