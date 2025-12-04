package analysis

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
)

// Test helper functions for building test fixtures

// buildHealthyPod creates a healthy running pod with all containers ready
func buildHealthyPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:1.21",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					RestartCount: 0,
				},
			},
		},
	}
}

// buildPodWithCrashLoop creates a pod with a container in CrashLoopBackOff
func buildPodWithCrashLoop(name, namespace, containerName string, restarts int32, exitCode int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         containerName,
					Ready:        false,
					RestartCount: restarts,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "Back-off restarting failed container",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: exitCode,
							Reason:   "Error",
							Message:  "Container failed",
						},
					},
				},
			},
		},
	}
}

// buildPodWithHighRestarts creates a pod with high restart count but currently running
func buildPodWithHighRestarts(name, namespace, containerName string, restarts int32) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         containerName,
					Ready:        true,
					RestartCount: restarts,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
				},
			},
		},
	}
}

// buildPodWithImagePullError creates a pod with ImagePullBackOff
func buildPodWithImagePullError(name, namespace, containerName, imageName, reason string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: imageName,
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  containerName,
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  reason,
							Message: "Failed to pull image",
						},
					},
					RestartCount: 0,
				},
			},
		},
	}
}

// buildPendingPodWithSchedulingFailure creates a pod that cannot be scheduled
func buildPendingPodWithSchedulingFailure(name, namespace, reason, message string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "nginx:1.21",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:    corev1.PodScheduled,
					Status:  corev1.ConditionFalse,
					Reason:  reason,
					Message: message,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{},
		},
	}
}

// buildPodWithReadinessFailure creates a running pod with failing readiness probe
func buildPodWithReadinessFailure(name, namespace, containerName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  containerName,
					Ready: false, // Not ready despite running
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{},
					},
					RestartCount: 0,
				},
			},
		},
	}
}

// buildPodWithOOMKilled creates a pod with OOMKilled container
func buildPodWithOOMKilled(name, namespace, containerName string, memLimit, memRequest string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  containerName,
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  containerName,
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason: "CrashLoopBackOff",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 137,
							Reason:   "OOMKilled",
							Message:  "Container was OOM killed",
						},
					},
					RestartCount: 5,
				},
			},
		},
	}

	// Add resource limits if provided
	if memLimit != "" || memRequest != "" {
		pod.Spec.Containers[0].Resources = corev1.ResourceRequirements{
			Limits:   corev1.ResourceList{},
			Requests: corev1.ResourceList{},
		}
		// Note: In real test, we'd parse these strings to resource.Quantity
		// For now, we'll keep it simple and let the diagnostic show them
	}

	return pod
}

// buildEvent creates a test event
func buildEvent(eventType, reason, message string, count int32) state.EventInfo {
	return state.EventInfo{
		Type:    eventType,
		Reason:  reason,
		Message: message,
		Count:   count,
	}
}

// Tests for AnalyzePod

func TestAnalyzePod_Healthy(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Should return exactly one diagnostic with "healthy" ID
	require.Len(t, diagnostics, 1, "Healthy pod should return exactly one diagnostic")

	diag := diagnostics[0]
	assert.Equal(t, "healthy", diag.ID)
	assert.Equal(t, SeverityInfo, diag.Severity)
	assert.Contains(t, diag.Title, "healthy")
	assert.NotEmpty(t, diag.Description)
	assert.NotEmpty(t, diag.SuggestedCommands)
}

func TestAnalyzePod_CrashLoopBackOff(t *testing.T) {
	engine := NewEngine()
	pod := buildPodWithCrashLoop("test-pod", "default", "api", 12, 1)
	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Should detect crash loop
	require.NotEmpty(t, diagnostics, "CrashLoopBackOff should be detected")

	// Find the crashloop diagnostic
	var crashloopDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "crashloop_backoff" {
			crashloopDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, crashloopDiag, "Should have crashloop_backoff diagnostic")
	assert.Equal(t, SeverityError, crashloopDiag.Severity)
	assert.Contains(t, crashloopDiag.Title, "CrashLoopBackOff")
	assert.Contains(t, crashloopDiag.Title, "api")
	assert.Contains(t, crashloopDiag.Description, "Restart count: 12")
	assert.Contains(t, crashloopDiag.Description, "Exit code: 1")
	assert.NotEmpty(t, crashloopDiag.SuggestedCommands)

	// Should suggest kubectl logs with --previous
	foundPreviousFlag := false
	for _, cmd := range crashloopDiag.SuggestedCommands {
		if strings.Contains(cmd, "--previous") {
			foundPreviousFlag = true
			break
		}
	}
	assert.True(t, foundPreviousFlag, "Should suggest checking previous logs")
}

