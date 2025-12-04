package pods

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
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

	// Chrome overhead for window calculation
	chromeLines = 12

	// Spaces between columns
	columnGaps = 4 // 4 single-space gaps between 5 columns
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

	// Title
	nsDisplay := m.state.SelectedNamespace
	if nsDisplay == "" {
		nsDisplay = "none"
	}
	if debugMode {
		fmt.Fprintf(os.Stderr, "[DEBUG] PodsView: SelectedNS=%s, state.Pods=%d, LoadingPods=%v\n",
			m.state.SelectedNamespace, len(m.state.Pods.Data), m.state.LoadingPods)
	}
	title := theme.TitleStyle.Render(fmt.Sprintf("Pods (%s)", util.Ellipsize(nsDisplay, 25)))
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

		// Show sort mode indicator
		sortModeText := ""
		switch m.sortMode {
		case SortByName:
			sortModeText = "Sort: Name"
		case SortByStatus:
			sortModeText = "Sort: Status"
		case SortByRestarts:
			sortModeText = "Sort: Restarts ↓"
		case SortByAge:
			sortModeText = "Sort: Age"
		}
		b.WriteString(theme.HelpStyle.Render(sortModeText) + "\n\n")

		// Calculate dynamic column widths based on available space
		colWidths := m.calculateColumnWidths()

		// Column headers - use short labels for compact display
		headerLabel := func(s string, width int) string {
			if len(s) > width {
				s = s[:width]
			}
			return util.PadRight(s, width)
		}

		// Build header with explicit widths - respect column visibility
		var headerParts []string
		headerParts = append(headerParts, headerLabel("NAME", colWidths.name))

		if colWidths.ready > 0 {
			headerParts = append(headerParts, headerLabel("READY", colWidths.ready))
		}

		headerParts = append(headerParts, headerLabel("STATUS", colWidths.status))

		if colWidths.showRestarts {
			headerParts = append(headerParts, headerLabel("RST", colWidths.restarts))
		}

		if colWidths.showAge {
			headerParts = append(headerParts, headerLabel("AGE", colWidths.age))
		}

		// BUGFIX: Add 2-char prefix to header to match row alignment
		header := "  " + strings.Join(headerParts, " ")
		b.WriteString(theme.HelpStyle.Render(header) + "\n")

		// Separator line - calculate actual width from visible columns plus 2-char prefix
		sepWidth := colWidths.name + colWidths.status
		gaps := 1 // NAME and STATUS always visible, so at least 1 gap

		if colWidths.ready > 0 {
			sepWidth += colWidths.ready
			gaps++
		}
		if colWidths.showRestarts {
			sepWidth += colWidths.restarts
			gaps++
		}
		if colWidths.showAge {
			sepWidth += colWidths.age
			gaps++
		}

		sepWidth += (gaps - 1) // Add gap spaces
		sepWidth += 2           // Add prefix width to match header/rows
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
				// NAME column with cursor
				cursorPrefix := "  "
				if i == m.cursor {
					cursorPrefix = "▶ "
				}

				// Truncate name to fit, accounting for cursor (2 chars)
				nameWidth := colWidths.name - 2
				podName := pod.Name
				if len(podName) > nameWidth {
					podName = util.Ellipsize(podName, nameWidth)
				}
				// Pad the combined cursor+name to full column width
				nameCol := cursorPrefix + podName
				nameCol = util.PadRight(nameCol, colWidths.name)

				// Build row parts respecting column visibility
				var rowParts []string
				rowParts = append(rowParts, nameCol)

				// READY column - only if visible
				if colWidths.ready > 0 {
					readyText := pod.Ready
					if len(readyText) > colWidths.ready {
						readyText = readyText[:colWidths.ready]
					}
					readyCol := util.PadRight(readyText, colWidths.ready)
					rowParts = append(rowParts, readyCol)
				}

				// STATUS column - always visible, truncate BEFORE styling, pad BEFORE styling
				statusText := pod.Status
				if len(statusText) > colWidths.status {
					statusText = util.Ellipsize(statusText, colWidths.status)
				}
				statusText = util.PadRight(statusText, colWidths.status)
				statusCol := theme.StatusStyle(pod.Status).Render(statusText)
				rowParts = append(rowParts, statusCol)

				// RESTARTS column - only if visible
				if colWidths.showRestarts {
					restartsText := fmt.Sprintf("%d", pod.Restarts)
					if len(restartsText) > colWidths.restarts {
						restartsText = restartsText[:colWidths.restarts]
					}
					restartsCol := util.PadLeft(restartsText, colWidths.restarts)
					if pod.Restarts > 5 {
						restartsCol = theme.StatusWarnStyle.Render(restartsCol)
					}
					rowParts = append(rowParts, restartsCol)
				}

				// AGE column - only if visible
				if colWidths.showAge {
					ageText := pod.Age
					if len(ageText) > colWidths.age {
						ageText = ageText[:colWidths.age]
					}
					ageCol := util.PadRight(ageText, colWidths.age)
					rowParts = append(rowParts, ageCol)
				}

				// Build final row with single spaces - guarantees single line
				line := strings.Join(rowParts, " ")

				// Apply selection styling to entire row if selected
				if i == m.cursor {
					line = theme.ListItemSelectedStyle.Render(line)
				}

				b.WriteString(line + "\n")
			}

			// Show "more below" indicator if there are more items
			if end < len(pods) {
				b.WriteString(theme.HelpStyle.Render("  ↓ more below...\n"))
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
		// Normal mode: full help with priorities (most important first)
		help := util.FormatHelpLine(m.width-8,
			"↑/↓ move",
			"Enter details",
			"x diagnose",
			"l logs",
			"/ filter",
			"s sort")
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

// calculateColumnWidths computes dynamic column widths based on available pane width
// Ensures single-line headers and rows by guaranteeing consistent column sizes
// Drops lower-priority columns (AGE, RESTARTS) for very narrow panes
func (m Model) calculateColumnWidths() columnWidths {
	// Available width for content (excluding borders and padding)
	availWidth := m.width - 6
	if availWidth < 20 {
		availWidth = 20 // Absolute minimum
	}

	// Try full 5-column layout first (NAME, READY, STATUS, RESTARTS, AGE)
	minTotal5Col := colMinName + colMinReady + colMinStatus + colMinRestarts + colMinAge + columnGaps

	// If we can fit all 5 columns at minimum widths
	if availWidth >= minTotal5Col {
		// Calculate extra space to distribute
		extra := availWidth - minTotal5Col

		// Give most extra space to NAME (70%), some to STATUS (30%)
		nameExtra := (extra * 70) / 100
		statusExtra := (extra * 30) / 100

		return columnWidths{
			name:         colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     colPrefRestarts,
			age:          colPrefAge,
			showRestarts: true,
			showAge:      true,
		}
	}

	// Try 4-column layout (drop AGE column)
	minTotal4Col := colMinName + colMinReady + colMinStatus + colMinRestarts + 3 // 3 gaps

	if availWidth >= minTotal4Col {
		extra := availWidth - minTotal4Col
		nameExtra := (extra * 70) / 100
		statusExtra := (extra * 30) / 100

		return columnWidths{
			name:         colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     colPrefRestarts,
			age:          0, // Not shown
			showRestarts: true,
			showAge:      false,
		}
	}

	// Try 3-column layout (drop AGE and RESTARTS)
	minTotal3Col := colMinName + colMinReady + colMinStatus + 2 // 2 gaps

	if availWidth >= minTotal3Col {
		extra := availWidth - minTotal3Col
		nameExtra := (extra * 60) / 100
		statusExtra := (extra * 40) / 100

		return columnWidths{
			name:         colMinName + nameExtra,
			ready:        colPrefReady,
			status:       colMinStatus + statusExtra,
			restarts:     0, // Not shown
			age:          0, // Not shown
			showRestarts: false,
			showAge:      false,
		}
	}

	// Ultra-narrow: just NAME, STATUS (drop READY too)
	// Minimum viable: NAME (10) + STATUS (10) + 1 gap = 21
	return columnWidths{
		name:         max(10, (availWidth*60)/100),
		ready:        0, // Not shown
		status:       max(10, (availWidth*40)/100),
		restarts:     0, // Not shown
		age:          0, // Not shown
		showRestarts: false,
		showAge:      false,
	}
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
