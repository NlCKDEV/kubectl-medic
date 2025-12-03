package analysis

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"

	"github.com/NlCKDEV/kubectl-medic/internal/state"
)

// Severity represents the severity level of a diagnostic finding
type Severity string

const (
	SeverityInfo    Severity = "info"
	SeverityWarning Severity = "warning"
	SeverityError   Severity = "error"
)

// Diagnostic represents a single diagnostic finding about a pod
type Diagnostic struct {
	ID                string   // Unique identifier (e.g., "crashloop_backoff")
	Severity          Severity // Severity level
	Title             string   // Short summary
	Description       string   // Human-readable explanation
	SuggestedCommands []string // kubectl commands user can run
}

// PodContext contains all information needed to analyze a pod
type PodContext struct {
	Pod    *corev1.Pod       // Full pod object
	Events []state.EventInfo // Related events
}

// Engine runs diagnostic checks on pods
// This is the main diagnostics engine for kubectl-medic
type Engine struct {
	// Future: Add configuration options for diagnostic rules
}

// NewEngine creates a new diagnostics engine
func NewEngine() *Engine {
	return &Engine{}
}

// AnalyzePod performs comprehensive diagnostics on a pod
// Returns a list of diagnostic findings ordered by severity (errors first)
func (e *Engine) AnalyzePod(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Run all diagnostic checks
	diags = append(diags, analyzeCrashLoops(ctx)...)
	diags = append(diags, analyzeImagePulls(ctx)...)
	diags = append(diags, analyzeScheduling(ctx)...)
	diags = append(diags, analyzeProbes(ctx)...)
	diags = append(diags, analyzeResources(ctx)...)
	diags = append(diags, analyzeStorageIssues(ctx)...)
	diags = append(diags, analyzeConfigIssues(ctx)...)
	diags = append(diags, analyzeInitContainers(ctx)...)

	// If no issues found, report healthy status
	if len(diags) == 0 || allInfoSeverity(diags) {
		diags = append(diags, healthyDiagnostic(ctx))
	}

	return diags
}

// analyzeCrashLoops detects CrashLoopBackOff and restart storms
func analyzeCrashLoops(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	for _, containerStatus := range ctx.Pod.Status.ContainerStatuses {
		// Check for CrashLoopBackOff state
		if containerStatus.State.Waiting != nil &&
			containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {

			desc := fmt.Sprintf("Container '%s' is in CrashLoopBackOff state.\n", containerStatus.Name)
			desc += fmt.Sprintf("Restart count: %d\n\n", containerStatus.RestartCount)

			// Add termination details if available
			if containerStatus.LastTerminationState.Terminated != nil {
				term := containerStatus.LastTerminationState.Terminated
				desc += fmt.Sprintf("Last termination:\n")
				desc += fmt.Sprintf("  - Exit code: %d\n", term.ExitCode)
				if term.Reason != "" {
					desc += fmt.Sprintf("  - Reason: %s\n", term.Reason)
				}
				if term.Message != "" {
					desc += fmt.Sprintf("  - Message: %s\n", term.Message)
				}
			}

			desc += "\nThis usually indicates the container is starting but then immediately crashing."

			diags = append(diags, Diagnostic{
				ID:          "crashloop_backoff",
				Severity:    SeverityError,
				Title:       fmt.Sprintf("Container '%s' in CrashLoopBackOff", containerStatus.Name),
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					fmt.Sprintf("kubectl logs %s -n %s -c %s", ctx.Pod.Name, ctx.Pod.Namespace, containerStatus.Name),
					fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", ctx.Pod.Name, ctx.Pod.Namespace, containerStatus.Name),
				},
			})
		}

		// Check for high restart count even if not currently in CrashLoopBackOff
		if containerStatus.RestartCount > 5 && containerStatus.State.Waiting == nil {
			desc := fmt.Sprintf("Container '%s' has restarted %d times.\n", containerStatus.Name, containerStatus.RestartCount)
			desc += "\nFrequent restarts may indicate:\n"
			desc += "  - Application errors or crashes\n"
			desc += "  - Resource constraints (OOM)\n"
			desc += "  - Failed health checks\n"
			desc += "  - External dependency issues"

			diags = append(diags, Diagnostic{
				ID:          "high_restart_count",
				Severity:    SeverityWarning,
				Title:       fmt.Sprintf("Container '%s' has high restart count", containerStatus.Name),
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					fmt.Sprintf("kubectl logs %s -n %s -c %s", ctx.Pod.Name, ctx.Pod.Namespace, containerStatus.Name),
				},
			})
		}
	}

	return diags
}

