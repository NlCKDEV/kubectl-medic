package details

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/analysis"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/constants"
	"github.com/NlCKDEV/kubectl-medic/internal/util"
)

// Model represents the details/diagnostics/logs pane (right side)
type Model struct {
	state    *state.AppState
	active   bool
	width    int
	height   int
	viewport viewport.Model
	ready    bool // Viewport initialized
}

// NewModel creates a new details pane model
func NewModel(appState *state.AppState) Model {
	vp := viewport.New(80, 20) // Will be resized properly in SetSize
	vp.HighPerformanceRendering = false

	return Model{
		state:    appState,
		active:   false,
		viewport: vp,
		ready:    false,
	}
}

// Init initializes the details model
func (m Model) Init() tea.Cmd {
	return nil
}

// Update handles messages for the details pane
func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	var cmd tea.Cmd

	if !m.active {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "c":
			// In diagnostics view: toggle to copy commands mode
			if m.state.CurrentView == state.ViewDiagnostics {
				m.state.CurrentView = state.ViewCopyCommands
				return m, nil
			}
			// In logs view: switch container
			if m.state.CurrentView == state.ViewLogs && len(m.state.CurrentContainers) > 0 {
				m.state.SelectedContainer = (m.state.SelectedContainer + 1) % len(m.state.CurrentContainers)
				// Trigger reload of logs for new container
				m.state.LoadingLogs = true
			}
			return m, nil
		case "x":
			// In copy commands view: return to diagnostics
			if m.state.CurrentView == state.ViewCopyCommands {
				m.state.CurrentView = state.ViewDiagnostics
				return m, nil
			}
		// Follow mode disabled for v1.0 - requires complex goroutine management
		// and risks blocking the TUI. Will be implemented properly in v1.1.
		// See Phase 13 documentation for details.
		default:
			// Let viewport handle scrolling (up/down/k/j/pgup/pgdn)
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
	}

	// Let viewport handle other messages
	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

// View renders the details pane based on current view mode
// Uses viewport for scrollable content
func (m Model) View() string {
	if !m.ready {
		return theme.LoadingStyle.Render("Initializing...")
	}

	// Generate content based on current view mode
	var content string
	switch m.state.CurrentView {
	case state.ViewDetails:
		content = m.renderDetails()
	case state.ViewDiagnostics:
		content = m.renderDiagnostics()
	case state.ViewLogs:
		content = m.renderLogs()
	case state.ViewNamespaceHealth:
		content = m.renderNamespaceHealth()
	case state.ViewCopyCommands:
		content = m.renderCopyCommands()
	default:
		content = m.renderDetails()
	}

	// Update viewport content
	m.viewport.SetContent(content)

	// Render viewport (scrollable area)
	viewportView := m.viewport.View()

	// Apply pane styling
	style := theme.PaneStyle
	if m.active {
		style = theme.PaneStyleActive
	}

	return style.
		Width(m.width - constants.PaneBorderPadding).
		Height(m.height - constants.PaneBorderPadding).
		Render(viewportView)
}

