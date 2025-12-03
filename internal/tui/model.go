package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	corev1 "k8s.io/api/core/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/analysis"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
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
	LayoutThreePane LayoutMode = iota // Wide terminals: all 3 panes
	LayoutTwoPane                     // Medium terminals: 2 relevant panes
	LayoutSinglePane                  // Small terminals: 1 active pane
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
	state       *state.AppState
	activePane  ActivePane
	layoutMode  LayoutMode
	namespaces  namespace.Model
	pods        pods.Model
	details     details.Model
	width       int
	height      int
	showHelp    bool
}

// NewModel creates a new TUI application model with Kubernetes client
func NewModel(kubeClient state.KubeClient) Model {
	appState := state.NewAppState(kubeClient)

	return Model{
		state:      appState,
		activePane: PaneNamespace,
		namespaces: namespace.NewModel(appState),
		pods:       pods.NewModel(appState),
		details:    details.NewModel(appState),
		showHelp:   false,
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

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			// Quit application
			return m, tea.Quit

		case "?":
			// Toggle help
			m.showHelp = !m.showHelp
			return m, nil

		case "tab":
			// Switch to next pane (UI_DESIGN.md: Tab switches panes)
			m.activePane = (m.activePane + 1) % 3
			m.updateActivePanes()
			return m, nil

		case "shift+tab":
			// Switch to previous pane
			m.activePane = (m.activePane - 1 + 3) % 3
			m.updateActivePanes()
			return m, nil
		}

		// Forward key events to active pane
		switch m.activePane {
		case PaneNamespace:
			m.namespaces, cmd = m.namespaces.Update(msg)
			cmds = append(cmds, cmd)

			// Check if namespace was selected and pods need to be loaded
			if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] PaneNamespace: SelectedNS=%s, LoadingPods=%v\n", m.state.SelectedNamespace, m.state.LoadingPods)
			}
			if m.state.SelectedNamespace != "" && !m.state.LoadingPods && len(m.state.Pods) == 0 {
				if debugMode {
					fmt.Fprintf(os.Stderr, "[DEBUG] Triggering loadPods for namespace: %s\n", m.state.SelectedNamespace)
				}
				// Trigger pod loading and set flag to prevent retriggering
				m.state.LoadingPods = true
				cmd = loadPods(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			} else if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] NOT triggering loadPods (condition failed)\n")
			}

			// Check if namespace health was requested
			if m.state.LoadingNamespaceHealth && m.state.SelectedNamespace != "" {
				cmd = loadNamespaceHealth(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			}

		case PanePods:
			m.pods, cmd = m.pods.Update(msg)
			cmds = append(cmds, cmd)

			// Check if pod was selected for details or logs
			if m.state.SelectedPod != "" {
				// Check if we need to load pod details (ViewDetails or ViewDiagnostics)
				needsDetails := (m.state.CurrentView == state.ViewDetails || m.state.CurrentView == state.ViewDiagnostics) && m.state.LoadingPodDetails
				// Trigger load if LoadingPodDetails is true (handles both new loads and pod switches)
				if needsDetails {
					cmd = loadPodDetails(m.state.KubeClient, m.state.SelectedNamespace, m.state.SelectedPod)
					cmds = append(cmds, cmd)
				}

				// Check if we need to load logs
				needsLogs := m.state.CurrentView == state.ViewLogs && m.state.LoadingLogs
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
		m.state.LoadingNamespaces = false
		if msg.Err != nil {
			m.state.NamespacesError = msg.Err.Error()
		} else {
			m.state.Namespaces = msg.Namespaces
			m.state.NamespacesError = ""
		}

	case PodsLoadedMsg:
		if debugMode {
			fmt.Fprintf(os.Stderr, "[DEBUG] PodsLoadedMsg: ns=%s, pods=%d, err=%v\n", msg.Namespace, len(msg.Pods), msg.Err)
		}
		m.state.LoadingPods = false

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
				m.state.LoadingPods = true
				cmd = loadPods(m.state.KubeClient, m.state.SelectedNamespace)
				cmds = append(cmds, cmd)
			}
			return m, tea.Batch(cmds...)
		}

		if msg.Err != nil {
			m.state.PodsError = msg.Err.Error()
		} else {
			m.state.Pods = msg.Pods
			m.state.PodsError = ""
			if debugMode {
				fmt.Fprintf(os.Stderr, "[DEBUG] State updated: state.Pods now has %d pods\n", len(m.state.Pods))
			}
		}

	case PodDetailsLoadedMsg:
		m.state.LoadingPodDetails = false
		if msg.Err != nil {
			m.state.PodDetailsError = msg.Err.Error()
		} else {
			m.state.CurrentPod = msg.Pod
			m.state.CurrentEvents = msg.Events
			m.state.PodDetailsError = ""

			// Run diagnostics engine on the pod
			engine := analysis.NewEngine()
			diagnostics := engine.AnalyzePod(analysis.PodContext{
				Pod:    msg.Pod,
				Events: msg.Events,
			})

			// Store diagnostics results (convert to []interface{} to avoid import cycle)
			m.state.Diagnostics = make([]interface{}, len(diagnostics))
			for i, diag := range diagnostics {
				m.state.Diagnostics[i] = diag
			}
		}

	case LogsLoadedMsg:
		m.state.LoadingLogs = false
		if msg.Err != nil {
			m.state.LogsError = msg.Err.Error()
		} else {
			m.state.CurrentLogs = msg.Content
			m.state.CurrentContainers = msg.Containers
			m.state.LogsError = ""
			// Default to first container
			if len(m.state.CurrentContainers) > 0 {
				m.state.SelectedContainer = 0
			}
		}

	case NamespaceHealthLoadedMsg:
		m.state.LoadingNamespaceHealth = false
		if msg.Err != nil {
			m.state.NamespaceHealthError = msg.Err.Error()
		} else {
			m.state.CurrentNamespaceHealth = msg.Health
			m.state.NamespaceHealthError = ""
		}
	}

	return m, tea.Batch(cmds...)
}