// analyzeImagePulls detects image pull errors
func analyzeImagePulls(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	for _, containerStatus := range ctx.Pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil {
			reason := containerStatus.State.Waiting.Reason

			if reason == "ImagePullBackOff" || reason == "ErrImagePull" {
				desc := fmt.Sprintf("Cannot pull image for container '%s'.\n", containerStatus.Name)
				desc += fmt.Sprintf("Image: %s\n\n", getContainerImage(ctx.Pod, containerStatus.Name))

				// Check events for more details
				imagePullEvents := filterEventsByReason(ctx.Events, []string{"Failed", "BackOff", "ErrImagePull"})
				if len(imagePullEvents) > 0 {
					desc += "Recent events:\n"
					for _, event := range imagePullEvents {
						if strings.Contains(event.Message, "pull") || strings.Contains(event.Message, "image") {
							desc += fmt.Sprintf("  - %s\n", event.Message)
						}
					}
					desc += "\n"
				}

				desc += "Common causes:\n"
				desc += "  - Image name or tag is incorrect\n"
				desc += "  - Image does not exist in the registry\n"
				desc += "  - Registry requires authentication\n"
				desc += "  - Network connectivity issues\n"
				desc += "  - ImagePullSecrets are missing or incorrect"

				diags = append(diags, Diagnostic{
					ID:          "image_pull_error",
					Severity:    SeverityError,
					Title:       fmt.Sprintf("Cannot pull image for container '%s'", containerStatus.Name),
					Description: desc,
					SuggestedCommands: []string{
						fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
						fmt.Sprintf("kubectl get secret -n %s", ctx.Pod.Namespace),
						"# Verify the image exists and is accessible",
					},
				})
			}

			if reason == "InvalidImageName" {
				desc := fmt.Sprintf("Image name is invalid for container '%s'.\n", containerStatus.Name)
				desc += fmt.Sprintf("Image: %s\n\n", getContainerImage(ctx.Pod, containerStatus.Name))
				desc += "The image name does not follow Docker image naming conventions."

				diags = append(diags, Diagnostic{
					ID:          "invalid_image_name",
					Severity:    SeverityError,
					Title:       fmt.Sprintf("Invalid image name for container '%s'", containerStatus.Name),
					Description: desc,
					SuggestedCommands: []string{
						fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					},
				})
			}
		}
	}

	return diags
}

// analyzeScheduling detects pod scheduling issues
func analyzeScheduling(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check if pod is pending
	if ctx.Pod.Status.Phase != corev1.PodPending {
		return diags
	}

	// Check PodScheduled condition
	for _, condition := range ctx.Pod.Status.Conditions {
		if condition.Type == corev1.PodScheduled && condition.Status == corev1.ConditionFalse {
			desc := "Pod cannot be scheduled on any node.\n\n"

			// Add condition reason and message
			if condition.Reason != "" {
				desc += fmt.Sprintf("Reason: %s\n", condition.Reason)
			}
			if condition.Message != "" {
				desc += fmt.Sprintf("Message: %s\n\n", condition.Message)
			}

			// Check events for scheduling failures
			schedulingEvents := filterEventsByReason(ctx.Events, []string{"FailedScheduling", "Unschedulable"})
			if len(schedulingEvents) > 0 {
				desc += "Recent scheduling events:\n"
				for _, event := range schedulingEvents {
					desc += fmt.Sprintf("  - %s\n", event.Message)
				}
				desc += "\n"
			}

			desc += "Common causes:\n"
			desc += "  - Insufficient CPU or memory on available nodes\n"
			desc += "  - Node selector constraints not met\n"
			desc += "  - Taints and tolerations mismatch\n"
			desc += "  - Node affinity rules not satisfied\n"
			desc += "  - Persistent volume claims cannot be bound"

			diags = append(diags, Diagnostic{
				ID:          "scheduling_failed",
				Severity:    SeverityError,
				Title:       "Pod cannot be scheduled",
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					"kubectl get nodes",
					"kubectl describe nodes",
					fmt.Sprintf("kubectl get pvc -n %s", ctx.Pod.Namespace),
				},
			})

			break
		}
	}

	return diags
}

