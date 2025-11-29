package k8sclient

// PodPhase is a simplified version of Kubernetes pod phases.
type PodPhase string

const (
	PodPending   PodPhase = "Pending"
	PodRunning   PodPhase = "Running"
	PodFailed    PodPhase = "Failed"
	PodSucceeded PodPhase = "Succeeded"
	PodUnknown   PodPhase = "Unknown"
)

// Pod is a minimal view of what we care about for now.
type Pod struct {
	Namespace string
	Name      string
	Phase     PodPhase
}

// Client defines what our scanner needs from "the cluster".
type Client interface {
	// ListPods returns pods across all namespaces if namespace == "".
	ListPods(namespace string) ([]Pod, error)
}
