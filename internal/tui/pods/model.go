package pods

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/constants"
	"github.com/NlCKDEV/kubectl-medic/internal/util"
)

// debugMode enables debug logging to stderr
const debugMode = false

// Column width constants for pods table
const (
	// Minimum column widths
	colMinName     = 14
	colMinReady    = 5
	colMinStatus   = 10
	colMinRestarts = 4 // "RST" or number
	colMinAge      = 4

	// Preferred column widths (when space available)
	colPrefReady    = 7
	colPrefRestarts = 4
	colPrefAge      = 5

	// Chrome overhead for window calculation (title, sort, header, separator, help, padding)
	chromeLines = 12
)

// Model represents the pod list pane
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

// SortMode defines how pods are sorted
type SortMode int

const (
	SortByName SortMode = iota
	SortByStatus
	SortByRestarts
	SortByAge
)

// NewModel creates a new pods pane model
func NewModel(appState *state.AppState) Model {
	return Model{
		state:    appState,
		cursor:   0,
		sortMode: SortByName,
		active:   false,
	}
}

// Init initializes the pods model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the pods pane
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
				m.adjustWindowForCursor(m.getFilteredAndSortedPods())
			}
		case "down", "j":
			pods := m.getFilteredAndSortedPods()
			if m.cursor < len(pods)-1 {
				m.cursor++
				m.adjustWindowForCursor(pods)
			}
		case "pgup":
			pageSize := m.calculatePageSize()
			if m.cursor >= pageSize {
				m.cursor -= pageSize
			} else {
				m.cursor = 0
			}
			m.adjustWindowForCursor(m.getFilteredAndSortedPods())
		case "pgdown":
			pods := m.getFilteredAndSortedPods()
			pageSize := m.calculatePageSize()
			if m.cursor+pageSize < len(pods) {
				m.cursor += pageSize
			} else if len(pods) > 0 {
				m.cursor = len(pods) - 1
			}
			m.adjustWindowForCursor(pods)
		case "enter", "d":
			// Select pod and show details (with smart caching)
			pods := m.getFilteredAndSortedPods()
			if m.cursor < len(pods) {
				selectedPod := pods[m.cursor].Name
				m.state.SelectedPod = selectedPod
				m.state.CurrentView = state.ViewDetails

				// Smart caching: Only load if not already loading and cache is stale
				needsLoad := !m.state.PodDetails.Loading &&
					(!m.state.PodDetails.Loaded || m.state.PodDetails.Data.Pod == nil || m.state.PodDetails.Data.Pod.Name != selectedPod)

				if needsLoad {
					m.state.PodDetails.Loading = true
					m.state.LoadingPodDetails = true // DEPRECATED
					m.state.PodDetailsError = ""     // DEPRECATED
				}
			}
		case "x":
			// Run diagnostics on selected pod (with smart caching)
			pods := m.getFilteredAndSortedPods()
			if m.cursor < len(pods) {
				selectedPod := pods[m.cursor].Name
				m.state.SelectedPod = selectedPod
				m.state.CurrentView = state.ViewDiagnostics

				// Smart caching: Only load if not already loading and cache is stale
				needsLoad := !m.state.PodDetails.Loading &&
					(!m.state.PodDetails.Loaded || m.state.PodDetails.Data.Pod == nil || m.state.PodDetails.Data.Pod.Name != selectedPod)

				if needsLoad {
					m.state.PodDetails.Loading = true
					m.state.LoadingPodDetails = true // DEPRECATED
					m.state.PodDetailsError = ""     // DEPRECATED
				}
				// If already loaded for this pod, diagnostics are already computed - just switch view
			}
		case "l":
			// View logs for selected pod (with smart caching)
			pods := m.getFilteredAndSortedPods()
			if m.cursor < len(pods) {
				selectedPod := pods[m.cursor].Name
				m.state.SelectedPod = selectedPod
				m.state.CurrentView = state.ViewLogs

				// Smart caching: Check if logs are cached for this pod/container
				// For now, always load logs (container selection may have changed)
				// TODO: Implement per-container caching based on restart count
				needsLoad := !m.state.Logs.Loading
				if needsLoad {
					m.state.Logs.Loading = true
					m.state.LoadingLogs = true // DEPRECATED
					m.state.LogsError = ""     // DEPRECATED
				}
			}
		case "s":
			// Cycle sort mode
			m.sortMode = (m.sortMode + 1) % 4
			m.cursor = 0
			m.windowStart = 0
		case "/":
			// Enter filter mode
			m.isFiltering = true
			m.filter = ""
		}
	}

	return m, nil
}