func TestAnalyzePod_HighRestarts(t *testing.T) {
	engine := NewEngine()
	pod := buildPodWithHighRestarts("test-pod", "default", "app", 10)
	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Should detect high restart count
	var highRestartDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "high_restart_count" {
			highRestartDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, highRestartDiag, "Should detect high restart count")
	assert.Equal(t, SeverityWarning, highRestartDiag.Severity)
	assert.Contains(t, highRestartDiag.Title, "high restart count")
	assert.Contains(t, highRestartDiag.Description, "10 times")
	assert.NotEmpty(t, highRestartDiag.SuggestedCommands)
}

func TestAnalyzePod_ImagePullBackOff(t *testing.T) {
	tests := []struct {
		name           string
		reason         string
		expectedID     string
		expectedTitle  string
	}{
		{
			name:           "ImagePullBackOff",
			reason:         "ImagePullBackOff",
			expectedID:     "image_pull_error",
			expectedTitle:  "Cannot pull image",
		},
		{
			name:           "ErrImagePull",
			reason:         "ErrImagePull",
			expectedID:     "image_pull_error",
			expectedTitle:  "Cannot pull image",
		},
		{
			name:           "InvalidImageName",
			reason:         "InvalidImageName",
			expectedID:     "invalid_image_name",
			expectedTitle:  "Invalid image name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := NewEngine()
			pod := buildPodWithImagePullError("test-pod", "default", "web", "nginx:invalid-tag", tt.reason)

			events := []state.EventInfo{
				buildEvent("Warning", "Failed", "Failed to pull image nginx:invalid-tag", 5),
			}

			ctx := PodContext{
				Pod:    pod,
				Events: events,
			}

			diagnostics := engine.AnalyzePod(ctx)

			// Find the image pull diagnostic
			var imagePullDiag *Diagnostic
			for i := range diagnostics {
				if diagnostics[i].ID == tt.expectedID {
					imagePullDiag = &diagnostics[i]
					break
				}
			}

			require.NotNil(t, imagePullDiag, "Should detect image pull error")
			assert.Equal(t, SeverityError, imagePullDiag.Severity)
			assert.Contains(t, imagePullDiag.Title, tt.expectedTitle)
			assert.Contains(t, imagePullDiag.Description, "nginx:invalid-tag")
			assert.NotEmpty(t, imagePullDiag.SuggestedCommands)

			// Should suggest checking secrets for ImagePullBackOff/ErrImagePull
			if tt.reason == "ImagePullBackOff" || tt.reason == "ErrImagePull" {
				foundSecretCheck := false
				for _, cmd := range imagePullDiag.SuggestedCommands {
					if strings.Contains(cmd, "secret") {
						foundSecretCheck = true
						break
					}
				}
				assert.True(t, foundSecretCheck, "Should suggest checking secrets")
			}
		})
	}
}