// analyzeProbes detects readiness and liveness probe issues
func analyzeProbes(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check for unhealthy containers
	for _, containerStatus := range ctx.Pod.Status.ContainerStatuses {
		// Container running but not ready
		if containerStatus.State.Running != nil && !containerStatus.Ready {
			desc := fmt.Sprintf("Container '%s' is running but not ready.\n\n", containerStatus.Name)

			// Check for readiness probe failures in events
			probeEvents := filterEventsByReason(ctx.Events, []string{"Unhealthy", "ProbeError"})
			if len(probeEvents) > 0 {
				desc += "Recent probe failures:\n"
				for _, event := range probeEvents {
					if strings.Contains(event.Message, containerStatus.Name) {
						desc += fmt.Sprintf("  - %s\n", event.Message)
					}
				}
				desc += "\n"
			}

			desc += "Possible causes:\n"
			desc += "  - Readiness probe is failing\n"
			desc += "  - Application is not fully initialized\n"
			desc += "  - Probe endpoint is incorrect\n"
			desc += "  - Probe timeout is too short"

			diags = append(diags, Diagnostic{
				ID:          "readiness_probe_failed",
				Severity:    SeverityWarning,
				Title:       fmt.Sprintf("Container '%s' failing readiness checks", containerStatus.Name),
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					fmt.Sprintf("kubectl logs %s -n %s -c %s", ctx.Pod.Name, ctx.Pod.Namespace, containerStatus.Name),
				},
			})
		}
	}

	return diags
}