// renderDetails shows pod details, metadata, and events
func (m Model) renderDetails() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Pod Details")
	b.WriteString(title + "\n\n")

	// Show loading state
	if m.state.LoadingPodDetails {
		b.WriteString(theme.LoadingStyle.Render("⏳ Loading pod details...\n"))
		return b.String()
	}

	// Show error state
	if m.state.PodDetailsError != "" {
		b.WriteString(theme.StatusErrorStyle.Render("✗ Error loading pod details:\n"))
		b.WriteString(theme.EmptyStyle.Render(util.Ellipsize(m.state.PodDetailsError, 60) + "\n"))
		return b.String()
	}

	// Check if pod is selected
	pod := m.state.CurrentPod
	if pod == nil {
		b.WriteString(theme.EmptyStyle.Render("No pod selected\n\n"))
		b.WriteString(theme.HelpStyle.Render("Select a pod from the Pods pane"))
		return b.String()
	}

	// Pod metadata section
	b.WriteString(theme.StatusInfoStyle.Render("Pod") + "\n")
	b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 40)) + "\n")
	b.WriteString(fmt.Sprintf("  Name:       %s\n", pod.Name))
	b.WriteString(fmt.Sprintf("  Namespace:  %s\n", pod.Namespace))
	b.WriteString(fmt.Sprintf("  Status:     %s\n", theme.StatusStyle(string(pod.Status.Phase)).Render(string(pod.Status.Phase))))
	b.WriteString(fmt.Sprintf("  Node:       %s\n", pod.Spec.NodeName))
	b.WriteString(fmt.Sprintf("  IP:         %s\n", pod.Status.PodIP))

	// Containers section
	b.WriteString("\n")
	b.WriteString(theme.StatusInfoStyle.Render("Containers") + "\n")
	b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 40)) + "\n")
	for _, container := range pod.Spec.Containers {
		b.WriteString(fmt.Sprintf("  • %s\n", theme.KeyStyle.Render(container.Name)))
		b.WriteString(fmt.Sprintf("    Image: %s\n", container.Image))

		// Find container status
		for _, status := range pod.Status.ContainerStatuses {
			if status.Name == container.Name {
				b.WriteString(fmt.Sprintf("    Ready: %v\n", status.Ready))
				b.WriteString(fmt.Sprintf("    Restart Count: %d\n", status.RestartCount))

				// Container state
				if status.State.Running != nil {
					b.WriteString(fmt.Sprintf("    State: %s\n", theme.StatusOKStyle.Render("Running")))
				} else if status.State.Waiting != nil {
					b.WriteString(fmt.Sprintf("    State: %s (%s)\n",
						theme.StatusWarnStyle.Render("Waiting"),
						status.State.Waiting.Reason))
				} else if status.State.Terminated != nil {
					b.WriteString(fmt.Sprintf("    State: %s (exit %d)\n",
						theme.StatusErrorStyle.Render("Terminated"),
						status.State.Terminated.ExitCode))
				}
			}
		}
	}

	// Conditions section
	b.WriteString("\n")
	b.WriteString(theme.StatusInfoStyle.Render("Conditions") + "\n")
	b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 40)) + "\n")
	for _, condition := range pod.Status.Conditions {
		statusIcon := "✓"
		statusStyle := theme.StatusOKStyle
		if condition.Status != corev1.ConditionTrue {
			statusIcon = "✗"
			statusStyle = theme.StatusErrorStyle
		}

		b.WriteString(fmt.Sprintf("  %s %s: %s\n",
			statusStyle.Render(statusIcon),
			condition.Type,
			condition.Status))
	}

	// Events section
	b.WriteString("\n")
	b.WriteString(theme.StatusInfoStyle.Render("Recent Events") + "\n")
	b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", 40)) + "\n")
	if len(m.state.CurrentEvents) == 0 {
		b.WriteString("  No events\n")
	} else {
		for i, event := range m.state.CurrentEvents {
			if i >= constants.DetailsMaxRecentEvents {
				break
			}

			eventStyle := theme.StatusInfoStyle
			if event.Type == "Warning" {
				eventStyle = theme.StatusWarnStyle
			}

			b.WriteString(fmt.Sprintf("  %s %s: %s\n",
				eventStyle.Render(event.Type),
				event.Reason,
				event.Message))
			b.WriteString(fmt.Sprintf("    Last seen: %s (x%d)\n", event.LastSeen, event.Count))
		}
	}

	b.WriteString("\n")
	help := util.FormatHelpLine(m.width-constants.DetailsHelpTextOffset,
		"↑/↓ scroll",
		"x diagnostics",
		"l logs")
	b.WriteString(theme.HelpStyle.Render(help))

	return b.String()
}