func TestAnalyzePod_SchedulingFailed(t *testing.T) {
	engine := NewEngine()
	pod := buildPendingPodWithSchedulingFailure(
		"test-pod",
		"default",
		"Unschedulable",
		"0/3 nodes are available: 3 Insufficient cpu",
	)

	events := []state.EventInfo{
		buildEvent("Warning", "FailedScheduling", "0/3 nodes are available: 3 Insufficient cpu", 10),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find scheduling diagnostic
	var schedDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "scheduling_failed" {
			schedDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, schedDiag, "Should detect scheduling failure")
	assert.Equal(t, SeverityError, schedDiag.Severity)
	assert.Contains(t, schedDiag.Title, "cannot be scheduled")
	assert.Contains(t, schedDiag.Description, "Unschedulable")
	assert.Contains(t, schedDiag.Description, "Insufficient cpu")
	assert.NotEmpty(t, schedDiag.SuggestedCommands)

	// Should suggest checking nodes
	foundNodeCheck := false
	for _, cmd := range schedDiag.SuggestedCommands {
		if strings.Contains(cmd, "nodes") {
			foundNodeCheck = true
			break
		}
	}
	assert.True(t, foundNodeCheck, "Should suggest checking nodes")
}

func TestAnalyzePod_ReadinessProbeFailed(t *testing.T) {
	engine := NewEngine()
	pod := buildPodWithReadinessFailure("test-pod", "default", "app")

	events := []state.EventInfo{
		buildEvent("Warning", "Unhealthy", "Readiness probe failed: HTTP probe failed with statuscode: 500", 3),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find readiness probe diagnostic
	var probeDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "readiness_probe_failed" {
			probeDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, probeDiag, "Should detect readiness probe failure")
	assert.Equal(t, SeverityWarning, probeDiag.Severity)
	assert.Contains(t, probeDiag.Title, "readiness")
	assert.Contains(t, probeDiag.Title, "app")
	assert.Contains(t, probeDiag.Description, "not ready")
	assert.NotEmpty(t, probeDiag.SuggestedCommands)
}

func TestAnalyzePod_OOMKilled(t *testing.T) {
	engine := NewEngine()
	pod := buildPodWithOOMKilled("test-pod", "default", "app", "512Mi", "256Mi")
	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find OOMKilled diagnostic
	var oomDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "oom_killed" {
			oomDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, oomDiag, "Should detect OOMKilled")
	assert.Equal(t, SeverityError, oomDiag.Severity)
	assert.Contains(t, oomDiag.Title, "OOM")
	assert.Contains(t, oomDiag.Title, "app")
	assert.Contains(t, oomDiag.Description, "Exit code: 137")
	assert.Contains(t, oomDiag.Description, "out-of-memory")
	assert.NotEmpty(t, oomDiag.SuggestedCommands)

	// Should suggest kubectl top
	foundTopCheck := false
	for _, cmd := range oomDiag.SuggestedCommands {
		if strings.Contains(cmd, "top") {
			foundTopCheck = true
			break
		}
	}
	assert.True(t, foundTopCheck, "Should suggest checking resource usage with kubectl top")
}

func TestAnalyzePod_ResourceQuotaExceeded(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Warning", "FailedCreate", "Error creating: pods \"test-pod\" is forbidden: exceeded quota", 1),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find resource quota diagnostic
	var quotaDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "resource_quota_exceeded" {
			quotaDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, quotaDiag, "Should detect resource quota exceeded")
	assert.Equal(t, SeverityError, quotaDiag.Severity)
	assert.Contains(t, quotaDiag.Title, "quota")
	assert.Contains(t, quotaDiag.Description, "exceeded quota")
	assert.NotEmpty(t, quotaDiag.SuggestedCommands)

	// Should suggest checking quota
	foundQuotaCheck := false
	for _, cmd := range quotaDiag.SuggestedCommands {
		if strings.Contains(cmd, "quota") {
			foundQuotaCheck = true
			break
		}
	}
	assert.True(t, foundQuotaCheck, "Should suggest checking quota")
}

func TestAnalyzePod_MultipleIssues(t *testing.T) {
	engine := NewEngine()

	// Create a pod with both CrashLoopBackOff AND high restarts
	pod := buildPodWithCrashLoop("test-pod", "default", "api", 15, 1)

	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Should detect crashloop (high restarts is implicit when in crashloop)
	require.NotEmpty(t, diagnostics, "Should detect multiple issues")

	// Find crashloop
	foundCrashloop := false
	for _, diag := range diagnostics {
		if diag.ID == "crashloop_backoff" {
			foundCrashloop = true
			assert.Equal(t, SeverityError, diag.Severity)
		}
	}
	assert.True(t, foundCrashloop, "Should detect CrashLoopBackOff")
}

func TestAnalyzePod_HealthyWithPastRestarts(t *testing.T) {
	engine := NewEngine()

	// Healthy pod but with some past restarts
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.ContainerStatuses[0].RestartCount = 3

	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Should still report as healthy (restarts < 5)
	require.Len(t, diagnostics, 1)
	assert.Equal(t, "healthy", diagnostics[0].ID)
	assert.Equal(t, SeverityInfo, diagnostics[0].Severity)
	assert.Contains(t, diagnostics[0].Description, "Total restarts: 3")
}

// Tests for SummarizeNamespace

func TestSummarizeNamespace_AllHealthy(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildHealthyPod("pod2", "default"),
		buildHealthyPod("pod3", "default"),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {},
		"pod3": {},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, "default", health.Namespace)
	assert.Equal(t, 3, health.TotalPods)
	assert.Equal(t, 0, health.FailingPods)
	assert.Equal(t, 0, health.CrashLoopPods)
	assert.Equal(t, 0, health.ImagePullPods)
	assert.Equal(t, 0, health.SchedulingPods)
	assert.Equal(t, 0, health.ProbeIssuePods)
	assert.Equal(t, 0, health.ResourceIssuePods)
}