// analyzeResources detects resource-related issues
func analyzeResources(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check for OOMKilled containers
	for _, containerStatus := range ctx.Pod.Status.ContainerStatuses {
		if containerStatus.LastTerminationState.Terminated != nil {
			term := containerStatus.LastTerminationState.Terminated

			if term.Reason == "OOMKilled" {
				desc := fmt.Sprintf("Container '%s' was killed due to out-of-memory (OOM).\n\n", containerStatus.Name)
				desc += fmt.Sprintf("Exit code: %d\n", term.ExitCode)
				desc += "\nThe container exceeded its memory limit and was terminated by the system.\n\n"

				// Get container resource limits
				for _, container := range ctx.Pod.Spec.Containers {
					if container.Name == containerStatus.Name {
						if memLimit := container.Resources.Limits.Memory(); memLimit != nil {
							desc += fmt.Sprintf("Memory limit: %s\n", memLimit.String())
						}
						if memRequest := container.Resources.Requests.Memory(); memRequest != nil {
							desc += fmt.Sprintf("Memory request: %s\n", memRequest.String())
						}
						break
					}
				}

				desc += "\nRecommendations:\n"
				desc += "  - Increase memory limits for the container\n"
				desc += "  - Investigate memory leaks in the application\n"
				desc += "  - Optimize application memory usage"

				diags = append(diags, Diagnostic{
					ID:          "oom_killed",
					Severity:    SeverityError,
					Title:       fmt.Sprintf("Container '%s' killed due to OOM", containerStatus.Name),
					Description: desc,
					SuggestedCommands: []string{
						fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
						fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", ctx.Pod.Name, ctx.Pod.Namespace, containerStatus.Name),
						fmt.Sprintf("kubectl top pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
					},
				})
			}
		}
	}

	// Check for resource quota issues in events
	quotaEvents := filterEventsByReason(ctx.Events, []string{"FailedCreate", "ExceededQuota"})
	if len(quotaEvents) > 0 {
		desc := "Pod creation is being blocked by resource quotas.\n\n"
		desc += "Recent events:\n"
		for _, event := range quotaEvents {
			desc += fmt.Sprintf("  - %s\n", event.Message)
		}

		diags = append(diags, Diagnostic{
			ID:          "resource_quota_exceeded",
			Severity:    SeverityError,
			Title:       "Resource quota exceeded",
			Description: desc,
			SuggestedCommands: []string{
				fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				fmt.Sprintf("kubectl describe quota -n %s", ctx.Pod.Namespace),
				fmt.Sprintf("kubectl describe limitrange -n %s", ctx.Pod.Namespace),
			},
		})
	}

	return diags
}

// analyzeStorageIssues detects PVC and volume mounting problems
func analyzeStorageIssues(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check for FailedMount and FailedAttachVolume events
	// Exclude events related to ConfigMaps and Secrets (those are handled separately)
	storageEvents := []state.EventInfo{}
	rawStorageEvents := filterEventsByReason(ctx.Events, []string{
		"FailedMount",
		"FailedAttachVolume",
		"VolumeFailedMount",
		"FailedMapVolume",
	})
	for _, event := range rawStorageEvents {
		// Skip if this is a ConfigMap or Secret issue
		if strings.Contains(event.Message, "configmap") || strings.Contains(event.Message, "secret") {
			continue
		}
		storageEvents = append(storageEvents, event)
	}

	if len(storageEvents) > 0 {
		desc := "Pod is experiencing volume mounting issues.\n\n"
		desc += "Recent storage-related events:\n"

		for _, event := range storageEvents {
			desc += fmt.Sprintf("  - %s: %s\n", event.Reason, event.Message)
		}

		desc += "\nCommon causes:\n"
		desc += "  - Persistent Volume Claim (PVC) not found\n"
		desc += "  - PVC not bound to a Persistent Volume\n"
		desc += "  - Volume plugin issues\n"
		desc += "  - Node cannot access the volume\n"
		desc += "  - Permissions or access mode conflicts"

		diags = append(diags, Diagnostic{
			ID:          "storage.volume.failed_mount",
			Severity:    SeverityError,
			Title:       "Failed to mount volume",
			Description: desc,
			SuggestedCommands: []string{
				fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				fmt.Sprintf("kubectl get pvc -n %s", ctx.Pod.Namespace),
				fmt.Sprintf("kubectl get pv"),
			},
		})
	}

	// Check for PVC not found events
	pvcNotFoundEvents := filterEventsByReason(ctx.Events, []string{
		"not found",
		"NotFound",
	})

	for _, event := range pvcNotFoundEvents {
		// Only process events that mention PVC
		if strings.Contains(event.Message, "persistentvolumeclaim") ||
			strings.Contains(event.Message, "pvc") ||
			strings.Contains(event.Message, "claim") {

			desc := "Persistent Volume Claim referenced by this pod was not found.\n\n"
			desc += fmt.Sprintf("Event details: %s\n\n", event.Message)
			desc += "The pod is trying to mount a PVC that does not exist in this namespace."

			diags = append(diags, Diagnostic{
				ID:          "storage.pvc.not_found",
				Severity:    SeverityError,
				Title:       "Persistent Volume Claim not found",
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl get pvc -n %s", ctx.Pod.Namespace),
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				},
			})
			break // Only report once
		}
	}

	// Check for PVC not bound (pending state)
	pvcPendingEvents := filterEventsByReason(ctx.Events, []string{
		"Pending",
		"Waiting",
		"not bound",
		"unbound",
	})

	for _, event := range pvcPendingEvents {
		if strings.Contains(event.Message, "persistentvolumeclaim") ||
			strings.Contains(event.Message, "pvc") ||
			strings.Contains(event.Message, "waiting for") {

			desc := "Persistent Volume Claim is not bound to a volume.\n\n"
			desc += fmt.Sprintf("Event details: %s\n\n", event.Message)
			desc += "Possible reasons:\n"
			desc += "  - No Persistent Volume available matching the claim\n"
			desc += "  - Storage class does not exist or cannot provision\n"
			desc += "  - Insufficient storage capacity\n"
			desc += "  - Access mode mismatch"

			diags = append(diags, Diagnostic{
				ID:          "storage.pvc.not_bound",
				Severity:    SeverityError,
				Title:       "Persistent Volume Claim not bound",
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl get pvc -n %s", ctx.Pod.Namespace),
					fmt.Sprintf("kubectl describe pvc -n %s", ctx.Pod.Namespace),
					fmt.Sprintf("kubectl get pv"),
					fmt.Sprintf("kubectl get storageclass"),
				},
			})
			break // Only report once
		}
	}

	return diags
}