// View renders the pods pane
func (m Model) View() string {
	var b strings.Builder

	// Content width = pane width minus border and padding overhead
	contentWidth := max(m.width-constants.PaneHorizontalOverhead, 20)

	// Title with inline sort indicator (compact layout)
	nsDisplay := m.state.SelectedNamespace
	if nsDisplay == "" {
		nsDisplay = "none"
	}
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] PodsView: SelectedNS=%s, state.Pods=%d, LoadingPods=%v\n",
			m.state.SelectedNamespace, len(m.state.Pods.Data), m.state.LoadingPods)
	}
	sortIndicator := ""
	switch m.sortMode {
	case SortByName:
		sortIndicator = "↑"
	case SortByStatus:
		sortIndicator = "st"
	case SortByRestarts:
		sortIndicator = "rst"
	case SortByAge:
		sortIndicator = "age"
	}
	title := theme.TitleStyle.Render(fmt.Sprintf("Pods (%s) [%s]", util.Ellipsize(nsDisplay, 20), sortIndicator))
	b.WriteString(title + "\n\n")

	// Handle different states explicitly
	if m.state.LoadingPods {
		// Loading state
		b.WriteString(theme.LoadingStyle.Render(fmt.Sprintf("⏳ Loading pods for %s...\n", nsDisplay)))
	} else if m.state.PodsError != "" {
		// Error state
		b.WriteString(theme.StatusErrorStyle.Render("✗ Error loading pods:\n"))
		b.WriteString(theme.EmptyStyle.Render(fmt.Sprintf("  %s\n", util.Ellipsize(m.state.PodsError, 60))))
	} else if m.state.SelectedNamespace == "" {
		// No namespace selected
		b.WriteString(theme.EmptyStyle.Render("Select a namespace to view pods\n\n"))
		b.WriteString(theme.HelpStyle.Render("Press Tab to switch to Namespaces pane"))
	} else {
		// Normal state: show pods table
		pods := m.getFilteredAndSortedPods()

		// Calculate dynamic column widths using content width
		colWidths := m.calculateColumnWidths(contentWidth)

		// Build header using unified row builder
		// Use plain text (no HelpStyle) to avoid italic rendering issues
		headerRow := m.buildTableRow(
			"  ", // Gutter placeholder for header
			"NAME", colWidths.name,
			"READY", colWidths.ready,
			"STATUS", colWidths.status,
			"RST", colWidths.restarts,
			"AGE", colWidths.age,
			colWidths,
			false, // not selected
			"",    // no raw status for header
			0,     // no restarts for header
		)
		b.WriteString(headerRow + "\n")

		// Separator line - calculate total row width
		sepWidth := m.calculateRowWidth(colWidths)
		b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", sepWidth)) + "\n")

		// Pod list or empty message
		if len(pods) == 0 {
			if m.filter != "" {
				b.WriteString("\n" + theme.EmptyStyle.Render("No pods match filter '"+m.filter+"'"))
			} else {
				b.WriteString("\n" + theme.EmptyStyle.Render("No pods in this namespace"))
			}
		} else {
			// Show scroll indicators if content extends beyond window
			start, end := m.calculateVisibleWindow(len(pods))
			if start > 0 {
				b.WriteString(theme.HelpStyle.Render("  ↑ more above...\n"))
			}

			// Windowed rendering: only render visible items
			for i := start; i < end; i++ {
				pod := pods[i]

				// Selection gutter: "▶ " or "  " (fixed width)
				gutter := "  "
				if i == m.cursor {
					gutter = "▶ "
				}

				// Build row using unified builder
				row := m.buildTableRow(
					gutter,
					pod.Name, colWidths.name,
					pod.Ready, colWidths.ready,
					pod.Status, colWidths.status,
					fmt.Sprintf("%d", pod.Restarts), colWidths.restarts,
					pod.Age, colWidths.age,
					colWidths,
					i == m.cursor,
					pod.Status,
					pod.Restarts,
				)

				b.WriteString(row + "\n")
			}

			// Show "more below" indicator if there are more items
			if end < len(pods) {
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
		help := util.FormatHelpLineCentered(contentWidth, true, "Enter apply", "Esc cancel")
		b.WriteString(theme.HelpStyle.Render(help))
	} else {
		// Normal mode: full help with priorities (most important first)
		help := util.FormatHelpLineCentered(contentWidth, true,
			"↑/↓ move",
			"Enter details",
			"x diagnose",
			"l logs",
			"/ filter",
			"s sort")
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
// The gutter is SEPARATE from the name column, ensuring selection never shifts content.
func (m Model) buildTableRow(gutter string, name string, nameWidth int, ready string, readyWidth int, status string, statusWidth int, restarts string, restartsWidth int, age string, ageWidth int, colWidths columnWidths, isSelected bool, rawStatus string, rawRestarts int) string {
	var parts []string

	// GUTTER + NAME: Fixed gutter width, then name padded to remaining width
	// Name width already includes gutter (SelectionGutterWidth)
	nameText := name
	actualNameWidth := nameWidth - constants.SelectionGutterWidth
	if len(nameText) > actualNameWidth {
		nameText = util.Ellipsize(nameText, actualNameWidth)
	}
	nameText = util.PadRight(nameText, actualNameWidth)
	parts = append(parts, gutter+nameText)

	// READY column - only if visible
	if readyWidth > 0 {
		readyText := ready
		if len(readyText) > readyWidth {
			readyText = readyText[:readyWidth]
		}
		readyText = util.PadRight(readyText, readyWidth)
		parts = append(parts, readyText)
	}

	// STATUS column - always visible, pad BEFORE styling
	statusText := status
	if len(statusText) > statusWidth {
		statusText = util.Ellipsize(statusText, statusWidth)
	}
	statusText = util.PadRight(statusText, statusWidth)
	// Apply status color (no padding in style)
	if rawStatus != "" {
		statusText = theme.StatusStyle(rawStatus).Render(statusText)
	}
	parts = append(parts, statusText)

	// RESTARTS column - only if visible
	if colWidths.showRestarts && restartsWidth > 0 {
		restartsText := restarts
		if len(restartsText) > restartsWidth {
			restartsText = restartsText[:restartsWidth]
		}
		restartsText = util.PadLeft(restartsText, restartsWidth)
		// Apply warning style for high restart counts
		if rawRestarts > 5 {
			restartsText = theme.StatusWarnStyle.Render(restartsText)
		}
		parts = append(parts, restartsText)
	}

	// AGE column - only if visible
	if colWidths.showAge && ageWidth > 0 {
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

// calculateRowWidth returns the total width of a row given column widths
func (m Model) calculateRowWidth(colWidths columnWidths) int {
	// Start with gutter + name (name includes gutter in its width)
	width := colWidths.name
	gaps := 0

	if colWidths.ready > 0 {
		width += colWidths.ready
		gaps++
	}

	width += colWidths.status
	gaps++

	if colWidths.showRestarts {
		width += colWidths.restarts
		gaps++
	}

	if colWidths.showAge {
		width += colWidths.age
		gaps++
	}

	return width + gaps
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

// getFilteredAndSortedPods returns pods for the selected namespace, with filtering and sorting applied
func (m Model) getFilteredAndSortedPods() []state.PodInfo {
	// Get pods for selected namespace
	pods := m.state.GetPodsInNamespace(m.state.SelectedNamespace)

	// Apply filter
	result := make([]state.PodInfo, 0, len(pods))
	for _, pod := range pods {
		if m.filter == "" || strings.Contains(strings.ToLower(pod.Name), strings.ToLower(m.filter)) {
			result = append(result, pod)
		}
	}

	// Apply sorting
	switch m.sortMode {
	case SortByName:
		sort.Slice(result, func(i, j int) bool {
			return result[i].Name < result[j].Name
		})
	case SortByStatus:
		// Sort by status priority: Error > Pending/Unknown > Running/Succeeded
		sort.Slice(result, func(i, j int) bool {
			priorityI := getStatusPriority(result[i].Status)
			priorityJ := getStatusPriority(result[j].Status)
			if priorityI != priorityJ {
				return priorityI < priorityJ
			}
			return result[i].Name < result[j].Name
		})
	case SortByRestarts:
		// Sort by restarts descending (highest first)
		sort.Slice(result, func(i, j int) bool {
			if result[i].Restarts != result[j].Restarts {
				return result[i].Restarts > result[j].Restarts
			}
			return result[i].Name < result[j].Name
		})
	case SortByAge:
		// Sort by age (we have string format like "5d", "2h", etc.)
		// For simplicity, just sort alphabetically - proper time parsing would be better
		sort.Slice(result, func(i, j int) bool {
			return result[i].Age < result[j].Age
		})
	}

	return result
}

// getStatusPriority returns a priority value for sorting by status
// Lower values = higher priority (shown first)
func getStatusPriority(status string) int {
	switch status {
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff", "ErrImagePull":
		return 0 // Errors first
	case "Pending", "Unknown", "ContainerCreating":
		return 1 // Warnings/pending second
	case "Running":
		return 2 // Running third
	case "Succeeded", "Completed":
		return 3 // Succeeded last
	default:
		return 2
	}
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

// columnWidths holds the calculated widths for each column in the pods table
type columnWidths struct {
	name     int
	ready    int
	status   int
	restarts int
	age      int

	// Visibility flags for very narrow panes (column dropping)
	showRestarts bool
	showAge      bool
}

// calculateColumnWidths computes dynamic column widths based on content width.
// contentWidth should already have PaneHorizontalOverhead subtracted.
// Ensures single-line headers and rows by guaranteeing consistent column sizes.
// Drops lower-priority columns (AGE, RESTARTS) for very narrow panes.
func (m Model) calculateColumnWidths(contentWidth int) columnWidths {
	// Row structure: [gutter+name] [ready] [status] [restarts] [age]
	// Gutter is SelectionGutterWidth (2), included in name column
	gutter := constants.SelectionGutterWidth

	// Try full 5-column layout: gutter+name + ready + status + restarts + age + 4 gaps
	minTotal5Col := gutter + colMinName + colMinReady + colMinStatus + colMinRestarts + colMinAge + 4

	if contentWidth >= minTotal5Col {
		// Calculate extra space to distribute
		extra := contentWidth - minTotal5Col

		// Give most extra space to NAME (70%), some to STATUS (30%)
		nameExtra := (extra * 70) / 100
		statusExtra := (extra * 30) / 100

		return columnWidths{
			name:         gutter + colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     colPrefRestarts,
			age:          colPrefAge,
			showRestarts: true,
			showAge:      true,
		}
	}

	// Try 4-column layout (drop AGE): gutter+name + ready + status + restarts + 3 gaps
	minTotal4Col := gutter + colMinName + colMinReady + colMinStatus + colMinRestarts + 3

	if contentWidth >= minTotal4Col {
		extra := contentWidth - minTotal4Col
		nameExtra := (extra * 70) / 100
		statusExtra := (extra * 30) / 100

		return columnWidths{
			name:         gutter + colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     colPrefRestarts,
			age:          0, // Not shown
			showRestarts: true,
			showAge:      false,
		}
	}

	// Try 3-column layout (drop AGE and RESTARTS): gutter+name + ready + status + 2 gaps
	minTotal3Col := gutter + colMinName + colMinReady + colMinStatus + 2

	if contentWidth >= minTotal3Col {
		extra := contentWidth - minTotal3Col
		nameExtra := (extra * 60) / 100
		statusExtra := (extra * 40) / 100

		return columnWidths{
			name:         gutter + colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     0, // Not shown
			age:          0, // Not shown
			showRestarts: false,
			showAge:      false,
		}
	}

	// Ultra-narrow: just NAME, STATUS (drop READY too)
	// Row structure: gutter+name + gap(1) + status = contentWidth
	// Available for name+status: contentWidth - gutter - gap
	available := contentWidth - gutter - 1
	nameExtra := (available * 60) / 100
	statusExtra := (available * 40) / 100

	return columnWidths{
		name:         gutter + max(8, colMinName+nameExtra),
		ready:        0, // Not shown
		status:       max(10, colMinStatus+statusExtra),
		restarts:     0, // Not shown
		age:          0, // Not shown
		showRestarts: false,
		showAge:      false,
	}
}

// calculatePageSize returns the number of items that fit in one page
func (m Model) calculatePageSize() int {
	return max(m.height-chromeLines, 1)
}

// adjustWindowForCursor adjusts windowStart to keep cursor visible
func (m *Model) adjustWindowForCursor(pods []state.PodInfo) {
	if len(pods) == 0 {
		m.windowStart = 0
		return
	}

	pageSize := m.calculatePageSize()

	// If all items fit on screen, no scrolling needed
	if len(pods) <= pageSize {
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
	maxStart := len(pods) - pageSize
	if m.windowStart > maxStart && maxStart >= 0 {
		m.windowStart = maxStart
	}
}
