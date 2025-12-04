package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/analysis"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/constants"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/details"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/namespace"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/pods"
)

// ActivePane represents which pane is currently focused
type ActivePane int

const (
	PaneNamespace ActivePane = iota
	PanePods
	PaneDetails
)

// debugMode enables debug logging to stderr
const debugMode = false

// LayoutMode represents how panes are arranged based on terminal size
type LayoutMode int

const (
	LayoutThreePane  LayoutMode = iota // Wide terminals: all 3 panes
	LayoutTwoPane                      // Medium terminals: 2 relevant panes
	LayoutSinglePane                   // Small terminals: 1 active pane
)

// Message types for async Kubernetes operations

// NamespacesLoadedMsg is sent when namespaces are loaded from the cluster
type NamespacesLoadedMsg struct {
	Namespaces []state.NamespaceInfo
	Err        error
}

// PodsLoadedMsg is sent when pods are loaded for a namespace
type PodsLoadedMsg struct {
	Namespace string
	Pods      []state.PodInfo
	Err       error
}

// PodDetailsLoadedMsg is sent when pod details and events are loaded
type PodDetailsLoadedMsg struct {
	Pod    *corev1.Pod
	Events []state.EventInfo
	Err    error
}

// LogsLoadedMsg is sent when logs are loaded for a pod
type LogsLoadedMsg struct {
	Pod        string
	Container  string
	Content    string
	Containers []string
	Err        error
}

// NamespaceHealthLoadedMsg is sent when namespace health summary is loaded
type NamespaceHealthLoadedMsg struct {
	Namespace string
	Health    *state.NamespaceHealth
	Err       error
}

// Model is the main application model containing all three panes
type Model struct {
	state        *state.AppState
	activePane   ActivePane
	layoutMode   LayoutMode
	namespaces   namespace.Model
	pods         pods.Model
	details      details.Model
	width        int
	height       int
	showHelp     bool
	helpViewport *viewport.Model // Scrollable viewport for help screen
}

// NewModel creates a new TUI application model with Kubernetes client
func NewModel(kubeClient state.KubeClient) Model {
	appState := state.NewAppState(kubeClient)

	// Initialize help viewport
	helpVp := viewport.New(80, 30)
	helpVp.HighPerformanceRendering = false

	return Model{
		state:        appState,
		activePane:   PaneNamespace,
		namespaces:   namespace.NewModel(appState),
		pods:         pods.NewModel(appState),
		details:      details.NewModel(appState),
		showHelp:     false,
		helpViewport: &helpVp,
	}
}

// Init initializes the TUI application and triggers initial namespace load
func (m Model) Init() tea.Cmd {
	// Load namespaces immediately on startup
	return loadNamespaces(m.state.KubeClient)
}

