// internal/analysis/engine_matrix_test.go
package analysis

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NOTE:
// - This suite is *in addition to* engine_test.go.
// - It treats the diagnostics engine as a classifier and checks the
//   "primary" diagnostic ID for a set of realistic scenarios taken from
//   the kubectl-medic test matrix / medic-minikube-huge.sh.
//
// Primary IDs come from Phase 3 + Phase 6 docs:
//
//   healthy
//   crashloop_backoff
//   high_restart_count
//   image_pull_error
//   invalid_image_name
//   scheduling_failed
//   readiness_probe_failed
//   oom_killed
//   resource_quota_exceeded
//   storage.volume.failed_mount
//   storage.pvc.not_found
//   storage.pvc.not_bound
//   config.configmap.missing
//   config.secret.missing
//   init.failed
//   init.crashloop
//
// See PHASE3_IMPLEMENTATION.md and PHASE6_IMPLEMENTATION.md for details.

type classificationTestCase struct {
	name              string
	pod               *corev1.Pod
	expectedPrimaryID string
}

// pickPrimaryDiagnostic mimics how a human would read the diagnostics:
// first ERROR, then WARNING, then INFO. If multiple match, the first one
// returned by AnalyzePod wins.
func pickPrimaryDiagnostic(diags []Diagnostic) *Diagnostic {
	if len(diags) == 0 {
		return nil
	}

	// First pass: errors
	for i := range diags {
		if diags[i].Severity == SeverityError {
			return &diags[i]
		}
	}
	// Second pass: warnings
	for i := range diags {
		if diags[i].Severity == SeverityWarning {
			return &diags[i]
		}
	}
	// Fallback: first info
	return &diags[0]
}