func TestSummarizeNamespace_WithCrashLoops(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPodWithCrashLoop("pod2", "default", "api", 10, 1),
		buildPodWithCrashLoop("pod3", "default", "web", 15, 137),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {},
		"pod3": {},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 3, health.TotalPods)
	assert.Equal(t, 2, health.FailingPods, "Two pods are failing")
	assert.Equal(t, 2, health.CrashLoopPods, "Two pods in crash loop")
}

func TestSummarizeNamespace_WithImagePullErrors(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPodWithImagePullError("pod2", "default", "api", "invalid:tag", "ImagePullBackOff"),
		buildPodWithImagePullError("pod3", "default", "web", "missing:image", "ErrImagePull"),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {buildEvent("Warning", "Failed", "Failed to pull image", 3)},
		"pod3": {buildEvent("Warning", "Failed", "Image pull failed", 2)},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 3, health.TotalPods)
	assert.Equal(t, 2, health.FailingPods)
	assert.Equal(t, 2, health.ImagePullPods)
}

func TestSummarizeNamespace_WithSchedulingFailures(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPendingPodWithSchedulingFailure("pod2", "default", "Unschedulable", "Insufficient CPU"),
		buildPendingPodWithSchedulingFailure("pod3", "default", "Unschedulable", "Insufficient memory"),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {buildEvent("Warning", "FailedScheduling", "Insufficient CPU", 5)},
		"pod3": {buildEvent("Warning", "FailedScheduling", "Insufficient memory", 3)},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 3, health.TotalPods)
	assert.Equal(t, 2, health.FailingPods)
	assert.Equal(t, 2, health.SchedulingPods)
}

func TestSummarizeNamespace_WithProbeFailures(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPodWithReadinessFailure("pod2", "default", "api"),
		buildPodWithReadinessFailure("pod3", "default", "web"),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {buildEvent("Warning", "Unhealthy", "Readiness probe failed", 2)},
		"pod3": {buildEvent("Warning", "Unhealthy", "Liveness probe failed", 1)},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 3, health.TotalPods)
	assert.Equal(t, 2, health.FailingPods)
	assert.Equal(t, 2, health.ProbeIssuePods)
}

func TestSummarizeNamespace_WithResourceIssues(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPodWithOOMKilled("pod2", "default", "api", "512Mi", "256Mi"),
	}

	quotaPod := buildHealthyPod("pod3", "default")
	quotaPod.Status.Phase = corev1.PodPending

	pods = append(pods, quotaPod)

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {},
		"pod3": {buildEvent("Warning", "FailedCreate", "exceeded quota", 1)},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 3, health.TotalPods)
	assert.Greater(t, health.FailingPods, 0, "Should have at least one failing pod")
	assert.Greater(t, health.ResourceIssuePods, 0, "Should detect resource issues")
}