// analyzeConfigIssues detects missing ConfigMaps and Secrets
func analyzeConfigIssues(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check for missing ConfigMap events
	for _, event := range ctx.Events {
		if strings.Contains(event.Message, "configmap") &&
			(strings.Contains(strings.ToLower(event.Message), "not found") ||
				strings.Contains(event.Reason, "FailedMount")) {

			desc := "ConfigMap referenced by this pod was not found.\n\n"
			desc += fmt.Sprintf("Event details: %s\n\n", event.Message)
			desc += "The pod is trying to use a ConfigMap that does not exist in this namespace.\n\n"
			desc += "Common causes:\n"
			desc += "  - ConfigMap was deleted\n"
			desc += "  - ConfigMap name is misspelled in pod spec\n"
			desc += "  - ConfigMap not created yet\n"
			desc += "  - Wrong namespace"

			diags = append(diags, Diagnostic{
				ID:          "config.configmap.missing",
				Severity:    SeverityError,
				Title:       "ConfigMap not found",
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl get configmap -n %s", ctx.Pod.Namespace),
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				},
			})
			break // Only report once
		}
	}

	// Check for missing Secret events
	for _, event := range ctx.Events {
		if strings.Contains(event.Message, "secret") &&
			(strings.Contains(strings.ToLower(event.Message), "not found") ||
				strings.Contains(event.Reason, "FailedMount")) {

			desc := "Secret referenced by this pod was not found.\n\n"
			desc += fmt.Sprintf("Event details: %s\n\n", event.Message)
			desc += "The pod is trying to use a Secret that does not exist in this namespace.\n\n"
			desc += "Common causes:\n"
			desc += "  - Secret was deleted\n"
			desc += "  - Secret name is misspelled in pod spec\n"
			desc += "  - Secret not created yet\n"
			desc += "  - Wrong namespace\n"
			desc += "  - Insufficient RBAC permissions"

			diags = append(diags, Diagnostic{
				ID:          "config.secret.missing",
				Severity:    SeverityError,
				Title:       "Secret not found",
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl get secret -n %s", ctx.Pod.Namespace),
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				},
			})
			break // Only report once
		}
	}

	return diags
}

// analyzeInitContainers detects init container failures
func analyzeInitContainers(ctx PodContext) []Diagnostic {
	var diags []Diagnostic

	// Check init container statuses
	for _, initStatus := range ctx.Pod.Status.InitContainerStatuses {
		// Check for crashed/failed init containers
		if initStatus.State.Terminated != nil && initStatus.State.Terminated.ExitCode != 0 {
			term := initStatus.State.Terminated

			desc := fmt.Sprintf("Init container '%s' failed with non-zero exit code.\n\n", initStatus.Name)
			desc += fmt.Sprintf("Exit code: %d\n", term.ExitCode)

			if term.Reason != "" {
				desc += fmt.Sprintf("Reason: %s\n", term.Reason)
			}
			if term.Message != "" {
				desc += fmt.Sprintf("Message: %s\n", term.Message)
			}

			desc += "\nInit containers must complete successfully before the main containers can start.\n"
			desc += "This failure is blocking the pod from running."

			diags = append(diags, Diagnostic{
				ID:          "init.failed",
				Severity:    SeverityError,
				Title:       fmt.Sprintf("Init container '%s' failed", initStatus.Name),
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl logs %s -n %s -c %s", ctx.Pod.Name, ctx.Pod.Namespace, initStatus.Name),
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				},
			})
		}

		// Check for init containers in CrashLoopBackOff
		if initStatus.State.Waiting != nil && initStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			desc := fmt.Sprintf("Init container '%s' is in CrashLoopBackOff.\n\n", initStatus.Name)
			desc += fmt.Sprintf("Restart count: %d\n\n", initStatus.RestartCount)

			if initStatus.LastTerminationState.Terminated != nil {
				term := initStatus.LastTerminationState.Terminated
				desc += fmt.Sprintf("Last termination:\n")
				desc += fmt.Sprintf("  - Exit code: %d\n", term.ExitCode)
				if term.Reason != "" {
					desc += fmt.Sprintf("  - Reason: %s\n", term.Reason)
				}
			}

			desc += "\nThe init container keeps crashing. Init containers must complete successfully\n"
			desc += "before the main application containers can start."

			diags = append(diags, Diagnostic{
				ID:          "init.crashloop",
				Severity:    SeverityError,
				Title:       fmt.Sprintf("Init container '%s' in CrashLoopBackOff", initStatus.Name),
				Description: desc,
				SuggestedCommands: []string{
					fmt.Sprintf("kubectl logs %s -n %s -c %s", ctx.Pod.Name, ctx.Pod.Namespace, initStatus.Name),
					fmt.Sprintf("kubectl logs %s -n %s -c %s --previous", ctx.Pod.Name, ctx.Pod.Namespace, initStatus.Name),
					fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
				},
			})
		}
	}

	return diags
}