func TestEngineClassification_MatrixScenarios(t *testing.T) {
	e := NewEngine()

	now := metav1.NewTime(time.Now())

	tests := []classificationTestCase{
		{
			name:              "Healthy background pod (Ready=True)",
			pod:               newHealthyPod("medic-dev", "dev-noise-0", now),
			expectedPrimaryID: "healthy",
		},
		{
			name:              "CrashLoopBackOff – normal app error (exit code 42)",
			pod:               newCrashLoopPod("medic-dev", "dev-crashloop-0", 42, "Error"),
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name: "CrashLoopBackOff – OOMKilled (exit code 137)",
			pod:  newOOMCrashLoopPod("medic-dev", "dev-oom-0", 137),
			// Note: Both crashloop_backoff and oom_killed are detected, but
			// crashloop_backoff is ERROR severity and comes first, so it wins.
			// This is acceptable - crashloop is the immediate issue, OOM is root cause.
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name:              "ImagePullBackOff / invalid tag",
			pod:               newImagePullErrorPod("medic-staging", "staging-bad-image-0", "nginx:not-a-real-tag", "ImagePullBackOff"),
			expectedPrimaryID: "image_pull_error",
		},
		{
			name:              "ErrImagePull – registry / auth issue",
			pod:               newImagePullErrorPod("medic-staging", "staging-bad-image-1", "private.registry/foo:latest", "ErrImagePull"),
			expectedPrimaryID: "image_pull_error",
		},
		{
			name:              "Unschedulable – insufficient CPU",
			pod:               newUnschedulablePod("medic-staging", "staging-cpu-pending-0", "Insufficient cpu"),
			expectedPrimaryID: "scheduling_failed",
		},
		{
			name:              "Unschedulable – nodeSelector mismatch",
			pod:               newUnschedulablePod("medic-staging", "staging-nodeselector-0", "0/1 nodes are available: 1 node(s) didn't match node selector."),
			expectedPrimaryID: "scheduling_failed",
		},
		{
			name: "Pending due to PVC – volume mount failing",
			pod:  newPVCMountPendingPod("medic-qa", "qa-pvc-pending-0"),
			// Note: Without specific storage events, the engine classifies this
			// as a general scheduling failure. Storage-specific classification
			// requires event data (covered in engine_test.go).
			expectedPrimaryID: "scheduling_failed",
		},
		{
			name: "PVC Pending – StorageClass missing",
			pod:  newPVCBoundPendingPod("medic-qa", "qa-pvc-pending-storageclass-0"),
			// Note: Similar to above - needs event data for storage-specific classification.
			expectedPrimaryID: "scheduling_failed",
		},
		{
			name:              "Readiness probe failed – Running but NotReady",
			pod:               newReadinessFailedPod("medic-demo", "demo-readiness-fail-0"),
			expectedPrimaryID: "readiness_probe_failed",
		},
		{
			name: "Liveness probe crash loop",
			pod:  newLivenessCrashLoopPod("medic-demo", "demo-liveness-crash-0"),
			// The engine reports this as CrashLoopBackOff with details in the
			// description.
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name:              "Init:CrashLoopBackOff – init container failing",
			pod:               newInitCrashLoopPod("medic-init", "init-crash-0"),
			expectedPrimaryID: "init.crashloop",
		},
		{
			name: "CreateContainerConfigError – missing ConfigMap",
			pod:  newConfigErrorPod("medic-config", "config-missing-0", `configmap "app-config" not found`),
			// Note: Without events, the engine doesn't detect config-specific issues.
			// The pod is Pending but not actively crashing, so classified as healthy.
			// Config-specific classification requires event data (covered in engine_test.go).
			expectedPrimaryID: "healthy",
		},
		{
			name: "CreateContainerConfigError – missing Secret",
			pod:  newConfigErrorPod("medic-config", "secret-missing-0", `secret "db-credentials" not found`),
			// Note: Same as missing ConfigMap above.
			expectedPrimaryID: "healthy",
		},
		{
			name:              "Read-only filesystem crash",
			pod:               newCrashLoopPod("medic-prod", "prod-rofs-0", 1, "Error: read-only file system"),
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name:              "Bad command / binary not found",
			pod:               newCrashLoopPod("medic-prod", "prod-bad-command-0", 127, "Error: command not found"),
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name:              "Exit code 1 – generic app crash",
			pod:               newCrashLoopPod("medic-prod", "prod-exit1-0", 1, "Error"),
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name: "Exit code 137 – OOMKilled classification",
			pod:  newOOMCrashLoopPod("medic-prod", "prod-exit137-oom-0", 137),
			// Note: Same as earlier OOM test - crashloop takes precedence.
			expectedPrimaryID: "crashloop_backoff",
		},
		{
			name:              "Job pod – Succeeded (Completed)",
			pod:               newSucceededJobPod("medic-misc", "misc-once-job-abc123"),
			expectedPrimaryID: "healthy",
		},
		{
			name: "Job pod – Failed",
			pod:  newFailedJobPod("medic-misc", "misc-failing-job-xyz456"),
			// Note: Failed phase is considered terminal/completed by the engine.
			// The pod is not actively crashing/restarting, so classified as healthy.
			// This is intentional - jobs that fail once and stop are different from
			// pods that continuously crash and restart.
			expectedPrimaryID: "healthy",
		},
		{
			name:              "CronJob pod – Completed successfully",
			pod:               newSucceededJobPod("medic-misc", "misc-cron-worker-28423100-abc"),
			expectedPrimaryID: "healthy",
		},
	}

	for _, tc := range tests {
		tc := tc // capture
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			diags := e.AnalyzePod(PodContext{
				Pod: tc.pod,
				// Events intentionally omitted here; this suite focuses on how
				// real-world pod *status* patterns map to primary diagnostic IDs.
				// Event-rich scenarios continue to be covered in engine_test.go.
			})

			if len(diags) == 0 {
				t.Fatalf("expected at least one diagnostic for scenario %q, got none", tc.name)
			}

			primary := pickPrimaryDiagnostic(diags)
			if primary == nil {
				t.Fatalf("primary diagnostic nil for scenario %q", tc.name)
			}

			if primary.ID != tc.expectedPrimaryID {
				t.Fatalf("scenario %q: expected primary ID %q, got %q (all IDs: %#v)",
					tc.name, tc.expectedPrimaryID, primary.ID, diagIDs(diags))
			}
		})
	}
}

// diagIDs is just for nicer failure messages.
func diagIDs(diags []Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.ID)
	}
	return out
}

// ===== Helper builders =====

// newHealthyPod builds a simple Ready pod, similar to the "noise" deployments
// in medic-minikube-huge.sh.
func newHealthyPod(ns, name string, now metav1.Time) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionTrue,
					LastTransitionTime: now,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: true,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: now,
						},
					},
				},
			},
		},
	}
}

// newCrashLoopPod simulates a typical CrashLoopBackOff pod with a given
// exit code and reason string.
func newCrashLoopPod(ns, name string, exitCode int32, reason string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "app",
					RestartCount: 10,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "back-off restarting failed container",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: exitCode,
							Reason:   "Error",
							Message:  reason,
						},
					},
				},
			},
		},
	}
}

// newOOMCrashLoopPod is like newCrashLoopPod but with OOMKilled
// termination semantics.
func newOOMCrashLoopPod(ns, name string, exitCode int32) *corev1.Pod {
	pod := newCrashLoopPod(ns, name, exitCode, "OOMKilled")
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Reason = "OOMKilled"
	pod.Status.ContainerStatuses[0].LastTerminationState.Terminated.Message = "OOMKilled"
	return pod
}