func TestSummarizeNamespace_WithMultipleIssueTypes(t *testing.T) {
	pods := []*corev1.Pod{
		buildHealthyPod("pod1", "default"),
		buildPodWithCrashLoop("pod2", "default", "api", 10, 1),
		buildPodWithImagePullError("pod3", "default", "web", "bad:image", "ImagePullBackOff"),
		buildPendingPodWithSchedulingFailure("pod4", "default", "Unschedulable", "No nodes"),
		buildPodWithReadinessFailure("pod5", "default", "app"),
	}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
		"pod2": {},
		"pod3": {buildEvent("Warning", "Failed", "Image pull failed", 2)},
		"pod4": {buildEvent("Warning", "FailedScheduling", "No nodes available", 5)},
		"pod5": {buildEvent("Warning", "Unhealthy", "Readiness probe failed", 3)},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 5, health.TotalPods)
	assert.Equal(t, 4, health.FailingPods, "Four pods should be failing")
	assert.Equal(t, 1, health.CrashLoopPods)
	assert.Equal(t, 1, health.ImagePullPods)
	assert.Equal(t, 1, health.SchedulingPods)
	assert.Equal(t, 1, health.ProbeIssuePods)
}

func TestSummarizeNamespace_NoDuplicateCounting(t *testing.T) {
	// Pod with both crash loop AND OOM (should be counted once in each category but only once as failing)
	pod := buildPodWithCrashLoop("pod1", "default", "api", 10, 137)
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Reason = "OOMKilled"

	pods := []*corev1.Pod{pod}

	eventsByPod := map[string][]state.EventInfo{
		"pod1": {},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 1, health.TotalPods)
	assert.Equal(t, 1, health.FailingPods, "Should count pod only once as failing")
	assert.Equal(t, 1, health.CrashLoopPods)
	assert.Equal(t, 1, health.ResourceIssuePods, "Should also detect OOM")
}

func TestSummarizeNamespace_EmptyNamespace(t *testing.T) {
	pods := []*corev1.Pod{}
	eventsByPod := map[string][]state.EventInfo{}

	health := SummarizeNamespace("empty", pods, eventsByPod)

	assert.Equal(t, "empty", health.Namespace)
	assert.Equal(t, 0, health.TotalPods)
	assert.Equal(t, 0, health.FailingPods)
	assert.Equal(t, 0, health.CrashLoopPods)
	assert.Equal(t, 0, health.ImagePullPods)
	assert.Equal(t, 0, health.SchedulingPods)
	assert.Equal(t, 0, health.ProbeIssuePods)
	assert.Equal(t, 0, health.ResourceIssuePods)
	assert.Equal(t, 0, health.StorageIssuePods)
	assert.Equal(t, 0, health.ConfigIssuePods)
	assert.Equal(t, 0, health.InitFailurePods)
}

func TestSummarizeNamespace_WithNewCategories(t *testing.T) {
	// Pod with storage issue
	storagePod := buildHealthyPod("storage-pod", "default")
	storagePod.Status.Phase = corev1.PodPending

	// Pod with config issue
	configPod := buildHealthyPod("config-pod", "default")
	configPod.Status.Phase = corev1.PodPending

	// Pod with init container failure
	initPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "init-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{Name: "init", Image: "alpine"},
			},
			Containers: []corev1.Container{
				{Name: "app", Image: "myapp"},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
						},
					},
				},
			},
		},
	}

	pods := []*corev1.Pod{
		buildHealthyPod("healthy-pod", "default"),
		storagePod,
		configPod,
		initPod,
	}

	eventsByPod := map[string][]state.EventInfo{
		"healthy-pod": {},
		"storage-pod": {
			buildEvent("Warning", "FailedMount", "MountVolume.SetUp failed", 3),
		},
		"config-pod": {
			buildEvent("Warning", "FailedMount", "configmap \"app-config\" not found", 2),
		},
		"init-pod": {},
	}

	health := SummarizeNamespace("default", pods, eventsByPod)

	assert.Equal(t, 4, health.TotalPods)
	assert.Equal(t, 3, health.FailingPods, "Three pods should be failing")
	assert.Equal(t, 1, health.StorageIssuePods, "One pod with storage issue")
	assert.Equal(t, 1, health.ConfigIssuePods, "One pod with config issue")
	assert.Equal(t, 1, health.InitFailurePods, "One pod with init failure")
}

