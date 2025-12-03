package kube

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
)

// Client wraps the Kubernetes clientset and provides methods for kubectl-medic
// This client is read-only focused, following the project's philosophy:
// "A doctor who diagnoses, not a surgeon who performs operations"
type Client struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

// NewClient creates a new Kubernetes client from kubeconfig
// It attempts to load configuration in the following order:
// 1. Provided kubeconfig path
// 2. KUBECONFIG environment variable
// 3. ~/.kube/config
// 4. In-cluster config (when running inside a pod)
func NewClient(kubeconfigPath string, contextName string) (*Client, error) {
	var config *rest.Config
	var err error

	// Determine kubeconfig path
	if kubeconfigPath == "" {
		// Try KUBECONFIG env var first
		if envKubeconfig := os.Getenv("KUBECONFIG"); envKubeconfig != "" {
			kubeconfigPath = envKubeconfig
		} else {
			// Default to ~/.kube/config
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("failed to get home directory: %w", err)
			}
			kubeconfigPath = filepath.Join(home, ".kube", "config")
		}
	}

	// Try to load kubeconfig from file
	if _, err := os.Stat(kubeconfigPath); err == nil {
		// File exists, load it
		loadingRules := &clientcmd.ClientConfigLoadingRules{
			ExplicitPath: kubeconfigPath,
		}

		configOverrides := &clientcmd.ConfigOverrides{}
		if contextName != "" {
			configOverrides.CurrentContext = contextName
		}

		kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
			loadingRules,
			configOverrides,
		)

		config, err = kubeConfig.ClientConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s: %w", kubeconfigPath, err)
		}
	} else {
		// Try in-cluster config as fallback
		config, err = rest.InClusterConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to load kubeconfig from %s and in-cluster config: %w", kubeconfigPath, err)
		}
	}

	// Create the clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kubernetes clientset: %w", err)
	}

	return &Client{
		clientset: clientset,
		config:    config,
	}, nil
}

// ListNamespaces retrieves all namespaces from the cluster
// Returns NamespaceInfo with name, status, and age
// Complies with REQUIREMENTS.md: "Display: name, status, age"
func (c *Client) ListNamespaces() ([]state.NamespaceInfo, error) {
	ctx := context.Background()

	namespaceList, err := c.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	namespaces := make([]state.NamespaceInfo, 0, len(namespaceList.Items))

	for _, ns := range namespaceList.Items {
		namespaces = append(namespaces, state.NamespaceInfo{
			Name:   ns.Name,
			Status: string(ns.Status.Phase),
			Age:    formatAge(ns.CreationTimestamp.Time),
		})
	}

	// Sort by name for consistent display
	sort.Slice(namespaces, func(i, j int) bool {
		return namespaces[i].Name < namespaces[j].Name
	})

	return namespaces, nil
}

// ListPods retrieves all pods in a given namespace
// Returns PodInfo with name, ready count, status, restarts, age
// Complies with REQUIREMENTS.md: "name, restarts, ready count, status, age"
func (c *Client) ListPods(namespace string) ([]state.PodInfo, error) {
	ctx := context.Background()

	// If namespace is empty, list across all namespaces
	listOptions := metav1.ListOptions{}

	var podList *corev1.PodList
	var err error

	if namespace == "" {
		podList, err = c.clientset.CoreV1().Pods(corev1.NamespaceAll).List(ctx, listOptions)
	} else {
		podList, err = c.clientset.CoreV1().Pods(namespace).List(ctx, listOptions)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to list pods in namespace %s: %w", namespace, err)
	}

	pods := make([]state.PodInfo, 0, len(podList.Items))

	for _, pod := range podList.Items {
		pods = append(pods, podToPodInfo(&pod))
	}

	// Sort by name for consistent display
	sort.Slice(pods, func(i, j int) bool {
		return pods[i].Name < pods[j].Name
	})

	return pods, nil
}

// GetPod retrieves detailed information about a specific pod
// Returns the full corev1.Pod object for detailed analysis
func (c *Client) GetPod(namespace, name string) (*corev1.Pod, error) {
	ctx := context.Background()

	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}

	return pod, nil
}

// GetPodEvents retrieves events related to a specific pod
// Events are essential for diagnostics (REQUIREMENTS.md: "Display relevant events")
func (c *Client) GetPodEvents(namespace, podName string) ([]state.EventInfo, error) {
	ctx := context.Background()

	// List events in the namespace, then filter by pod
	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get events for pod %s/%s: %w", namespace, podName, err)
	}

	events := make([]state.EventInfo, 0, len(eventList.Items))

	for _, event := range eventList.Items {
		events = append(events, state.EventInfo{
			Type:      event.Type,
			Reason:    event.Reason,
			Message:   event.Message,
			Count:     event.Count,
			FirstSeen: formatAge(event.FirstTimestamp.Time),
			LastSeen:  formatAge(event.LastTimestamp.Time),
		})
	}

	// Sort by last seen time (most recent first)
	sort.Slice(events, func(i, j int) bool {
		return events[i].LastSeen < events[j].LastSeen
	})

	return events, nil
}

