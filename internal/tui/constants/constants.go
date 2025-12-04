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
	PaneBorderPadding = 4
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
	DetailsHelpTextOffset = 8
)