// Update handles messages and updates the model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.layoutMode = m.determineLayoutMode()
		m.updatePaneSizes()
		m.updateActivePanes()

		// Update help viewport dimensions
		if m.helpViewport != nil {
			helpHeight := m.height - 6 // border + padding + margins
			helpWidth := m.width - 10  // border + padding
			if helpHeight < 10 {
				helpHeight = 10
			}
			if helpWidth < 30 {
				helpWidth = 30
			}
			m.helpViewport.Width = helpWidth
			m.helpViewport.Height = helpHeight
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Quit application
			return m, tea.Quit

		case "?":
			// Toggle help
			m.showHelp = !m.showHelp
			if m.showHelp && m.helpViewport != nil {
				// Reset help viewport scroll position when opening
				m.helpViewport.GotoTop()
			}
			return m, nil

		case "tab":
			// Switch to next pane (UI_DESIGN.md: Tab switches panes)
			// But don't switch panes if help is shown
			if !m.showHelp {
				m.activePane = (m.activePane + 1) % 3
				m.updateActivePanes()
			}
			return m, nil

		case "shift+tab":
			// Switch to previous pane
			// But don't switch panes if help is shown
			if !m.showHelp {
				m.activePane = (m.activePane - 1 + 3) % 3
				m.updateActivePanes()
			}
			return m, nil

		// Help viewport scrolling (when help is open)
		case "up", "k":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.LineUp(1)
				return m, nil
			}

		case "down", "j":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.LineDown(1)
				return m, nil
			}

		case "pageup":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.PageUp()
				return m, nil
			}

		case "pagedown":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.PageDown()
				return m, nil
			}

		case "home":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.GotoTop()
				return m, nil
			}

		case "end":
			if m.showHelp && m.helpViewport != nil {
				m.helpViewport.GotoBottom()
				return m, nil
			}
		}

		// Forward key events to active pane
		switch m.activePane {
		case PaneNamespace:
			m.namespaces, cmd = m.namespaces.Update(msg)
			cmds = append(cmds, cmd)

			// Smart caching: Check if namespace was selected and pods need to be loaded
			if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] PaneNamespace: SelectedNS=%s, LoadingPods=%v, Loaded=%v, CachedNS=%s\n",
					m.state.SelectedNamespace, m.state.Pods.Loading, m.state.Pods.Loaded, m.state.LastPodNamespace)
			}
			// Load pods if: namespace selected AND (not already loading) AND (not cached for this namespace OR cache empty)
			needsPodsLoad := m.state.SelectedNamespace != "" &&
				!m.state.Pods.Loading &&
				(!m.state.Pods.Loaded || m.state.LastPodNamespace != m.state.SelectedNamespace)

			if needsPodsLoad {
				if debugMode {
					fmt.Fprintf(os.Stderr, "[DEBUG] Triggering loadPods for namespace: %s\n", m.state.SelectedNamespace)
				}
				m.state.Pods.Loading = true
				m.state.LoadingPods = true // DEPRECATED
				cmd = loadPods(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			} else if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] NOT triggering loadPods (using cache)\n")
			}

			// Check if namespace health was requested
			if m.state.Health.Loading && m.state.SelectedNamespace != "" {
				cmd = loadNamespaceHealth(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			}

		case PanePods:
			m.pods, cmd = m.pods.Update(msg)
			cmds = append(cmds, cmd)

			// Smart caching: Check if pod was selected for details or logs
			if m.state.SelectedPod != "" {
				// Check if we need to load pod details (ViewDetails or ViewDiagnostics)
				needsDetails := (m.state.CurrentView == state.ViewDetails || m.state.CurrentView == state.ViewDiagnostics) &&
					m.state.PodDetails.Loading
				// Trigger load if Loading is true (handles both new loads and pod switches)
				if needsDetails {
					cmd = loadPodDetails(m.state.KubeClient, m.state.SelectedNamespace, m.state.SelectedPod)
					cmds = append(cmds, cmd)
				}

				// Check if we need to load logs
				needsLogs := m.state.CurrentView == state.ViewLogs && m.state.Logs.Loading
				if needsLogs {
					cmd = loadLogs(m.state.KubeClient, m.state.SelectedNamespace, m.state.SelectedPod)
					cmds = append(cmds, cmd)
				}
			}

		case PaneDetails:
			m.details, cmd = m.details.Update(msg)
			cmds = append(cmds, cmd)
		}

	// Handle async Kubernetes responses
	case NamespacesLoadedMsg:
		m.state.Namespaces.Loading = false
		m.state.LoadingNamespaces = false // DEPRECATED: backward compat
		if msg.Err != nil {
			m.state.Namespaces.Error = msg.Err
			m.state.NamespacesError = msg.Err.Error() // DEPRECATED
		} else {
			m.state.Namespaces.Data = msg.Namespaces
			m.state.Namespaces.Loaded = true
			m.state.NamespacesError = "" // DEPRECATED
		}

	case PodsLoadedMsg:
		if debugMode {
			fmt.Fprintf(os.Stderr, "[DEBUG] PodsLoadedMsg: ns=%s, pods=%d, err=%v\n", msg.Namespace, len(msg.Pods), msg.Err)
		}
		m.state.Pods.Loading = false
		m.state.LoadingPods = false // DEPRECATED: backward compat

		// Check if this message is for the currently selected namespace
		if msg.Namespace != m.state.SelectedNamespace {
			if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] Ignoring stale PodsLoadedMsg: msg.ns=%s != selected.ns=%s\n", msg.Namespace, m.state.SelectedNamespace)
			}
			// Namespace changed while loading - trigger a fresh load
			if m.state.SelectedNamespace != "" {
				if debugMode {
					fmt.Fprintf(os.Stderr, "[DEBUG] Triggering fresh load for current namespace: %s\n", m.state.SelectedNamespace)
				}
				m.state.Pods.Loading = true
				m.state.LoadingPods = true // DEPRECATED
				cmd = loadPods(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if msg.Err != nil {
			m.state.Pods.Error = msg.Err
			m.state.PodsError = msg.Err.Error() // DEPRECATED
		} else {
			m.state.Pods.Data = msg.Pods
			m.state.Pods.Loaded = true
			m.state.LastPodNamespace = msg.Namespace // Cache key
			m.state.PodsError = ""                   // DEPRECATED
			if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] State updated: state.Pods now has %d pods\n", len(msg.Pods))
			}
		}

	case PodDetailsLoadedMsg:
		m.state.PodDetails.Loading = false
		m.state.LoadingPodDetails = false // DEPRECATED: backward compat
		if msg.Err != nil {
			m.state.PodDetails.Error = msg.Err
			m.state.PodDetailsError = msg.Err.Error() // DEPRECATED
		} else {
			m.state.PodDetails.Data = state.PodDetailsData{
				Pod:    msg.Pod,
				Events: msg.Events,
			}
			m.state.PodDetails.Loaded = true
			if msg.Pod != nil {
				m.state.LastPodUID = string(msg.Pod.UID) // Cache key
			}
			m.state.PodDetailsError = "" // DEPRECATED

			// DEPRECATED: Maintain backward compatibility
			m.state.CurrentPod = msg.Pod
			m.state.CurrentEvents = msg.Events

			// Run diagnostics engine on the pod
			engine := analysis.NewEngine()
			diagnostics := engine.AnalyzePod(analysis.PodContext{
				Pod:    msg.Pod,
				Events: msg.Events,
			})

			// Store diagnostics results (now strongly typed via internal/types)
			m.state.Diagnostics = diagnostics
		}

	case LogsLoadedMsg:
		m.state.Logs.Loading = false
		m.state.LoadingLogs = false // DEPRECATED: backward compat
		if msg.Err != nil {
			m.state.Logs.Error = msg.Err
			m.state.LogsError = msg.Err.Error() // DEPRECATED
		} else {
			containerIdx := 0
			if len(msg.Containers) > 0 {
				containerIdx = 0 // Default to first container
			}
			m.state.Logs.Data = state.LogsData{
				Content:        msg.Content,
				Containers:     msg.Containers,
				ContainerIndex: containerIdx,
			}
			m.state.Logs.Loaded = true
			m.state.LastLogKey = msg.Pod + ":" + msg.Container // Cache key
			m.state.LogsError = ""                             // DEPRECATED

			// DEPRECATED: Maintain backward compatibility
			m.state.CurrentLogs = msg.Content
			m.state.CurrentContainers = msg.Containers
			m.state.SelectedContainer = containerIdx
		}

	case NamespaceHealthLoadedMsg:
		m.state.Health.Loading = false
		m.state.LoadingNamespaceHealth = false // DEPRECATED: backward compat
		if msg.Err != nil {
			m.state.Health.Error = msg.Err
			m.state.NamespaceHealthError = msg.Err.Error() // DEPRECATED
		} else {
			m.state.Health.Data = msg.Health
			m.state.Health.Loaded = true
			m.state.NamespaceHealthError = "" // DEPRECATED

			// DEPRECATED: Maintain backward compatibility
			m.state.CurrentNamespaceHealth = msg.Health
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the entire TUI with responsive layout
func (m Model) View() string {
	if m.showHelp {
		// Render help with border and padding
		helpContent := m.renderHelp()

		// Apply pane styling to help
		style := lipgloss.NewStyle().
			Width(m.width-2).   // Subtract border width
			Height(m.height-2). // Account for border height
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(theme.ColorBorderActive).
			Padding(1, 2) // Vertical 1, horizontal 2

		return style.Render(helpContent)
	}

	if m.width == 0 || m.height == 0 {
		return theme.LoadingStyle.Render("⏳ Loading...")
	}

	// Render panes based on layout mode
	var mainView string

	switch m.layoutMode {
	case LayoutThreePane:
		// Wide terminals: show all 3 panes
		namespaceView := m.namespaces.View()
		podsView := m.pods.View()
		detailsView := m.details.View()

		mainView = lipgloss.JoinHorizontal(
			lipgloss.Top,
			namespaceView,
			podsView,
			detailsView,
		)

	case LayoutTwoPane:
		// Medium terminals: show 2 relevant panes
		if m.activePane == PaneNamespace {
			// Show Namespaces + Pods
			namespaceView := m.namespaces.View()
			podsView := m.pods.View()

			mainView = lipgloss.JoinHorizontal(
				lipgloss.Top,
				namespaceView,
				podsView,
			)
		} else {
			// Show Pods + Details (most common workflow)
			podsView := m.pods.View()
			detailsView := m.details.View()

			mainView = lipgloss.JoinHorizontal(
				lipgloss.Top,
				podsView,
				detailsView,
			)
		}

	case LayoutSinglePane:
		// Small terminals: show only active pane
		switch m.activePane {
		case PaneNamespace:
			mainView = m.namespaces.View()
		case PanePods:
			mainView = m.pods.View()
		case PaneDetails:
			mainView = m.details.View()
		}
	}

	// Add status bar at bottom
	statusBar := m.renderStatusBar()

	// Combine main view and status bar vertically
	return lipgloss.JoinVertical(
		lipgloss.Left,
		mainView,
		statusBar,
	)
}

// renderStatusBar shows current state and key hints at the bottom
func (m Model) renderStatusBar() string {
	// Build pane tabs
	nsStyle := theme.StatusBarInactiveStyle
	podsStyle := theme.StatusBarInactiveStyle
	detailsStyle := theme.StatusBarInactiveStyle

	switch m.activePane {
	case PaneNamespace:
		nsStyle = theme.StatusBarActiveStyle
	case PanePods:
		podsStyle = theme.StatusBarActiveStyle
	case PaneDetails:
		// Determine what kind of details view we're showing
		detailsStyle = theme.StatusBarActiveStyle
	}

	// Build left side with tabs
	var left string
	if m.width < constants.StatusBarWidthNarrow {
		// Very narrow: just show active pane
		var activeName string
		switch m.activePane {
		case PaneNamespace:
			activeName = "NS"
		case PanePods:
			activeName = "Pods"
		case PaneDetails:
			activeName = "Details"
		}
		left = fmt.Sprintf(" %s ", theme.StatusBarActiveStyle.Render(activeName))
	} else {
		// Normal: show all tabs
		left = fmt.Sprintf(" %s | %s | %s ",
			nsStyle.Render("NS"),
			podsStyle.Render("Pods"),
			detailsStyle.Render("Details"))
	}

	// Build right side with hints
	var right string
	if m.width < constants.StatusBarWidthMinimalHints {
		// Narrow: minimal hints
		right = "Tab | ? | q"
	} else {
		// Normal: full hints
		right = "Tab switch | ? help | q quit"
	}

	// Calculate spacing with proper padding
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	totalNeeded := leftLen + rightLen + 4 // 2 spaces padding each side
	padding := m.width - totalNeeded
	if padding < 1 {
		padding = 1
	}

	statusLine := " " + left + strings.Repeat(" ", padding) + right + " "

	return theme.HelpStyle.Render(statusLine)
}

// renderHelp shows the help screen with all keybindings.
// Uses scrollable viewport for long content with visual hierarchy and proper spacing.
func (m Model) renderHelp() string {
	contentWidth := m.width - 10 // border(2) + padding(4+4)
	if contentWidth < 40 {
		contentWidth = 40
	}

	// Build help content with visual hierarchy
	var b strings.Builder

	// Title - centered, prominent
	title := "kubectl-medic - Help"
	titleLine := lipgloss.NewStyle().Width(contentWidth).Align(lipgloss.Center).Render(title)
	b.WriteString(theme.TitleStyle.Render(titleLine) + "\n\n")

	// Global keybindings section
	b.WriteString(theme.SectionHeaderStyle.Render("Global Keybindings") + "\n")
	b.WriteString("  q                 Quit application\n")
	b.WriteString("  ?                 Toggle this help screen\n")
	b.WriteString("  Tab, Shift+Tab    Switch between panes\n\n")

	// Namespace pane section
	b.WriteString(theme.SectionHeaderStyle.Render("Namespace Pane") + "\n")
	b.WriteString("  Up, Down, k, j    Navigate namespaces\n")
	b.WriteString("  Enter             Select namespace and load pods\n")
	b.WriteString("  /                 Filter namespaces (type to filter)\n")
	b.WriteString("  s                 Cycle sort mode\n")
	b.WriteString("  h                 Show namespace health summary\n\n")

	// Pods pane section
	b.WriteString(theme.SectionHeaderStyle.Render("Pods Pane") + "\n")
	b.WriteString("  Up, Down, k, j    Navigate pods\n")
	b.WriteString("  Enter or d        Show pod details\n")
	b.WriteString("  x                 Run diagnostics on selected pod\n")
	b.WriteString("  l                 View logs for selected pod\n")
	b.WriteString("  s                 Cycle sort mode\n")
	b.WriteString("  /                 Filter pods (type to filter)\n\n")

	// Details pane section
	b.WriteString(theme.SectionHeaderStyle.Render("Details Pane") + "\n")
	b.WriteString("  Up, Down, k, j    Scroll content\n")
	b.WriteString("  c                 Switch container (in logs view)\n")
	b.WriteString("  d                 Return to pod details\n")
	b.WriteString("  x                 View diagnostics\n")
	b.WriteString("  l                 View logs\n\n")

	// Help navigation section
	b.WriteString(theme.SectionHeaderStyle.Render("Help Navigation") + "\n")
	b.WriteString("  Up, Down, k, j    Scroll this help\n")
	b.WriteString("  Page Up, Page Down Page scroll\n")
	b.WriteString("  Home, End         Jump to top or bottom\n")
	b.WriteString("  ?                 Close this help screen\n")

	helpContent := b.String()

	// Set viewport content
	if m.helpViewport != nil {
		m.helpViewport.SetContent(helpContent)
		return m.helpViewport.View()
	}

	// Fallback if viewport not initialized
	return helpContent
}

// determineLayoutMode determines which layout mode to use based on terminal width
// Conservative thresholds to prevent wrapping and ensure readability
func (m Model) determineLayoutMode() LayoutMode {
	if m.width >= constants.LayoutWidthThreePane {
		return LayoutThreePane
	} else if m.width >= constants.LayoutWidthTwoPane {
		return LayoutTwoPane
	}
	return LayoutSinglePane
}

// updatePaneSizes recalculates and sets sizes for all panes based on layout mode
func (m *Model) updatePaneSizes() {
	paneHeight := m.height - constants.StatusBarHeight

	switch m.layoutMode {
	case LayoutThreePane:
		// Wide terminals: use percentage-based layout with minimums
		pane1Width := max(constants.PaneWidthMinimum, m.width*constants.PanePercentLeft/100)
		pane2Width := max(constants.PaneWidthMinimum, m.width*constants.PanePercentMiddle/100)
		pane3Width := m.width - pane1Width - pane2Width
		if pane3Width < constants.PaneWidthMinimum {
			pane3Width = constants.PaneWidthMinimum
		}

		m.namespaces.SetSize(pane1Width, paneHeight)
		m.pods.SetSize(pane2Width, paneHeight)
		m.details.SetSize(pane3Width, paneHeight)

	case LayoutTwoPane:
		// Medium terminals: two panes with percentage-based layout
		pane1Width := max(constants.PaneWidthMinimum, m.width*constants.PanePercentTwoPaneLeft/100)
		pane2Width := m.width - pane1Width

		// Which two panes to show depends on active pane
		if m.activePane == PaneNamespace {
			// Namespaces + Pods
			m.namespaces.SetSize(pane1Width, paneHeight)
			m.pods.SetSize(pane2Width, paneHeight)
			m.details.SetSize(pane2Width, paneHeight) // Same size for consistency
		} else {
			// Pods + Details (most common workflow)
			m.pods.SetSize(pane1Width, paneHeight)
			m.details.SetSize(pane2Width, paneHeight)
			m.namespaces.SetSize(pane1Width, paneHeight) // Same size for consistency
		}

	case LayoutSinglePane:
		// Small terminals: one pane gets full width
		fullWidth := max(constants.LayoutWidthMinimum, m.width)
		m.namespaces.SetSize(fullWidth, paneHeight)
		m.pods.SetSize(fullWidth, paneHeight)
		m.details.SetSize(fullWidth, paneHeight)
	}
}

// updateActivePanes updates which pane is marked as active
func (m *Model) updateActivePanes() {
	m.namespaces.SetActive(m.activePane == PaneNamespace)
	m.pods.SetActive(m.activePane == PanePods)
	m.details.SetActive(m.activePane == PaneDetails)
}

// Async command functions - these run in background and send messages when done

// loadNamespaces fetches namespaces from Kubernetes
func loadNamespaces(client state.KubeClient) tea.Cmd {
	return func() tea.Msg {
		namespaces, err := client.ListNamespaces()
		return NamespacesLoadedMsg{
			Namespaces: namespaces,
			Err:        err,
		}
	}
}

// loadPods fetches pods for a specific namespace
func loadPods(client state.KubeClient, namespace string) tea.Cmd {
	return func() tea.Msg {
		pods, err := client.ListPods(namespace)
		return PodsLoadedMsg{
			Namespace: namespace,
			Pods:      pods,
			Err:       err,
		}
	}
}

// loadPodDetails fetches detailed pod information and events
func loadPodDetails(client state.KubeClient, namespace, podName string) tea.Cmd {
	return func() tea.Msg {
		pod, err := client.GetPod(namespace, podName)
		if err != nil {
			return PodDetailsLoadedMsg{Err: err}
		}

		events, err := client.GetPodEvents(namespace, podName)
		if err != nil {
			// Don't fail completely if events fail
			events = []state.EventInfo{}
		}

		return PodDetailsLoadedMsg{
			Pod:    pod,
			Events: events,
			Err:    nil,
		}
	}
}

// loadLogs fetches logs for a pod
func loadLogs(client state.KubeClient, namespace, podName string) tea.Cmd {
	return func() tea.Msg {
		// Get container list first
		containers, err := client.GetPodContainers(namespace, podName)
		if err != nil {
			return LogsLoadedMsg{Err: err}
		}

		// Load logs from first container by default
		var logs string
		if len(containers) > 0 {
			logs, err = client.GetPodLogs(namespace, podName, containers[0], 100)
			if err != nil {
				return LogsLoadedMsg{Err: err}
			}
		}

		return LogsLoadedMsg{
			Pod:        podName,
			Container:  containers[0],
			Content:    logs,
			Containers: containers,
			Err:        nil,
		}
	}
}

// loadNamespaceHealth fetches all pods in a namespace and computes health summary
func loadNamespaceHealth(client state.KubeClient, namespace string) tea.Cmd {
	return func() tea.Msg {
		// Get all pods in the namespace
		podInfos, err := client.ListPods(namespace)
		if err != nil {
			return NamespaceHealthLoadedMsg{
				Namespace: namespace,
				Err:       err,
			}
		}

		// Fetch full pod objects and events for each pod
		pods := make([]*corev1.Pod, 0, len(podInfos))
		eventsByPod := make(map[string][]state.EventInfo)

		for _, podInfo := range podInfos {
			// Get full pod object
			pod, err := client.GetPod(namespace, podInfo.Name)
			if err != nil {
				// Skip pods we can't fetch
				continue
			}
			pods = append(pods, pod)

			// Get events for this pod
			events, err := client.GetPodEvents(namespace, podInfo.Name)
			if err != nil {
				// If events fail, just use empty list
				events = []state.EventInfo{}
			}
			eventsByPod[podInfo.Name] = events
		}

		// Run analysis to compute health summary
		health := analysis.SummarizeNamespace(namespace, pods, eventsByPod)

		return NamespaceHealthLoadedMsg{
			Namespace: namespace,
			Health:    &health,
			Err:       nil,
		}
	}
}