// renderDiagnostics shows diagnostic results from the diagnostics engine
// Formats output for readability and easy command copying
func (m Model) renderDiagnostics() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Diagnostics")
	b.WriteString(title + "\n\n")

	// Show loading state
	if m.state.LoadingPodDetails {
		b.WriteString(theme.LoadingStyle.Render("⏳ Loading pod details...\n"))
		return b.String()
	}

	// Check if pod is selected
	if m.state.CurrentPod == nil {
		b.WriteString(theme.EmptyStyle.Render("No pod selected\n\n"))
		b.WriteString(theme.HelpStyle.Render("Select a pod from the Pods pane"))
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Pod: %s\n\n", theme.KeyStyle.Render(m.state.CurrentPod.Name)))

	// Check if diagnostics are available
	if len(m.state.Diagnostics) == 0 {
		b.WriteString(theme.LoadingStyle.Render("Running diagnostics...\n"))
		return b.String()
	}

	// Calculate usable width for wrapping (account for borders and padding)
	contentWidth := m.width - 8
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Render diagnostics from engine (now strongly typed)
	for i, diag := range m.state.Diagnostics {

		// Add separator between diagnostics
		if i > 0 {
			b.WriteString("\n" + theme.SeparatorStyle.Render(strings.Repeat("─", min(contentWidth, 60))) + "\n\n")
		}

		// Render severity badge and title
		severityStyle := theme.StatusInfoStyle
		severityText := "[INFO]"
		switch diag.Severity {
		case analysis.SeverityError:
			severityStyle = theme.StatusErrorStyle
			severityText = "[ERROR]"
		case analysis.SeverityWarning:
			severityStyle = theme.StatusWarnStyle
			severityText = "[WARN]"
		case analysis.SeverityInfo:
			severityStyle = theme.StatusInfoStyle
			severityText = "[INFO]"
		}

		b.WriteString(severityStyle.Render(severityText) + " " + diag.Title + "\n\n")

		// Description - wrap to content width
		b.WriteString(wrapText(diag.Description, contentWidth) + "\n")

		// Suggested commands - format for easy copying with natural wrapping
		if len(diag.SuggestedCommands) > 0 {
			b.WriteString("\n" + theme.HelpStyle.Render("Suggested commands:") + "\n")
			for _, cmd := range diag.SuggestedCommands {
				// Check if it's a comment (starts with #)
				if strings.HasPrefix(cmd, "#") {
					// Render comment in subdued color, still selectable
					// Allow wrapping for long comments
					wrapped := wrapCommand(cmd, contentWidth, "  ")
					b.WriteString(theme.HelpStyle.Render(wrapped) + "\n")
				} else {
					// Render command with simple prefix for visual clarity
					// Allow natural wrapping if command is too long
					// Use plain text for easy selection - no styling inside the command
					wrapped := wrapCommand(cmd, contentWidth, "  $ ")
					b.WriteString(wrapped + "\n")
				}
			}
		}
	}

	b.WriteString("\n")
	help := util.FormatHelpLine(m.width-8,
		"↑/↓ scroll",
		"c copy mode",
		"d details",
		"l logs")
	b.WriteString(theme.HelpStyle.Render(help))

	return b.String()
}

