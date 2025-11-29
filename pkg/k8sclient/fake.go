package k8sclient

// FakeClient is an in-memory implementation of Client.
// Great for early development and unit tests.
type FakeClient struct {
	pods []Pod
}

// NewFakeClient creates a fake client with some example data.
func NewFakeClient() *FakeClient {
	return &FakeClient{
		pods: []Pod{
			{Namespace: "default", Name: "api-123", Phase: PodRunning},
			{Namespace: "default", Name: "frontend-1", Phase: PodPending},
			{Namespace: "kube-system", Name: "coredns-abc", Phase: PodRunning},
			{Namespace: "payments", Name: "payments-processor-0", Phase: PodFailed},
		},
	}
}

func (f *FakeClient) ListPods(namespace string) ([]Pod, error) {
	if namespace == "" {
		return f.pods, nil
	}

	var filtered []Pod
	for _, p := range f.pods {
		if p.Namespace == namespace {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}
