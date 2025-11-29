package scanner

import "github.com/NlCKDEV/kubectl-medic/pkg/k8sclient"

// Issue represents a problem found with a pod.
type Issue struct {
	Namespace  string
	PodName    string
	Phase      k8sclient.PodPhase
	Severity   string
	Suggestion string
}

// Scanner uses a k8sclient.Client to find unhealthy pods.
type Scanner struct {
	client k8sclient.Client
}

func NewScanner(c k8sclient.Client) *Scanner {
	return &Scanner{client: c}
}

// ScanNamespace scans a single namespace (or all if namespace == "").
func (s *Scanner) ScanNamespace(namespace string) ([]Issue, error) {
	pods, err := s.client.ListPods(namespace)
	if err != nil {
		return nil, err
	}

	var issues []Issue
	for _, pod := range pods {
		// Very naive "rules" to start with
		switch pod.Phase {
		case k8sclient.PodRunning:
			// healthy, skip
			continue
		case k8sclient.PodPending:
			issues = append(issues, Issue{
				Namespace:  pod.Namespace,
				PodName:    pod.Name,
				Phase:      pod.Phase,
				Severity:   "warning",
				Suggestion: "Pod is Pending. Check events: 'kubectl describe pod " + pod.Name + " -n " + pod.Namespace + "'.",
			})
		case k8sclient.PodFailed:
			issues = append(issues, Issue{
				Namespace:  pod.Namespace,
				PodName:    pod.Name,
				Phase:      pod.Phase,
				Severity:   "critical",
				Suggestion: "Pod Failed. Inspect logs and events, consider restarting or recreating deployment.",
			})
		default:
			issues = append(issues, Issue{
				Namespace:  pod.Namespace,
				PodName:    pod.Name,
				Phase:      pod.Phase,
				Severity:   "unknown",
				Suggestion: "Unknown state. Check 'kubectl describe' and 'kubectl logs'.",
			})
		}
	}

	return issues, nil
}
