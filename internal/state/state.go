package state

import (
	corev1 "k8s.io/api/core/v1"
)

// KubeClient interface allows for testing and decoupling
// The real implementation is in internal/kube/client.go
type KubeClient interface {
	ListNamespaces() ([]NamespaceInfo, error)
	ListPods(namespace string) ([]PodInfo, error)
	GetPod(namespace, name string) (*corev1.Pod, error)
	GetPodEvents(namespace, podName string) ([]EventInfo, error)
	GetPodLogs(namespace, podName, containerName string, tailLines int64) (string, error)
	GetPodContainers(namespace, podName string) ([]string, error)
	HealthCheck() error
}

// AppState holds the shared state across all TUI components
// This allows panes to communicate and share context
type AppState struct {
	// Kubernetes client
	KubeClient KubeClient

	// Current selections
	SelectedNamespace string
	SelectedPod       string

	// View state
	CurrentView ViewMode

	// Data cache (populated by kube client)
	Namespaces []NamespaceInfo
	Pods       []PodInfo

	// Pod details cache
	CurrentPod    *corev1.Pod
	CurrentEvents []EventInfo

	// Logs cache
	CurrentLogs       string
	CurrentContainers []string
	SelectedContainer int // index into CurrentContainers

	// Loading states
	LoadingNamespaces bool
	LoadingPods       bool
	LoadingPodDetails bool
	LoadingLogs       bool

	// Error states
	NamespacesError string
	PodsError       string
	PodDetailsError string
	LogsError       string

	// Diagnostics results (from analysis engine)
	// Type: []analysis.Diagnostic (stored as []interface{} to avoid import cycle)
	// Always assigned from analysis.Engine.AnalyzePod(), type-safe at assignment point
	Diagnostics []interface{}

	// Namespace health summary
	CurrentNamespaceHealth  *NamespaceHealth
	LoadingNamespaceHealth  bool
	NamespaceHealthError    string

	// Legacy diagnostics (deprecated, use Diagnostics instead)
	LastDiagnostic *DiagnosticResult
}

// ViewMode represents which detail view is currently active
type ViewMode int

const (
	ViewDetails ViewMode = iota
	ViewDiagnostics
	ViewLogs
	ViewNamespaceHealth
	ViewCopyCommands
)

// NamespaceInfo holds display information for a namespace
type NamespaceInfo struct {
	Name   string
	Status string
	Age    string
}

// PodInfo holds display information for a pod
type PodInfo struct {
	Name      string
	Namespace string
	Ready     string // e.g., "2/2"
	Status    string
	Restarts  int
	Age       string
}

// DiagnosticResult holds the output from the diagnostics engine
type DiagnosticResult struct {
	PodName     string
	Namespace   string
	Issues      []DiagnosticIssue
	Summary     string
	Suggestions []string
}

// DiagnosticIssue represents a single detected issue
type DiagnosticIssue struct {
	Severity    string // "error", "warning", "info"
	Category    string // "CrashLoopBackOff", "ImagePullBackOff", etc.
	Description string
	Reason      string
}

// EventInfo holds information about a Kubernetes event
type EventInfo struct {
	Type      string // "Normal", "Warning"
	Reason    string
	Message   string
	Count     int32
	FirstSeen string
	LastSeen  string
}

// NamespaceHealth holds health summary information for a namespace
type NamespaceHealth struct {
	Namespace         string
	TotalPods         int
	FailingPods       int
	CrashLoopPods     int
	ImagePullPods     int
	SchedulingPods    int
	ProbeIssuePods    int
	ResourceIssuePods int
	StorageIssuePods  int
	ConfigIssuePods   int
	InitFailurePods   int
}

// NewAppState creates a new application state with a Kubernetes client
func NewAppState(kubeClient KubeClient) *AppState {
	return &AppState{
		KubeClient:  kubeClient,
		CurrentView: ViewDetails,
		Namespaces:  []NamespaceInfo{},
		Pods:        []PodInfo{},
		// Start in loading state - namespaces will be loaded on Init
		LoadingNamespaces: true,
	}
}

// GetPodsInNamespace returns pods filtered by namespace
func (s *AppState) GetPodsInNamespace(namespace string) []PodInfo {
	if namespace == "" {
		return s.Pods
	}

	filtered := []PodInfo{}
	for _, pod := range s.Pods {
		if pod.Namespace == namespace {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

// GetSelectedPodInfo returns the currently selected pod's info
func (s *AppState) GetSelectedPodInfo() *PodInfo {
	for _, pod := range s.Pods {
		if pod.Name == s.SelectedPod {
			return &pod
		}
	}
	return nil
}

// ClearError clears all error states
func (s *AppState) ClearError() {
	s.NamespacesError = ""
	s.PodsError = ""
	s.PodDetailsError = ""
	s.LogsError = ""
}

// NamespaceExists checks if the given namespace name exists in the namespace list
func (s *AppState) NamespaceExists(name string) bool {
	for _, ns := range s.Namespaces {
		if ns.Name == name {
			return true
		}
	}
	return false
}

// PodExists checks if the given pod name exists in the pod list
func (s *AppState) PodExists(name string) bool {
	for _, pod := range s.Pods {
		if pod.Name == name {
			return true
		}
	}
	return false
}
