package theme

import "github.com/charmbracelet/lipgloss"

// Theme colors and styles for kubectl-medic
// Follows UI_DESIGN.md specification for consistent styling

var (
	// Status colors
	ColorOK    = lipgloss.Color("#00FF00") // Green for healthy/running
	ColorWarn  = lipgloss.Color("#FFFF00") // Yellow for warnings
	ColorError = lipgloss.Color("#FF0000") // Red for errors/failures
	ColorInfo  = lipgloss.Color("#00FFFF") // Cyan for informational

	// UI element colors
	ColorBorder        = lipgloss.Color("#555555") // Softer gray for borders
	ColorBorderActive  = lipgloss.Color("#00AAAA") // Softer cyan for active pane
	ColorTitle         = lipgloss.Color("#FFFFFF") // White for titles
	ColorSubtle        = lipgloss.Color("#888888") // Gray for subtle text
	ColorHighlight     = lipgloss.Color("#FFAA00") // Softer yellow for highlights
	ColorSelected      = lipgloss.Color("#00CCCC") // Softer cyan for selected items
	ColorLoading       = lipgloss.Color("#888888") // Gray for loading states
	ColorEmpty         = lipgloss.Color("#666666") // Darker gray for empty states
	ColorSeparator     = lipgloss.Color("#444444") // Dark gray for separators

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Padding(0, 1)

	// Border styles - softer appearance
	BorderStyleNormal = lipgloss.RoundedBorder()
	BorderStyleActive = lipgloss.RoundedBorder()

	// Pane styles (3-pane layout)
	PaneStyle = lipgloss.NewStyle().
			Border(BorderStyleNormal).
			BorderForeground(ColorBorder).
			Padding(1, 2)

	PaneStyleActive = lipgloss.NewStyle().
			Border(BorderStyleActive).
			BorderForeground(ColorBorderActive).
			Padding(1, 2).
			Bold(true)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Foreground(ColorTitle).
			Bold(true).
			Padding(0, 1)

	// Status-specific styles
	StatusOKStyle = lipgloss.NewStyle().
			Foreground(ColorOK).
			Bold(true)

	StatusWarnStyle = lipgloss.NewStyle().
			Foreground(ColorWarn).
			Bold(true)

	StatusErrorStyle = lipgloss.NewStyle().
				Foreground(ColorError).
				Bold(true)

	StatusInfoStyle = lipgloss.NewStyle().
			Foreground(ColorInfo)

	// List item styles
	// NOTE: No padding on list items - padding is handled by pane container.
	// Adding padding here would cause width mismatches and row shifting.
	ListItemStyle = lipgloss.NewStyle()

	// ListItemSelectedStyle applies ONLY color and bold - NO padding.
	// Padding would add characters to selected rows, causing:
	// 1. Selected rows to be wider than unselected rows
	// 2. Content to shift horizontally when selection changes
	// 3. Rows to wrap if total width exceeds container
	ListItemSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorSelected).
				Bold(true)

	// Help text style
	HelpStyle = lipgloss.NewStyle().
			Foreground(ColorSubtle).
			Italic(true)

	// Key binding hint style
	KeyStyle = lipgloss.NewStyle().
			Foreground(ColorHighlight).
			Bold(true)

	// Loading and empty state styles
	LoadingStyle = lipgloss.NewStyle().
			Foreground(ColorLoading).
			Italic(true)

	EmptyStyle = lipgloss.NewStyle().
			Foreground(ColorEmpty).
			Italic(true)

	// Separator style for visual grouping
	SeparatorStyle = lipgloss.NewStyle().
			Foreground(ColorSeparator)

	// Table row styles - MUST NOT have padding to avoid width changes.
	// These are used for pre-sized table content where padding would cause wrapping.

	// TableRowStyle is for unselected table rows - no styling (plain text)
	TableRowStyle = lipgloss.NewStyle()

	// TableRowSelectedStyle is for selected table rows - color and bold ONLY.
	// NO padding, margin, or any property that changes visual width.
	TableRowSelectedStyle = lipgloss.NewStyle().
				Foreground(ColorSelected).
				Bold(true)

	// TableHeaderStyle is for table column headers - subtle color, no italic/padding.
	// Italic can cause rendering issues on narrow terminals.
	TableHeaderStyle = lipgloss.NewStyle().
				Foreground(ColorSubtle)

	// Active status bar item style
	StatusBarActiveStyle = lipgloss.NewStyle().
				Foreground(ColorSelected).
				Bold(true)

	StatusBarInactiveStyle = lipgloss.NewStyle().
				Foreground(ColorSubtle)
)

// StatusColor returns the appropriate color for a pod status string
func StatusColor(status string) lipgloss.Color {
	switch status {
	case "Running", "Succeeded", "Active":
		return ColorOK
	case "Pending", "Unknown":
		return ColorWarn
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff":
		return ColorError
	default:
		return ColorInfo
	}
}

// StatusStyle returns the appropriate style for a pod status string
func StatusStyle(status string) lipgloss.Style {
	switch status {
	case "Running", "Succeeded", "Active":
		return StatusOKStyle
	case "Pending", "Unknown":
		return StatusWarnStyle
	case "Failed", "CrashLoopBackOff", "Error", "ImagePullBackOff":
		return StatusErrorStyle
	default:
		return StatusInfoStyle
	}
}
