package details

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/types"
	corev1 "k8s.io/api/core/v1"
)

// TestCopyModeFormatting verifies that copy mode outputs clean, copy-pasteable commands
func TestCopyModeFormatting(t *testing.T) {
	// Create test pod
	testPod := &corev1.Pod{}
	testPod.Name = "test-pod"
	testPod.Namespace = "default"

	// Create test state with diagnostics
	appState := &state.AppState{
		SelectedPod: "test-pod",
		CurrentView: state.ViewCopyCommands,
		CurrentPod:  testPod, // DEPRECATED but needed for renderCopyCommands
		PodDetails: state.Resource[state.PodDetailsData]{
			Data: state.PodDetailsData{
				Pod: testPod,
			},
			Loaded: true,
		},
		Diagnostics: []types.Diagnostic{
			{
				ID:       "test-1",
				Severity: types.SeverityError,
				Title:    "CrashLoopBackOff",
				SuggestedCommands: []string{
					"kubectl logs test-pod -n default",
					"kubectl describe pod test-pod -n default",
					"# This is a comment that should be skipped",
					"kubectl get events -n default",
				},
			},
			{
				ID:       "test-2",
				Severity: types.SeverityWarning,
				Title:    "High Restarts",
				SuggestedCommands: []string{
					"kubectl logs test-pod -n default --previous",
				},
			},
		},
	}

	m := NewModel(appState)
	m.SetSize(100, 30)
	m.ready = true

	output := m.renderCopyCommands()

	// Test 1: Commands should NOT have "$ " prefix
	if strings.Contains(output, "$ kubectl logs") {
		t.Error("Copy mode should not include '$ ' prefix in commands")
	}

	// Test 2: Commands should be present (without prefix)
	if !strings.Contains(output, "kubectl logs test-pod -n default") {
		t.Error("Copy mode should contain the actual command")
	}

	// Test 3: Comment lines should be skipped (not rendered as commands)
	// The title comment "# CrashLoopBackOff" should be present
	// But inline comment lines like "# This is a comment..." should be skipped
	commentCount := strings.Count(output, "# This is a comment that should be skipped")
	if commentCount > 0 {
		t.Error("Copy mode should skip comment lines from SuggestedCommands")
	}

	// Test 4: Diagnostic titles should be present as context
	if !strings.Contains(output, "# CrashLoopBackOff") {
		t.Error("Copy mode should include diagnostic titles as comments")
	}

	// Test 5: All non-comment commands should be present
	expectedCommands := []string{
		"kubectl logs test-pod -n default",
		"kubectl describe pod test-pod -n default",
		"kubectl get events -n default",
		"kubectl logs test-pod -n default --previous",
	}

	for _, cmd := range expectedCommands {
		if !strings.Contains(output, cmd) {
			t.Errorf("Copy mode should contain command: %s", cmd)
		}
	}

	// Test 6: Commands should not be truncated (no "…" markers)
	if strings.Contains(output, "…") {
		t.Error("Copy mode should not truncate commands")
	}
}

// TestCopyModeNoANSICodes verifies that commands have no embedded ANSI codes
func TestCopyModeNoANSICodes(t *testing.T) {
	// Create test pod
	testPod := &corev1.Pod{}
	testPod.Name = "test-pod"
	testPod.Namespace = "default"

	appState := &state.AppState{
		SelectedPod: "test-pod",
		CurrentView: state.ViewCopyCommands,
		CurrentPod:  testPod, // DEPRECATED but needed for renderCopyCommands
		PodDetails: state.Resource[state.PodDetailsData]{
			Data: state.PodDetailsData{
				Pod: testPod,
			},
			Loaded: true,
		},
		Diagnostics: []types.Diagnostic{
			{
				ID:       "test-1",
				Severity: types.SeverityError,
				Title:    "Test Diagnostic",
				SuggestedCommands: []string{
					"kubectl logs test-pod",
				},
			},
		},
	}

	m := NewModel(appState)
	m.SetSize(100, 30)
	m.ready = true

	output := m.renderCopyCommands()

	// Extract just the command lines (not including diagnostic titles or help text)
	// Look for the actual command we expect
	lines := strings.Split(output, "\n")
	var commandLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// Skip empty lines, comment lines, and help text
		if trimmed == "" || strings.HasPrefix(trimmed, "#") ||
			strings.Contains(trimmed, "Commands ready to copy") ||
			strings.Contains(trimmed, "scroll") {
			continue
		}
		// This should be a command line
		if strings.Contains(trimmed, "kubectl") {
			commandLines = append(commandLines, trimmed)
		}
	}

	// Check that command lines don't have ANSI escape codes
	for _, line := range commandLines {
		if strings.Contains(line, "\x1b[") {
			t.Errorf("Command line contains ANSI escape codes: %q", line)
		}
	}

	// Verify we found the expected command
	if len(commandLines) == 0 {
		t.Error("No kubectl command lines found in copy mode output")
	}

	found := false
	for _, line := range commandLines {
		if strings.Contains(line, "kubectl logs test-pod") {
			found = true
			// Verify no styling artifacts
			if strings.Contains(line, "$") {
				t.Error("Command line should not have $ prefix in copy mode")
			}
			break
		}
	}

	if !found {
		t.Error("Expected command 'kubectl logs test-pod' not found in output")
	}
}