// View renders the entire TUI with responsive layout
func (m Model) View() string {
	if m.showHelp {
		return m.renderHelp()
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
	if m.width < 60 {
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
	if m.width < 80 {
		// Narrow: minimal hints
		right = " Tab | ? | q "
	} else {
		// Normal: full hints
		right = " Tab switch | ? help | q quit "
	}

	// Calculate spacing
	leftLen := lipgloss.Width(left)
	rightLen := lipgloss.Width(right)
	padding := m.width - leftLen - rightLen
	if padding < 0 {
		padding = 0
	}

	statusLine := left + strings.Repeat(" ", padding) + right

	return theme.HelpStyle.Render(statusLine)
}

// renderHelp shows the help screen with all keybindings
func (m Model) renderHelp() string {
	help := `
kubectl-medic - Help

GLOBAL KEYBINDINGS:
  q          Quit application
  ?          Toggle help
  Tab        Switch to next pane
  Shift+Tab  Switch to previous pane

NAMESPACE PANE:
  ↑/↓ or k/j Navigate list
  Enter      Select namespace and load pods
  /          Filter namespaces (type to filter, Enter to apply, Esc to cancel)
  s          Cycle sort mode (Name A→Z, Name Z→A, Status)
  h          Show namespace health summary

PODS PANE:
  ↑/↓ or k/j Navigate list
  Enter or d Show pod details
  x          Run diagnostics on selected pod
  l          View logs for selected pod
  s          Cycle sort mode (Name, Status, Restarts, Age)
  /          Filter pods (type to filter, Enter to apply, Esc to cancel)

DETAILS PANE:
  ↑/↓ or k/j Scroll content
  c          Switch container (in logs view)
  f          Toggle follow mode (in logs view)
  Esc        Return to details view

Press ? to close this help screen
`

	style := lipgloss.NewStyle().
		Width(m.width).
		Height(m.height).
		Align(lipgloss.Center, lipgloss.Center).
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(theme.ColorBorderActive).
		Padding(2, 4)

	return style.Render(help)
}

// determineLayoutMode determines which layout mode to use based on terminal width
// Conservative thresholds to prevent wrapping and ensure readability
func (m Model) determineLayoutMode() LayoutMode {
	// Adjusted thresholds based on actual table widths
	// Three panes need ~130+ cols to display without wrapping
	// Two panes need ~85+ cols
	// Single pane works at any size but best at 60+
	if m.width >= 130 {
		return LayoutThreePane
	} else if m.width >= 85 {
		return LayoutTwoPane
	}
	return LayoutSinglePane
}

// updatePaneSizes recalculates and sets sizes for all panes based on layout mode
func (m *Model) updatePaneSizes() {
	paneHeight := m.height - 2 // Reserve 2 lines for status bar

	switch m.layoutMode {
	case LayoutThreePane:
		// Wide terminals: 28% | 36% | 36% (minimum 20 chars per pane)
		pane1Width := max(20, m.width*28/100)
		pane2Width := max(20, m.width*36/100)
		pane3Width := m.width - pane1Width - pane2Width
		if pane3Width < 20 {
			pane3Width = 20
		}

		m.namespaces.SetSize(pane1Width, paneHeight)
		m.pods.SetSize(pane2Width, paneHeight)
		m.details.SetSize(pane3Width, paneHeight)

	case LayoutTwoPane:
		// Medium terminals: two panes at 45% and 55%
		pane1Width := max(20, m.width*45/100)
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
		fullWidth := max(40, m.width)
		m.namespaces.SetSize(fullWidth, paneHeight)
		m.pods.SetSize(fullWidth, paneHeight)
		m.details.SetSize(fullWidth, paneHeight)
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
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
