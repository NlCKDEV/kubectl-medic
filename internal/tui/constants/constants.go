package constants

// Layout width thresholds for responsive design
const (
	// LayoutWidthThreePane is the minimum width for showing all 3 panes (namespace, pods, details)
	// Below this threshold, the layout switches to 2-pane or single-pane mode
	LayoutWidthThreePane = 130

	// LayoutWidthTwoPane is the minimum width for showing 2 panes
	// Below this threshold, the layout switches to single-pane mode
	LayoutWidthTwoPane = 85

	// LayoutWidthMinimum is the absolute minimum usable width
	LayoutWidthMinimum = 40
)

// Pane width percentages for three-pane layout
const (
	// PanePercentLeft is the percentage of screen width for the left pane (namespaces)
	PanePercentLeft = 28

	// PanePercentMiddle is the percentage of screen width for the middle pane (pods)
	PanePercentMiddle = 36

	// PanePercentRight is the percentage of screen width for the right pane (details)
	// This is implicit (100 - Left - Middle = 36%)
	PanePercentRight = 36
)

// Pane width percentages for two-pane layout
const (
	// PanePercentTwoPaneLeft is the percentage of screen width for the left pane
	PanePercentTwoPaneLeft = 45

	// PanePercentTwoPaneRight is the percentage of screen width for the right pane
	// This is implicit (100 - Left = 55%)
	PanePercentTwoPaneRight = 55
)

// Pane sizing constraints
const (
	// PaneWidthMinimum is the minimum usable width for any pane
	PaneWidthMinimum = 20

	// StatusBarHeight is the number of lines reserved for the status bar at bottom
	StatusBarHeight = 2

	// PaneBorderPadding is the internal padding within a pane (borders + margins)
	// DEPRECATED: Use PaneHorizontalOverhead for accurate width calculations
	PaneBorderPadding = 4

	// PaneHorizontalOverhead is the total horizontal space consumed by pane borders and padding.
	// PaneStyle uses Border(RoundedBorder) = 2 chars + Padding(1, 2) = 4 chars = 6 total.
	// This MUST be used consistently in:
	//   - style.Width(m.width - PaneHorizontalOverhead)
	//   - content width calculations (column sizing, row building)
	PaneHorizontalOverhead = 6

	// PaneVerticalOverhead is the total vertical space consumed by pane borders and padding.
	// PaneStyle uses Border = 2 lines + Padding(1, 2) vertical = 2 lines = 4 total.
	PaneVerticalOverhead = 4

	// SelectionGutterWidth is the fixed width for the selection indicator column.
	// Either "▶ " (selected) or "  " (unselected) - always 2 characters.
	SelectionGutterWidth = 2
)

// Status bar responsive thresholds
const (
	// StatusBarWidthNarrow is the threshold below which status bar shows minimal layout
	StatusBarWidthNarrow = 60

	// StatusBarWidthMinimalHints is the threshold below which key hints are abbreviated
	StatusBarWidthMinimalHints = 80
)

// Details pane rendering constants
const (
	// DetailsMaxRecentEvents is the maximum number of events to show in pod details
	DetailsMaxRecentEvents = 5

	// DetailsHelpTextOffset is the horizontal offset for help text rendering
	// Deprecated: Use PaneHorizontalOverhead instead for consistent width calculations.
	// This constant is unused and will be removed in a future release.
	DetailsHelpTextOffset = 8
)