// TestViewSwitchingKeybindings verifies keybindings for switching views in details pane
func TestViewSwitchingKeybindings(t *testing.T) {
	// Create test pod
	testPod := &corev1.Pod{}
	testPod.Name = "test-pod"
	testPod.Namespace = "default"

	tests := []struct {
		name        string
		initialView state.ViewMode
		key         string
		expectedView state.ViewMode
	}{
		{
			name:         "c key in Diagnostics -> CopyCommands",
			initialView:  state.ViewDiagnostics,
			key:          "c",
			expectedView: state.ViewCopyCommands,
		},
		{
			name:         "x key in CopyCommands -> Diagnostics",
			initialView:  state.ViewCopyCommands,
			key:          "x",
			expectedView: state.ViewDiagnostics,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appState := &state.AppState{
				SelectedPod: "test-pod",
				CurrentView: tt.initialView,
				CurrentPod:  testPod,
				PodDetails: state.Resource[state.PodDetailsData]{
					Data: state.PodDetailsData{
						Pod: testPod,
					},
					Loaded: true,
				},
				Diagnostics: []types.Diagnostic{
					{
						ID:       "test",
						Severity: types.SeverityError,
						Title:    "Test",
					},
				},
			}

			m := NewModel(appState)
			m.SetSize(100, 30)
			m.ready = true
			m.active = true

			// Simulate key press
			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			m, _ = m.Update(msg)

			if m.state.CurrentView != tt.expectedView {
				t.Errorf("After pressing '%s' from %v, expected view %v, got %v",
					tt.key, tt.initialView, tt.expectedView, m.state.CurrentView)
			}
		})
	}
}

// TestContainerSwitching verifies 'c' key cycles containers in logs view
func TestContainerSwitching(t *testing.T) {
	// Create test pod
	testPod := &corev1.Pod{}
	testPod.Name = "test-pod"
	testPod.Namespace = "default"

	appState := &state.AppState{
		SelectedPod:       "test-pod",
		CurrentView:       state.ViewLogs,
		CurrentPod:        testPod,
		CurrentContainers: []string{"container-a", "container-b", "container-c"},
		SelectedContainer: 0,
	}

	m := NewModel(appState)
	m.SetSize(100, 30)
	m.ready = true
	m.active = true

	// Press 'c' to cycle containers
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}}

	// First press: 0 -> 1
	m, _ = m.Update(msg)
	if m.state.SelectedContainer != 1 {
		t.Errorf("After first 'c', expected container 1, got %d", m.state.SelectedContainer)
	}

	// Second press: 1 -> 2
	m, _ = m.Update(msg)
	if m.state.SelectedContainer != 2 {
		t.Errorf("After second 'c', expected container 2, got %d", m.state.SelectedContainer)
	}

	// Third press: 2 -> 0 (cycle back)
	m, _ = m.Update(msg)
	if m.state.SelectedContainer != 0 {
		t.Errorf("After third 'c', expected container 0 (cycle), got %d", m.state.SelectedContainer)
	}
}

// TestDiagnosticsModeCommandFormatting verifies normal diagnostics view formatting
func TestDiagnosticsModeCommandFormatting(t *testing.T) {
	// Create test pod
	testPod := &corev1.Pod{}
	testPod.Name = "test-pod"
	testPod.Namespace = "default"

	appState := &state.AppState{
		SelectedPod: "test-pod",
		CurrentView: state.ViewDiagnostics,
		CurrentPod:  testPod, // DEPRECATED but needed for renderDiagnostics
		PodDetails: state.Resource[state.PodDetailsData]{
			Data: state.PodDetailsData{
				Pod: testPod,
			},
			Loaded: true,
		},
		Diagnostics: []types.Diagnostic{
			{
				ID:          "test-1",
				Severity:    types.SeverityError,
				Title:       "Test Issue",
				Description: "This is a test issue",
				SuggestedCommands: []string{
					"kubectl logs test-pod",
					"# This is a helpful comment",
					"kubectl describe pod test-pod",
				},
			},
		},
	}

	m := NewModel(appState)
	m.SetSize(100, 30)
	m.ready = true

	output := m.renderDiagnostics()

	// In diagnostics view (not copy mode), commands have formatting
	// The wrapCommand function adds "  $ " prefix
	if !strings.Contains(output, "kubectl logs test-pod") {
		t.Error("Diagnostics view should contain commands")
	}

	// Comments should be rendered in diagnostics view
	if !strings.Contains(output, "This is a helpful comment") {
		t.Error("Diagnostics view should render comment lines")
	}

	// Verify help text mentions copy mode
	if !strings.Contains(output, "copy") {
		t.Error("Diagnostics view help text should mention copy mode")
	}
}