// Tests for New Diagnostic Categories (Storage, Config, Init Containers)

func TestAnalyzePod_StorageVolumeFailed(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Warning", "FailedMount", "MountVolume.SetUp failed for volume \"data-volume\" : volume not found", 5),
		buildEvent("Warning", "FailedAttachVolume", "AttachVolume.Attach failed for volume \"pvc-123\" : rpc error", 2),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find storage diagnostic
	var storageDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "storage.volume.failed_mount" {
			storageDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, storageDiag, "Should detect volume mount failure")
	assert.Equal(t, SeverityError, storageDiag.Severity)
	assert.Contains(t, storageDiag.Title, "volume")
	assert.Contains(t, storageDiag.Description, "mounting issues")
	assert.Contains(t, storageDiag.Description, "FailedMount")
	assert.NotEmpty(t, storageDiag.SuggestedCommands)

	// Should suggest checking PVC
	foundPVCCheck := false
	for _, cmd := range storageDiag.SuggestedCommands {
		if strings.Contains(cmd, "pvc") {
			foundPVCCheck = true
			break
		}
	}
	assert.True(t, foundPVCCheck, "Should suggest checking PVC")
}

func TestAnalyzePod_PVCNotFound(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Warning", "FailedMount", "persistentvolumeclaim \"data-pvc\" not found", 3),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find PVC not found diagnostic
	var pvcDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "storage.pvc.not_found" {
			pvcDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, pvcDiag, "Should detect PVC not found")
	assert.Equal(t, SeverityError, pvcDiag.Severity)
	assert.Contains(t, pvcDiag.Title, "not found")
	assert.Contains(t, pvcDiag.Description, "not found")
	assert.Contains(t, pvcDiag.Description, "not exist")
	assert.NotEmpty(t, pvcDiag.SuggestedCommands)
}

func TestAnalyzePod_PVCNotBound(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Normal", "Waiting", "Waiting for persistentvolumeclaim \"data-pvc\" to be bound", 10),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find PVC not bound diagnostic
	var pvcDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "storage.pvc.not_bound" {
			pvcDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, pvcDiag, "Should detect PVC not bound")
	assert.Equal(t, SeverityError, pvcDiag.Severity)
	assert.Contains(t, pvcDiag.Title, "not bound")
	assert.Contains(t, pvcDiag.Description, "not bound")
	assert.NotEmpty(t, pvcDiag.SuggestedCommands)

	// Should suggest checking storage class
	foundStorageClassCheck := false
	for _, cmd := range pvcDiag.SuggestedCommands {
		if strings.Contains(cmd, "storageclass") {
			foundStorageClassCheck = true
			break
		}
	}
	assert.True(t, foundStorageClassCheck, "Should suggest checking storage class")
}

func TestAnalyzePod_ConfigMapMissing(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Warning", "FailedMount", "configmap \"app-config\" not found", 5),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find ConfigMap diagnostic
	var configDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "config.configmap.missing" {
			configDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, configDiag, "Should detect missing ConfigMap")
	assert.Equal(t, SeverityError, configDiag.Severity)
	assert.Contains(t, configDiag.Title, "ConfigMap")
	assert.Contains(t, configDiag.Title, "not found")
	assert.Contains(t, configDiag.Description, "does not exist")
	assert.NotEmpty(t, configDiag.SuggestedCommands)

	// Should suggest getting configmap
	foundConfigMapCheck := false
	for _, cmd := range configDiag.SuggestedCommands {
		if strings.Contains(cmd, "configmap") {
			foundConfigMapCheck = true
			break
		}
	}
	assert.True(t, foundConfigMapCheck, "Should suggest checking ConfigMap")
}