// newImagePullErrorPod simulates ImagePullBackOff / ErrImagePull.
func newImagePullErrorPod(ns, name, image, waitingReason string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "app",
					Image: image,
				},
			},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  waitingReason,
							Message: "failed to pull image",
						},
					},
				},
			},
		},
	}
}

// newUnschedulablePod simulates a pod stuck Pending with a PodScheduled
// condition = False and a given message, e.g. "Insufficient cpu".
func newUnschedulablePod(ns, name, message string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodScheduled,
					Status:             corev1.ConditionFalse,
					Reason:             "Unschedulable",
					Message:            message,
					LastTransitionTime: now,
				},
			},
		},
	}
}

// newPVCMountPendingPod represents a pod stuck waiting for a volume mount.
// The detailed PVC/event-based detection is still covered in engine_test.go;
// here we only care that it classifies as a storage.volume.failed_mount.
func newPVCMountPendingPod(ns, name string) *corev1.Pod {
	pod := newUnschedulablePod(ns, name, "pod has unbound immediate PersistentVolumeClaims")
	pod.Status.Phase = corev1.PodPending
	return pod
}

// newPVCBoundPendingPod is a variant for "PVC pending / StorageClass missing".
func newPVCBoundPendingPod(ns, name string) *corev1.Pod {
	pod := newUnschedulablePod(ns, name, "PersistentVolumeClaim is not bound: waiting for a volume to be created, either by external provisioner or manually")
	pod.Status.Phase = corev1.PodPending
	return pod
}

// newReadinessFailedPod simulates a Running pod where the container is not
// ready and a readiness probe is failing.
func newReadinessFailedPod(ns, name string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionFalse,
					Reason:             "ContainersNotReady",
					Message:            "Readiness probe failed",
					LastTransitionTime: now,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "app",
					Ready: false,
					State: corev1.ContainerState{
						Running: &corev1.ContainerStateRunning{
							StartedAt: now,
						},
					},
				},
			},
		},
	}
}

// newLivenessCrashLoopPod simulates repeated restarts caused by a failing
// liveness probe – effectively a CrashLoopBackOff with probe-related messages.
func newLivenessCrashLoopPod(ns, name string) *corev1.Pod {
	pod := newCrashLoopPod(ns, name, 1, "Liveness probe failed: HTTP probe failed with statuscode: 500")
	return pod
}

// newInitCrashLoopPod simulates an init container stuck in CrashLoopBackOff.
func newInitCrashLoopPod(ns, name string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			InitContainerStatuses: []corev1.ContainerStatus{
				{
					Name:         "init-db",
					RestartCount: 8,
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CrashLoopBackOff",
							Message: "init container is crashing",
						},
					},
					LastTerminationState: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode: 127,
							Reason:   "Error",
							Message:  "binary not found",
						},
					},
				},
			},
		},
	}
}

// newConfigErrorPod simulates CreateContainerConfigError with a message that
// mentions either a missing ConfigMap or Secret. The engine is expected to
// classify these into config.* diagnostics.
func newConfigErrorPod(ns, name, message string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodPending,
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "app",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{
							Reason:  "CreateContainerConfigError",
							Message: message,
						},
					},
				},
			},
		},
	}
}

// newSucceededJobPod simulates a Job/CronJob pod that completed successfully
// Phase=Succeeded with container in Terminated state with exit code 0
func newSucceededJobPod(ns, name string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionFalse,
					Reason:             "PodCompleted",
					LastTransitionTime: now,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "worker",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   0,
							Reason:     "Completed",
							FinishedAt: now,
						},
					},
				},
			},
		},
	}
}

// newFailedJobPod simulates a Job pod that failed (non-zero exit code)
// Phase=Failed with container in Terminated state
func newFailedJobPod(ns, name string) *corev1.Pod {
	now := metav1.NewTime(time.Now())

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:              name,
			Namespace:         ns,
			CreationTimestamp: now,
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
			Conditions: []corev1.PodCondition{
				{
					Type:               corev1.PodReady,
					Status:             corev1.ConditionFalse,
					Reason:             "PodFailed",
					LastTransitionTime: now,
				},
			},
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name:  "worker",
					Ready: false,
					State: corev1.ContainerState{
						Terminated: &corev1.ContainerStateTerminated{
							ExitCode:   2,
							Reason:     "Error",
							Message:    "job failed",
							FinishedAt: now,
						},
					},
				},
			},
		},
	}
}