// renderCopyCommands shows all diagnostic commands in a minimal, copy-friendly format
func (m Model) renderCopyCommands() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Copy Commands")
	b.WriteString(title + "\n\n")

	// Show loading state
	if m.state.LoadingPodDetails {
		b.WriteString(theme.LoadingStyle.Render("⏳ Loading pod details...\n"))
		return b.String()
	}

	// Check if pod is selected
	if m.state.CurrentPod == nil {
		b.WriteString(theme.EmptyStyle.Render("No pod selected\n\n"))
		b.WriteString(theme.HelpStyle.Render("Select a pod from the Pods pane"))
		return b.String()
	}

	b.WriteString(fmt.Sprintf("Pod: %s\n\n", m.state.CurrentPod.Name))

	// Check if diagnostics are available
	if len(m.state.Diagnostics) == 0 {
		b.WriteString(theme.LoadingStyle.Render("Running diagnostics...\n"))
		return b.String()
	}

	b.WriteString(theme.HelpStyle.Render("Commands ready to copy (select and paste):") + "\n\n")

	// Collect all commands from all diagnostics (now strongly typed)
	commandCount := 0
	for _, diag := range m.state.Diagnostics {

		// Add commands from this diagnostic
		if len(diag.SuggestedCommands) > 0 {
			// Add diagnostic title as a comment
			b.WriteString(fmt.Sprintf("# %s\n", diag.Title))

			for _, cmd := range diag.SuggestedCommands {
				// Skip comment lines (already have context from title)
				if strings.HasPrefix(cmd, "#") {
					continue
				}

				// Render command with NO prefix for easy copy-paste
				// Users can triple-click to select entire line
				b.WriteString(fmt.Sprintf("%s\n", cmd))
				commandCount++
			}

			b.WriteString("\n")
		}
	}

	if commandCount == 0 {
		b.WriteString(theme.EmptyStyle.Render("No commands available\n"))
	}

	b.WriteString("\n")
	help := util.FormatHelpLine(m.width-8,
		"↑/↓ scroll",
		"x back",
		"Triple-click to copy")
	b.WriteString(theme.HelpStyle.Render(help))

	return b.String()
}

// wrapText wraps text to specified width, preserving paragraphs
func wrapText(text string, width int) string {
	if width < 20 {
		width = 20
	}

	// Split into paragraphs (separated by blank lines)
	paragraphs := strings.Split(text, "\n\n")
	var result []string

	for _, para := range paragraphs {
		// Split into lines for cases where text already has newlines
		lines := strings.Split(para, "\n")
		var wrappedPara []string

		for _, line := range lines {
			// Preserve leading spaces (indentation)
			leadingSpaces := len(line) - len(strings.TrimLeft(line, " "))
			trimmedLine := strings.TrimSpace(line)

			if trimmedLine == "" {
				wrappedPara = append(wrappedPara, "")
				continue
			}

			// Simple word wrapping
			words := strings.Fields(trimmedLine)
			var currentLine string
			indent := strings.Repeat(" ", leadingSpaces)

			for _, word := range words {
				testLine := currentLine
				if testLine != "" {
					testLine += " "
				}
				testLine += word

				if len(indent+testLine) > width && currentLine != "" {
					wrappedPara = append(wrappedPara, indent+currentLine)
					currentLine = word
				} else {
					currentLine = testLine
				}
			}

			if currentLine != "" {
				wrappedPara = append(wrappedPara, indent+currentLine)
			}
		}

		result = append(result, strings.Join(wrappedPara, "\n"))
	}

	return strings.Join(result, "\n\n")
}

// wrapCommand wraps a command line with a prefix (like "  $ ") to fit width
// Continuation lines are indented to align after the prefix
// This allows long commands to wrap naturally without being cropped
func wrapCommand(cmd string, width int, prefix string) string {
	if width < 20 {
		width = 20
	}

	// First line gets the prefix
	firstLineWidth := width - len(prefix)
	if firstLineWidth < 10 {
		// Too narrow, just return prefix + command (let it overflow)
		return prefix + cmd
	}

	// Check if command fits on first line
	if len(cmd) <= firstLineWidth {
		return prefix + cmd
	}

	// Need to wrap - break at spaces
	words := strings.Fields(cmd)
	var lines []string
	var currentLine string

	// Continuation indent (align with start of command after prefix)
	contIndent := strings.Repeat(" ", len(prefix))

	for i, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		// First line has different width (accounts for prefix)
		maxWidth := firstLineWidth
		if len(lines) > 0 {
			maxWidth = width - len(contIndent)
		}

		if len(testLine) > maxWidth && currentLine != "" {
			// Line too long, emit current line and start new one
			if len(lines) == 0 {
				lines = append(lines, prefix+currentLine)
			} else {
				lines = append(lines, contIndent+currentLine)
			}
			currentLine = word
		} else {
			currentLine = testLine
		}

		// Last word
		if i == len(words)-1 && currentLine != "" {
			if len(lines) == 0 {
				lines = append(lines, prefix+currentLine)
			} else {
				lines = append(lines, contIndent+currentLine)
			}
		}
	}

	return strings.Join(lines, "\n")
}