// healthyDiagnostic returns an info-level diagnostic for healthy pods
func healthyDiagnostic(ctx PodContext) Diagnostic {
	desc := "Pod appears to be healthy with no major issues detected.\n\n"

	// Add some basic stats
	readyContainers := 0
	totalContainers := len(ctx.Pod.Spec.Containers)
	for _, status := range ctx.Pod.Status.ContainerStatuses {
		if status.Ready {
			readyContainers++
		}
	}

	desc += fmt.Sprintf("Status: %s\n", ctx.Pod.Status.Phase)
	desc += fmt.Sprintf("Containers: %d/%d ready\n", readyContainers, totalContainers)

	// Check for restarts
	totalRestarts := 0
	for _, status := range ctx.Pod.Status.ContainerStatuses {
		totalRestarts += int(status.RestartCount)
	}

	if totalRestarts > 0 {
		desc += fmt.Sprintf("Total restarts: %d (may indicate past issues)\n", totalRestarts)
	}

	return Diagnostic{
		ID:          "healthy",
		Severity:    SeverityInfo,
		Title:       "Pod is healthy",
		Description: desc,
		SuggestedCommands: []string{
			fmt.Sprintf("kubectl describe pod %s -n %s", ctx.Pod.Name, ctx.Pod.Namespace),
		},
	}
}

// Helper functions

// getContainerImage returns the image name for a given container
func getContainerImage(pod *corev1.Pod, containerName string) string {
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			return container.Image
		}
	}
	return "<unknown>"
}

// filterEventsByReason filters events by matching any of the given reasons
func filterEventsByReason(events []state.EventInfo, reasons []string) []state.EventInfo {
	var filtered []state.EventInfo
	for _, event := range events {
		for _, reason := range reasons {
			if strings.Contains(event.Reason, reason) || strings.Contains(event.Message, reason) {
				filtered = append(filtered, event)
				break
			}
		}
	}
	return filtered
}

// allInfoSeverity checks if all diagnostics are info-level
func allInfoSeverity(diags []Diagnostic) bool {
	for _, diag := range diags {
		if diag.Severity != SeverityInfo {
			return false
		}
	}
	return true
}

// SummarizeNamespace analyzes all pods in a namespace and produces a health summary
// This is used for the namespace health view (UC5)
func SummarizeNamespace(namespace string, pods []*corev1.Pod, eventsByPod map[string][]state.EventInfo) state.NamespaceHealth {
	health := state.NamespaceHealth{
		Namespace: namespace,
		TotalPods: len(pods),
	}

	engine := NewEngine()

	// Track which pods have been counted for which issues (avoid double-counting)
	crashLoopPods := make(map[string]bool)
	imagePullPods := make(map[string]bool)
	schedulingPods := make(map[string]bool)
	probePods := make(map[string]bool)
	resourcePods := make(map[string]bool)
	storagePods := make(map[string]bool)
	configPods := make(map[string]bool)
	initPods := make(map[string]bool)
	failingPods := make(map[string]bool)

	for _, pod := range pods {
		// Get events for this pod
		events := eventsByPod[pod.Name]

		// Run diagnostics
		diagnostics := engine.AnalyzePod(PodContext{
			Pod:    pod,
			Events: events,
		})

		// Check if pod is failing (not Running and Ready)
		podFailing := false
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			podFailing = true
		} else {
			// Check if all containers are ready
			for _, status := range pod.Status.ContainerStatuses {
				if !status.Ready {
					podFailing = true
					break
				}
			}
		}

		if podFailing {
			failingPods[pod.Name] = true
		}

		// Analyze diagnostic results to categorize issues
		for _, diag := range diagnostics {
			// Skip info-level diagnostics (healthy pods)
			if diag.Severity == SeverityInfo {
				continue
			}

			// Categorize by diagnostic ID
			if strings.Contains(diag.ID, "crashloop") || strings.Contains(diag.ID, "high_restarts") {
				crashLoopPods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "image_pull") || strings.Contains(diag.ID, "invalid_image") {
				imagePullPods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "scheduling") || strings.Contains(diag.ID, "unschedulable") {
				schedulingPods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "probe") || strings.Contains(diag.ID, "readiness") {
				probePods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "oom") || strings.Contains(diag.ID, "resource") || strings.Contains(diag.ID, "quota") {
				resourcePods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "storage.") {
				storagePods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "config.") {
				configPods[pod.Name] = true
			}
			if strings.Contains(diag.ID, "init.") {
				initPods[pod.Name] = true
			}
		}
	}

	// Populate counts
	health.FailingPods = len(failingPods)
	health.CrashLoopPods = len(crashLoopPods)
	health.ImagePullPods = len(imagePullPods)
	health.SchedulingPods = len(schedulingPods)
	health.ProbeIssuePods = len(probePods)
	health.ResourceIssuePods = len(resourcePods)
	health.StorageIssuePods = len(storagePods)
	health.ConfigIssuePods = len(configPods)
	health.InitFailurePods = len(initPods)

	return health
}