func TestAnalyzePod_SecretMissing(t *testing.T) {
	engine := NewEngine()
	pod := buildHealthyPod("test-pod", "default")
	pod.Status.Phase = corev1.PodPending

	events := []state.EventInfo{
		buildEvent("Warning", "FailedMount", "secret \"db-credentials\" not found", 3),
	}

	ctx := PodContext{
		Pod:    pod,
		Events: events,
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find Secret diagnostic
	var secretDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "config.secret.missing" {
			secretDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, secretDiag, "Should detect missing Secret")
	assert.Equal(t, SeverityError, secretDiag.Severity)
	assert.Contains(t, secretDiag.Title, "Secret")
	assert.Contains(t, secretDiag.Title, "not found")
	assert.Contains(t, secretDiag.Description, "does not exist")
	assert.NotEmpty(t, secretDiag.SuggestedCommands)

	// Should suggest getting secret
	foundSecretCheck := false
	for _, cmd := range secretDiag.SuggestedCommands {
		if strings.Contains(cmd, "secret") {
			foundSecretCheck = true
			break
		}
	}
	assert.True(t, foundSecretCheck, "Should suggest checking Secret")
}

func TestAnalyzePod_InitContainerFailed(t *testing.T) {
	engine := NewEngine()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  "init-db",
					Image: "busybox:latest",
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-db",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 1,
							Reason:   "Error",
							Message:  "Database connection failed",
						},
					},
					RestartCount: 0,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{},
		},
	}

	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find init container diagnostic
	var initDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "init.failed" {
			initDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, initDiag, "Should detect failed init container")
	assert.Equal(t, SeverityError, initDiag.Severity)
	assert.Contains(t, initDiag.Title, "Init container")
	assert.Contains(t, initDiag.Title, "init-db")
	assert.Contains(t, initDiag.Description, "Exit code: 1")
	assert.Contains(t, initDiag.Description, "blocking")
	assert.NotEmpty(t, initDiag.SuggestedCommands)

	// Should suggest checking init container logs
	foundLogsCheck := false
	for _, cmd := range initDiag.SuggestedCommands {
		if strings.Contains(cmd, "logs") && strings.Contains(cmd, "init-db") {
			foundLogsCheck = true
			break
		}
	}
	assert.True(t, foundLogsCheck, "Should suggest checking init container logs")
}

func TestAnalyzePod_InitContainerCrashLoop(t *testing.T) {
	engine := NewEngine()

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "default",
		},
		Spec: corev1.PodSpec{
			InitContainers: []corev1.Container{
				{
					Name:  "init-setup",
					Image: "alpine:latest",
				},
			},
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: "myapp:latest",
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "init-setup",
					Ready: false,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "Back-off restarting failed container",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 127,
							Reason:   "Error",
						},
					},
					RestartCount: 8,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{},
		},
	}

	ctx := PodContext{
		Pod:    pod,
		Events: []state.EventInfo{},
	}

	diagnostics := engine.AnalyzePod(ctx)

	// Find init crashloop diagnostic
	var initDiag *Diagnostic
	for i := range diagnostics {
		if diagnostics[i].ID == "init.crashloop" {
			initDiag = &diagnostics[i]
			break
		}
	}

	require.NotNil(t, initDiag, "Should detect init container crashloop")
	assert.Equal(t, SeverityError, initDiag.Severity)
	assert.Contains(t, initDiag.Title, "CrashLoopBackOff")
	assert.Contains(t, initDiag.Title, "init-setup")
	assert.Contains(t, initDiag.Description, "Restart count: 8")
	assert.Contains(t, initDiag.Description, "Exit code: 127")
	assert.NotEmpty(t, initDiag.SuggestedCommands)

	// Should suggest checking previous logs
	foundPreviousLogs := false
	for _, cmd := range initDiag.SuggestedCommands {
		if strings.Contains(cmd, "--previous") && strings.Contains(cmd, "init-setup") {
			foundPreviousLogs = true
			break
		}
	}
	assert.True(t, foundPreviousLogs, "Should suggest checking previous init container logs")
}