// min returns the smaller of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// renderLogs shows container logs
func (m Model) renderLogs() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Logs")
	b.WriteString(title + "\n\n")

	// Show loading state
	if m.state.LoadingLogs {
		b.WriteString(theme.HelpStyle.Render("Loading logs...\n"))
		return b.String()
	}

	// Show error state
	if m.state.LogsError != "" {
		b.WriteString(theme.StatusErrorStyle.Render("Error loading logs:\n"))
		b.WriteString(theme.HelpStyle.Render(m.state.LogsError + "\n"))
		return b.String()
	}

	// Check if pod is selected
	if m.state.SelectedPod == "" {
		b.WriteString(theme.HelpStyle.Render("No pod selected"))
		return b.String()
	}

	// Show container info
	b.WriteString(fmt.Sprintf("Pod: %s\n", theme.KeyStyle.Render(m.state.SelectedPod)))
	if len(m.state.CurrentContainers) > 0 {
		currentContainer := m.state.CurrentContainers[m.state.SelectedContainer]
		b.WriteString(fmt.Sprintf("Container: %s (%d/%d)\n\n",
			theme.KeyStyle.Render(currentContainer),
			m.state.SelectedContainer+1,
			len(m.state.CurrentContainers)))
	} else {
		b.WriteString("Container: none\n\n")
	}

	// Show logs - viewport handles scrolling automatically
	if m.state.CurrentLogs == "" {
		b.WriteString(theme.EmptyStyle.Render("No logs available\n"))
	} else {
		// Simply render all logs - viewport will handle scrolling and display
		b.WriteString(m.state.CurrentLogs)

		// Add log count info
		lineCount := strings.Count(m.state.CurrentLogs, "\n") + 1
		b.WriteString("\n\n")
		b.WriteString(theme.HelpStyle.Render(fmt.Sprintf("Total lines: %d", lineCount)))
	}

	b.WriteString("\n")
	help := util.FormatHelpLine(m.width-8,
		"↑/↓ scroll",
		"c container",
		"d details",
		"x diagnose")
	b.WriteString(theme.HelpStyle.Render(help))

	return b.String()
}

