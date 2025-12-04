package types

// Diagnostic represents a single diagnostic finding about a pod
// This type is shared between state and analysis packages to avoid import cycles
type Diagnostic struct {
	ID                string   // Unique identifier (e.g., "crashloop_backoff")
	Severity          Severity // Severity level
	Title             string   // Short summary
	Description       string   // Human-readable explanation
	SuggestedCommands []string // kubectl commands user can run
}

// Severity represents the severity level of a diagnostic finding
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// EventInfo holds information about a Kubernetes event
// Shared between state and analysis packages
type EventInfo struct {
	Type      string // "Normal", "Warning"
	Reason    string
	Message   string
	Count     int32
	FirstSeen string
	LastSeen  string
}

// NamespaceHealth holds health summary information for a namespace
// Shared between state and analysis packages
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
