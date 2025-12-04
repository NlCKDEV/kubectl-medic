package state

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/types"
)

// Resource represents a cacheable resource with loading state
// This pattern unifies loading/error handling across all async data
type Resource[T any] struct {
	Data    T
	Loaded  bool    // True after first successful load
	Loading bool    // True during active fetch
	Error   error   // Last error, if any
}

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

// PodDetailsData groups pod details and related data
type PodDetailsData struct {
	Pod    *corev1.Pod
	Events []types.EventInfo
}

// LogsData groups logs and container information
type LogsData struct {
	Content        string
	Containers     []string
	ContainerIndex int
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

	// Cached resources (using Resource[T] pattern)
	Namespaces Resource[[]NamespaceInfo]
	Pods       Resource[[]PodInfo]
	PodDetails Resource[PodDetailsData]
	Logs       Resource[LogsData]
	Health     Resource[*types.NamespaceHealth]

	// Diagnostics results (from analysis engine)
	// Now strongly typed using internal/types to avoid import cycle
	Diagnostics []types.Diagnostic

	// Cache keys for smart reloading
	LastPodNamespace string // Track which namespace pods are cached for
	LastPodUID       string // Track which pod details are cached for
	LastLogKey       string // Track which pod/container logs are cached for

	// DEPRECATED: Legacy fields kept for backward compatibility during migration
	// TODO: Remove these after migration is complete
	CurrentPod           *corev1.Pod
	CurrentEvents        []types.EventInfo
	CurrentLogs          string
	CurrentContainers    []string
	SelectedContainer    int
	LoadingNamespaces    bool
	LoadingPods          bool
	LoadingPodDetails    bool
	LoadingLogs          bool
	LoadingNamespaceHealth bool
	NamespacesError      string
	PodsError            string
	PodDetailsError      string
	LogsError            string
	CurrentNamespaceHealth *types.NamespaceHealth
	NamespaceHealthError   string
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

// Type aliases for backward compatibility - these types now live in internal/types
type EventInfo = types.EventInfo
type NamespaceHealth = types.NamespaceHealth

// NewAppState creates a new application state with a Kubernetes client
func NewAppState(kubeClient KubeClient) *AppState {
	return &AppState{
		KubeClient:  kubeClient,
		CurrentView: ViewDetails,

		// Initialize Resource[T] fields
		Namespaces: Resource[[]NamespaceInfo]{
			Data:    []NamespaceInfo{},
			Loading: true, // Start loading on init
		},
		Pods: Resource[[]PodInfo]{
			Data: []PodInfo{},
		},
		PodDetails: Resource[PodDetailsData]{},
		Logs:       Resource[LogsData]{},
		Health:     Resource[*types.NamespaceHealth]{},

		// DEPRECATED: Maintain backward compatibility
		LoadingNamespaces: true,
	}
}

// GetPodsInNamespace returns pods filtered by namespace
func (s *AppState) GetPodsInNamespace(namespace string) []PodInfo {
	pods := s.Pods.Data
	if namespace == "" {
		return pods
	}

	filtered := []PodInfo{}
	for _, pod := range pods {
		if pod.Namespace == namespace {
			filtered = append(filtered, pod)
		}
	}
	return filtered
}

// GetSelectedPodInfo returns the currently selected pod's info
func (s *AppState) GetSelectedPodInfo() *PodInfo {
	for _, pod := range s.Pods.Data {
		if pod.Name == s.SelectedPod {
			return &pod
		}
	}
	return nil
}

// ClearError clears all error states
func (s *AppState) ClearError() {
	s.Namespaces.Error = nil
	s.Pods.Error = nil
	s.PodDetails.Error = nil
	s.Logs.Error = nil
	s.Health.Error = nil

	// DEPRECATED: Maintain backward compatibility
	s.NamespacesError = ""
	s.PodsError = ""
	s.PodDetailsError = ""
	s.LogsError = ""
	s.NamespaceHealthError = ""
}

// NamespaceExists checks if the given namespace name exists in the namespace list
func (s *AppState) NamespaceExists(name string) bool {
	for _, ns := range s.Namespaces.Data {
		if ns.Name == name {
			return true
		}
	}
	return false
}

// PodExists checks if the given pod name exists in the pod list
func (s *AppState) PodExists(name string) bool {
	for _, pod := range s.Pods.Data {
		if pod.Name == name {
			return true
		}
	}
	return false
}