// renderNamespaceHealth shows namespace health summary
func (m Model) renderNamespaceHealth() string {
	var b strings.Builder

	title := theme.TitleStyle.Render("Namespace Health Summary")
	b.WriteString(title + "\n\n")

	// Show loading state
	if m.state.LoadingNamespaceHealth {
		b.WriteString(theme.HelpStyle.Render("Loading namespace health summary...\n"))
		return b.String()
	}

	// Show error state
	if m.state.NamespaceHealthError != "" {
		b.WriteString(theme.StatusErrorStyle.Render("Error loading namespace health:\n"))
		b.WriteString(theme.HelpStyle.Render(m.state.NamespaceHealthError + "\n"))
		return b.String()
	}

	// Check if health data is available
	if m.state.CurrentNamespaceHealth == nil {
		b.WriteString(theme.HelpStyle.Render("No namespace health data available\n\n"))
		b.WriteString(theme.HelpStyle.Render("Press 'h' in the Namespace pane to load health summary"))
		return b.String()
	}

	health := m.state.CurrentNamespaceHealth

	// Namespace header
	b.WriteString(fmt.Sprintf("Namespace: %s\n\n", theme.KeyStyle.Render(health.Namespace)))

	// Pod counts
	b.WriteString(theme.StatusInfoStyle.Render("Pod Summary:\n"))
	b.WriteString(formatHealthRow("Total pods", health.TotalPods, theme.StatusInfoStyle))

	if health.FailingPods > 0 {
		b.WriteString(formatHealthRow("Failing pods", health.FailingPods, theme.StatusErrorStyle))
	} else {
		b.WriteString(formatHealthRow("Failing pods", 0, theme.StatusOKStyle))
	}

	b.WriteString("\n")

	// Issue breakdown
	if health.FailingPods > 0 {
		b.WriteString(theme.StatusInfoStyle.Render("Issues Detected:\n"))

		if health.CrashLoopPods > 0 {
			b.WriteString(formatIssueRow("CrashLoopBackOff", health.CrashLoopPods, "✗", theme.StatusErrorStyle))
		}

		if health.ImagePullPods > 0 {
			b.WriteString(formatIssueRow("Image pull problems", health.ImagePullPods, "✗", theme.StatusErrorStyle))
		}

		if health.SchedulingPods > 0 {
			b.WriteString(formatIssueRow("Scheduling issues", health.SchedulingPods, "✗", theme.StatusErrorStyle))
		}

		if health.ProbeIssuePods > 0 {
			b.WriteString(formatIssueRow("Probe issues", health.ProbeIssuePods, "!", theme.StatusWarnStyle))
		}

		if health.ResourceIssuePods > 0 {
			b.WriteString(formatIssueRow("Resource/OOM issues", health.ResourceIssuePods, "✗", theme.StatusErrorStyle))
		}

		if health.StorageIssuePods > 0 {
			b.WriteString(formatIssueRow("Storage/Volume issues", health.StorageIssuePods, "✗", theme.StatusErrorStyle))
		}

		if health.ConfigIssuePods > 0 {
			b.WriteString(formatIssueRow("Config issues", health.ConfigIssuePods, "✗", theme.StatusErrorStyle))
		}

		if health.InitFailurePods > 0 {
			b.WriteString(formatIssueRow("Init container failures", health.InitFailurePods, "✗", theme.StatusErrorStyle))
		}
	} else {
		// Use formatIssueRow for consistent alignment
		b.WriteString(theme.StatusInfoStyle.Render("Status:\n"))
		b.WriteString(formatIssueRow("All pods healthy", health.TotalPods, "✓", theme.StatusOKStyle))
		b.WriteString("\n")
		b.WriteString(theme.HelpStyle.Render("No issues detected in this namespace.\n"))
	}

	b.WriteString("\n")
	help := util.FormatHelpLine(m.width-8,
		"d details",
		"Esc back")
	b.WriteString(theme.HelpStyle.Render(help))

	return b.String()
}

// SetActive sets whether this pane is active
func (m *Model) SetActive(active bool) {
	m.active = active
}

// SetSize sets the dimensions of the pane and updates viewport
func (m *Model) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Calculate viewport dimensions (subtract borders, padding, and title/help space)
	// Border + padding = 4 each side
	// Title and help = ~6 lines
	viewportWidth := width - 8
	viewportHeight := height - 10

	// Ensure minimum dimensions
	if viewportWidth < 20 {
		viewportWidth = 20
	}
	if viewportHeight < 5 {
		viewportHeight = 5
	}

	m.viewport.Width = viewportWidth
	m.viewport.Height = viewportHeight
	m.ready = true
}

// formatHealthRow formats a health summary row with consistent alignment
func formatHealthRow(label string, value int, style lipgloss.Style) string {
	// Fixed width for labels (24 chars) to ensure alignment
	labelWidth := 24
	paddedLabel := util.PadRight(label+":", labelWidth)
	valueStr := fmt.Sprintf("%d", value)
	styledValue := style.Render(valueStr)
	return fmt.Sprintf("  %s %s\n", paddedLabel, styledValue)
}

// formatIssueRow formats an issue row with icon, label, and count
func formatIssueRow(label string, count int, icon string, style lipgloss.Style) string {
	// Fixed width for labels (26 chars) to ensure alignment
	labelWidth := 26
	paddedLabel := util.PadRight(label+":", labelWidth)
	return fmt.Sprintf("  %s %s %d pod(s)\n", style.Render(icon), paddedLabel, count)
}