// StreamLogs retrieves logs for a specific container in a pod
// Supports follow mode for live log streaming (USE_CASES.md: UC4)
// Returns an io.ReadCloser that must be closed by the caller
func (c *Client) StreamLogs(namespace, podName, containerName string, follow bool, tailLines int64) (io.ReadCloser, error) {
	ctx := context.Background()

	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    follow,
	}

	if tailLines > 0 {
		logOptions.TailLines = &tailLines
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, logOptions)

	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to stream logs for pod %s/%s container %s: %w",
			namespace, podName, containerName, err)
	}

	return stream, nil
}

// GetPodLogs retrieves logs as a string (convenience method)
// For simple log display without streaming
func (c *Client) GetPodLogs(namespace, podName, containerName string, tailLines int64) (string, error) {
	stream, err := c.StreamLogs(namespace, podName, containerName, false, tailLines)
	if err != nil {
		return "", err
	}
	defer stream.Close()

	logs, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read logs: %w", err)
	}

	return string(logs), nil
}

// GetPodContainers returns a list of container names in a pod
// Useful for multi-container pods (UI_DESIGN.md: "Switch containers with 'c'")
func (c *Client) GetPodContainers(namespace, podName string) ([]string, error) {
	pod, err := c.GetPod(namespace, podName)
	if err != nil {
		return nil, err
	}

	containers := make([]string, 0, len(pod.Spec.Containers)+len(pod.Spec.InitContainers))

	// Add init containers
	for _, container := range pod.Spec.InitContainers {
		containers = append(containers, fmt.Sprintf("%s (init)", container.Name))
	}

	// Add regular containers
	for _, container := range pod.Spec.Containers {
		containers = append(containers, container.Name)
	}

	return containers, nil
}

// podToPodInfo converts a corev1.Pod to state.PodInfo for display
// Extracts the essential information needed by the TUI
func podToPodInfo(pod *corev1.Pod) state.PodInfo {
	// Calculate ready containers
	readyContainers := 0
	totalContainers := len(pod.Spec.Containers)

	for _, status := range pod.Status.ContainerStatuses {
		if status.Ready {
			readyContainers++
		}
	}

	// Calculate total restarts across all containers
	totalRestarts := 0
	for _, status := range pod.Status.ContainerStatuses {
		totalRestarts += int(status.RestartCount)
	}

	// Determine pod status
	// This follows kubectl's logic for determining pod phase/status
	status := getPodStatus(pod)

	return state.PodInfo{
		Name:      pod.Name,
		Namespace: pod.Namespace,
		Ready:     fmt.Sprintf("%d/%d", readyContainers, totalContainers),
		Status:    status,
		Restarts:  totalRestarts,
		Age:       formatAge(pod.CreationTimestamp.Time),
	}
}

// getPodStatus determines the human-readable status of a pod
// This matches kubectl's logic for showing pod status
func getPodStatus(pod *corev1.Pod) string {
	// Check for pod phase
	phase := string(pod.Status.Phase)

	// Check for special container states that override phase
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			reason := status.State.Waiting.Reason
			if reason != "" {
				// Common waiting reasons that should be shown as status
				switch reason {
				case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull",
					"CreateContainerConfigError", "InvalidImageName":
					return reason
				}
			}
		}

		if status.State.Terminated != nil {
			reason := status.State.Terminated.Reason
			if reason != "" {
				return reason
			}
		}
	}

	// Check pod conditions for more detailed status
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			if condition.Reason == corev1.PodReasonUnschedulable {
				return "Pending"
			}
		}
	}

	return phase
}

// formatAge converts a time.Time to a human-readable age string
// Formats like kubectl: "5d", "2h", "30m", "45s"
func formatAge(t time.Time) string {
	if t.IsZero() {
		return "<unknown>"
	}

	duration := time.Since(t)

	// Days
	if duration.Hours() >= 24 {
		days := int(duration.Hours() / 24)
		return fmt.Sprintf("%dd", days)
	}

	// Hours
	if duration.Hours() >= 1 {
		hours := int(duration.Hours())
		return fmt.Sprintf("%dh", hours)
	}

	// Minutes
	if duration.Minutes() >= 1 {
		minutes := int(duration.Minutes())
		return fmt.Sprintf("%dm", minutes)
	}

	// Seconds
	seconds := int(duration.Seconds())
	return fmt.Sprintf("%ds", seconds)
}

// HealthCheck verifies the client can connect to the Kubernetes API
// Useful for startup validation and error handling (REQUIREMENTS.md: "Helpful error messages for RBAC issues")
func (c *Client) HealthCheck() error {
	// Try to get server version as a simple health check
	_, err := c.clientset.Discovery().ServerVersion()
	if err != nil {
		return fmt.Errorf("failed to connect to Kubernetes API: %w", err)
	}

	return nil
}
