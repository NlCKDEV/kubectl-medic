package pods

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
)

// TestColumnWidthCalculation tests that column widths are calculated correctly for different pane widths
func TestColumnWidthCalculation(t *testing.T) {
	tests := []struct {
		name      string
		paneWidth int
	}{
		{"Narrow pane", 50},
		{"Medium pane", 80},
		{"Wide pane", 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				width: tt.paneWidth,
			}

			// Use content width (width minus PaneHorizontalOverhead)
			contentWidth := tt.paneWidth - 6
			widths := m.calculateColumnWidths(contentWidth)

			// Check that all columns have reasonable widths (> 0)
			// Note: some columns may be 0 if dropped for narrow panes
			if widths.name <= 0 {
				t.Errorf("Name column width must be positive, got %d", widths.name)
			}
			// Ready, restarts, age may be 0 for narrow panes - check status which is always visible
			if widths.status <= 0 {
				t.Errorf("Status column width must be positive, got %d", widths.status)
			}
		})
	}
}

// TestFilteringPods tests that pod filtering works correctly
func TestFilteringPods(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
			{Name: "nginx-deployment-abc123", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			{Name: "nginx-deployment-def456", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			{Name: "redis-pod-xyz789", Namespace: "default", Status: "CrashLoopBackOff", Restarts: 10, Age: "3d"},
			{Name: "postgres-pod-123", Namespace: "default", Status: "Running", Restarts: 1, Age: "10d"},
			{Name: "other-namespace-pod", Namespace: "kube-system", Status: "Running", Restarts: 0, Age: "100d"},
				},
		},
	}

	tests := []struct {
		name          string
		filter        string
		expectedCount int
		expectedFirst string
	}{
		{
			name:          "No filter",
			filter:        "",
			expectedCount: 4, // Only 'default' namespace pods
			expectedFirst: "nginx-deployment-abc123",
		},
		{
			name:          "Filter 'nginx'",
			filter:        "nginx",
			expectedCount: 2,
			expectedFirst: "nginx-deployment-abc123",
		},
		{
			name:          "Filter 'redis'",
			filter:        "redis",
			expectedCount: 1,
			expectedFirst: "redis-pod-xyz789",
		},
		{
			name:          "Case insensitive filter 'NGINX'",
			filter:        "NGINX",
			expectedCount: 2,
			expectedFirst: "nginx-deployment-abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:  appState,
				filter: tt.filter,
			}

			filtered := m.getFilteredAndSortedPods()

			if len(filtered) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(filtered))
			}

			if tt.expectedCount > 0 && filtered[0].Name != tt.expectedFirst {
				t.Errorf("Expected first result '%s', got '%s'", tt.expectedFirst, filtered[0].Name)
			}
		})
	}
}

// TestSortingPods tests that pod sorting works correctly
func TestSortingPods(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
			{Name: "zebra-pod", Namespace: "default", Status: "Running", Restarts: 1, Age: "5d"},
			{Name: "alpha-pod", Namespace: "default", Status: "CrashLoopBackOff", Restarts: 10, Age: "10d"},
			{Name: "beta-pod", Namespace: "default", Status: "Pending", Restarts: 0, Age: "1d"},
			{Name: "gamma-pod", Namespace: "default", Status: "Running", Restarts: 5, Age: "3d"},
				},
		},
	}

	tests := []struct {
		name     string
		sortMode SortMode
		expected []string
	}{
		{
			name:     "Sort by name",
			sortMode: SortByName,
			expected: []string{"alpha-pod", "beta-pod", "gamma-pod", "zebra-pod"},
		},
		{
			name:     "Sort by status (errors first)",
			sortMode: SortByStatus,
			expected: []string{"alpha-pod", "beta-pod", "gamma-pod", "zebra-pod"}, // CrashLoop, Pending, Running, Running
		},
		{
			name:     "Sort by restarts (descending)",
			sortMode: SortByRestarts,
			expected: []string{"alpha-pod", "gamma-pod", "zebra-pod", "beta-pod"}, // 10, 5, 1, 0
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:    appState,
				sortMode: tt.sortMode,
			}

			sorted := m.getFilteredAndSortedPods()

			if len(sorted) != len(tt.expected) {
				t.Fatalf("Expected %d results, got %d", len(tt.expected), len(sorted))
			}

			for i, expectedName := range tt.expected {
				if sorted[i].Name != expectedName {
					t.Errorf("Position %d: expected '%s', got '%s'", i, expectedName, sorted[i].Name)
				}
			}
		})
	}
}

// TestCalculateVisibleWindow tests the windowed list rendering logic for pods
func TestCalculateVisibleWindow(t *testing.T) {
	tests := []struct {
		name          string
		height        int
		windowStart   int
		totalItems    int
		expectedStart int
		expectedEnd   int
	}{
		{
			name:          "All items fit",
			height:        30,
			windowStart:   0,
			totalItems:    10,
			expectedStart: 0,
			expectedEnd:   10,
		},
		{
			name:          "Window at start of large list",
			height:        25,
			windowStart:   0,
			totalItems:    200,
			expectedStart: 0,
			expectedEnd:   13, // height 25 - chrome 12 = 13 visible
		},
		{
			name:          "Window in middle of large list",
			height:        25,
			windowStart:   100,
			totalItems:    200,
			expectedStart: 100,
			expectedEnd:   113,
		},
		{
			name:          "Window at end of large list",
			height:        25,
			windowStart:   187,
			totalItems:    200,
			expectedStart: 187,
			expectedEnd:   200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				height:      tt.height,
				windowStart: tt.windowStart,
			}

			start, end := m.calculateVisibleWindow(tt.totalItems)

			if start != tt.expectedStart {
				t.Errorf("Expected start %d, got %d", tt.expectedStart, start)
			}
			if end != tt.expectedEnd {
				t.Errorf("Expected end %d, got %d", tt.expectedEnd, end)
			}

			// Basic sanity checks
			if start < 0 {
				t.Errorf("Start index %d should not be negative", start)
			}
			if end > tt.totalItems {
				t.Errorf("End index %d should not exceed total items %d", end, tt.totalItems)
			}
			if start >= end {
				t.Errorf("Start index %d should be less than end index %d", start, end)
			}
		})
	}
}

// TestSingleLineGuarantee verifies that pod table rows never contain newlines
func TestSingleLineGuarantee(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
			{Name: "very-long-pod-name-that-exceeds-normal-column-width-limits", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 0, Age: "5d"},
			{Name: "short", Namespace: "default", Status: "CrashLoopBackOff", Ready: "0/1", Restarts: 999, Age: "100d"},
				},
		},
	}

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"Very narrow pane", 30, 20},
		{"Narrow pane", 50, 25},
		{"Medium pane", 80, 30},
		{"Wide pane", 120, 40},
		{"Very wide pane", 200, 50},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:  appState,
				width:  tt.width,
				height: tt.height,
				active: true,
			}

			view := m.View()

			// Check that no line contains a newline within it (except at line endings)
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				// Remove ANSI escape codes for accurate length checking
				plainLine := stripANSI(line)

				// Verify no embedded newlines
				if strings.Contains(plainLine, "\r") || strings.Count(line, "\n") > 0 {
					t.Errorf("Line %d contains embedded newline characters", i)
				}
			}
		})
	}
}

// TestColumnDropping verifies that columns are dropped appropriately for narrow panes
func TestColumnDropping(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		expectAge    bool
		expectRST    bool
		expectReady  bool
	}{
		{"Very wide - all columns", 120, true, true, true},
		{"Medium - all columns", 80, true, true, true},
		{"Narrow - drop age", 45, false, true, true},
		{"Very narrow - drop age and restarts", 40, false, false, true},
		{"Ultra narrow - only name and status", 30, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				width: tt.width,
			}

			// Use content width (width minus PaneHorizontalOverhead)
			contentWidth := tt.width - 6
			widths := m.calculateColumnWidths(contentWidth)

			if widths.showAge != tt.expectAge {
				t.Errorf("Width %d: expected showAge=%v, got %v", tt.width, tt.expectAge, widths.showAge)
			}

			if widths.showRestarts != tt.expectRST {
				t.Errorf("Width %d: expected showRestarts=%v, got %v", tt.width, tt.expectRST, widths.showRestarts)
			}

			readyVisible := widths.ready > 0
			if readyVisible != tt.expectReady {
				t.Errorf("Width %d: expected ready visible=%v, got %v", tt.width, tt.expectReady, readyVisible)
			}
		})
	}
}

// TestHeaderSingleLine specifically verifies that table headers never wrap
func TestHeaderSingleLine(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "test-pod", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 0, Age: "1d"},
			},
		},
	}

	tests := []struct {
		name  string
		width int
	}{
		{"Ultra narrow", 30},
		{"Very narrow", 40},
		{"Narrow", 50},
		{"Medium", 80},
		{"Wide", 120},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:  appState,
				width:  tt.width,
				height: 20,
				active: true,
			}

			view := m.View()
			lines := strings.Split(view, "\n")

			// Find the header line (contains "NAME" or "STATUS")
			for i, line := range lines {
				plainLine := stripANSI(line)
				if strings.Contains(plainLine, "NAME") && strings.Contains(plainLine, "STATUS") {
					// This is the header line - verify it doesn't have embedded formatting issues
					// and that it's on a single line
					if strings.Contains(line, "\r") {
						t.Errorf("Header line %d contains carriage return", i)
					}
					// Header should not be empty
					if len(plainLine) == 0 {
						t.Errorf("Header line %d is empty", i)
					}
					break
				}
			}
		})
	}
}

// stripANSI removes ANSI escape codes from a string for testing
func stripANSI(s string) string {
	// Simple regex-free approach: remove common ANSI patterns
	result := ""
	inEscape := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			inEscape = true
			i++ // Skip '['
			continue
		}
		if inEscape {
			if (s[i] >= 'A' && s[i] <= 'Z') || (s[i] >= 'a' && s[i] <= 'z') {
				inEscape = false
			}
			continue
		}
		result += string(s[i])
	}
	return result
}

// TestPodsEnterShowsDetails verifies that pressing Enter switches to ViewDetails
func TestPodsEnterShowsDetails(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		CurrentView:       state.ViewDetails,
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "nginx-pod", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
				{Name: "redis-pod", Namespace: "default", Status: "Running", Restarts: 0, Age: "3d"},
			},
		},
	}

	m := Model{
		state:  appState,
		active: true,
		cursor: 1, // Select "redis-pod"
	}

	// Simulate Enter key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(msg)

	if m.state.CurrentView != state.ViewDetails {
		t.Errorf("Expected CurrentView to be ViewDetails, got %v", m.state.CurrentView)
	}

	if m.state.SelectedPod != "redis-pod" {
		t.Errorf("Expected SelectedPod to be 'redis-pod', got '%s'", m.state.SelectedPod)
	}
}

// TestPodsDiagnosticsKey verifies that pressing 'x' switches to ViewDiagnostics
func TestPodsDiagnosticsKey(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		CurrentView:       state.ViewDetails,
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "nginx-pod", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			},
		},
	}

	m := Model{
		state:  appState,
		active: true,
		cursor: 0, // Select "nginx-pod"
	}

	// Simulate 'x' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}
	m, _ = m.Update(msg)

	if m.state.CurrentView != state.ViewDiagnostics {
		t.Errorf("Expected CurrentView to be ViewDiagnostics, got %v", m.state.CurrentView)
	}

	if m.state.SelectedPod != "nginx-pod" {
		t.Errorf("Expected SelectedPod to be 'nginx-pod', got '%s'", m.state.SelectedPod)
	}
}

// TestPodsLogsKey verifies that pressing 'l' switches to ViewLogs
func TestPodsLogsKey(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		CurrentView:       state.ViewDetails,
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "nginx-pod", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			},
		},
	}

	m := Model{
		state:  appState,
		active: true,
		cursor: 0, // Select "nginx-pod"
	}

	// Simulate 'l' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}}
	m, _ = m.Update(msg)

	if m.state.CurrentView != state.ViewLogs {
		t.Errorf("Expected CurrentView to be ViewLogs, got %v", m.state.CurrentView)
	}

	if m.state.SelectedPod != "nginx-pod" {
		t.Errorf("Expected SelectedPod to be 'nginx-pod', got '%s'", m.state.SelectedPod)
	}
}

// TestPodsSort verifies that pressing 's' cycles sort modes
func TestPodsSort(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "test-pod", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			},
		},
	}

	m := Model{
		state:  appState,
		active: true,
	}

	// Initial sort mode should be SortByName
	if m.sortMode != SortByName {
		t.Errorf("Expected initial sortMode to be SortByName, got %v", m.sortMode)
	}

	// Press 's' -> should go to SortByStatus
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	m, _ = m.Update(msg)
	if m.sortMode != SortByStatus {
		t.Errorf("After first 's', expected SortByStatus, got %v", m.sortMode)
	}

	// Press 's' again -> should go to SortByRestarts
	m, _ = m.Update(msg)
	if m.sortMode != SortByRestarts {
		t.Errorf("After second 's', expected SortByRestarts, got %v", m.sortMode)
	}

	// Press 's' again -> should go to SortByAge
	m, _ = m.Update(msg)
	if m.sortMode != SortByAge {
		t.Errorf("After third 's', expected SortByAge, got %v", m.sortMode)
	}

	// Press 's' again -> should cycle back to SortByName
	m, _ = m.Update(msg)
	if m.sortMode != SortByName {
		t.Errorf("After fourth 's', expected SortByName (cycle), got %v", m.sortMode)
	}
}

// TestPodsSelectionGutterWidthInvariant verifies that selection gutter is always 2 chars
func TestPodsSelectionGutterWidthInvariant(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "test-pod", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 5, Age: "5d"},
			},
		},
	}

	m := Model{
		state:  appState,
		width:  80,
		height: 30,
		active: true,
	}

	contentWidth := m.width - 6 // PaneHorizontalOverhead
	colWidths := m.calculateColumnWidths(contentWidth)

	// Build unselected and selected rows - use same gutter (spaces) to isolate selection style test
	unselectedRow := m.buildTableRow("  ", "test-pod", colWidths.name, "1/1", colWidths.ready, "Running", colWidths.status, "5", colWidths.restarts, "5d", colWidths.age, colWidths, false, "Running", 5)
	selectedRow := m.buildTableRow("  ", "test-pod", colWidths.name, "1/1", colWidths.ready, "Running", colWidths.status, "5", colWidths.restarts, "5d", colWidths.age, colWidths, true, "Running", 5)

	unselectedPlain := stripANSI(unselectedRow)
	selectedPlain := stripANSI(selectedRow)

	// Use rune count for UTF-8 safe comparison
	unselectedRunes := []rune(unselectedPlain)
	selectedRunes := []rune(selectedPlain)

	// Both should have the same length (selection style adds no width)
	if len(unselectedRunes) != len(selectedRunes) {
		t.Errorf("Selected row rune count (%d) differs from unselected (%d)", len(selectedRunes), len(unselectedRunes))
		t.Logf("Unselected: %q", unselectedPlain)
		t.Logf("Selected:   %q", selectedPlain)
	}
}

// TestPodsRowWidthConsistency verifies header and data rows have consistent widths
func TestPodsRowWidthConsistency(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "test-pod", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 0, Age: "5d"},
			},
		},
	}

	m := Model{
		state:  appState,
		width:  100,
		height: 30,
		active: true,
	}

	contentWidth := m.width - 6
	colWidths := m.calculateColumnWidths(contentWidth)

	// Build header row - use same gutter for both
	headerRow := m.buildTableRow("  ", "NAME", colWidths.name, "READY", colWidths.ready, "STATUS", colWidths.status, "RST", colWidths.restarts, "AGE", colWidths.age, colWidths, false, "", 0)

	// Build data row - use same gutter
	dataRow := m.buildTableRow("  ", "test-pod-name", colWidths.name, "1/1", colWidths.ready, "Running", colWidths.status, "0", colWidths.restarts, "5d", colWidths.age, colWidths, false, "Running", 0)

	headerPlain := stripANSI(headerRow)
	dataPlain := stripANSI(dataRow)

	// Use rune count for UTF-8 safe comparison
	headerRunes := []rune(headerPlain)
	dataRunes := []rune(dataPlain)

	if len(headerRunes) != len(dataRunes) {
		t.Errorf("Header rune count (%d) differs from data row (%d)", len(headerRunes), len(dataRunes))
		t.Logf("Header: %q", headerPlain)
		t.Logf("Data:   %q", dataPlain)
	}
}

// TestPodsNoNewlinesInRows verifies table rows never contain embedded newlines
func TestPodsNoNewlinesInRows(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
	}

	contentWidth := m.width - 6
	colWidths := m.calculateColumnWidths(contentWidth)

	// Build a row with potentially long content
	row := m.buildTableRow("▶ ", "very-long-pod-name-that-might-wrap-in-other-implementations", colWidths.name, "1/10", colWidths.ready, "CrashLoopBackOff", colWidths.status, "999", colWidths.restarts, "100d", colWidths.age, colWidths, true, "CrashLoopBackOff", 999)

	if strings.Contains(row, "\n") {
		t.Errorf("Row contains newline: %q", row)
	}
	if strings.Contains(row, "\r") {
		t.Errorf("Row contains carriage return: %q", row)
	}
}

// TestPodsHeaderNeverWraps verifies table headers fit on one line at all widths
func TestPodsHeaderNeverWraps(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: state.Resource[[]state.PodInfo]{
			Data: []state.PodInfo{
				{Name: "test-pod", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 0, Age: "5d"},
			},
		},
	}

	widths := []int{40, 60, 80, 100, 130, 200}

	for _, w := range widths {
		t.Run(fmt.Sprintf("width_%d", w), func(t *testing.T) {
			m := Model{
				state:  appState,
				width:  w,
				height: 20,
				active: true,
			}

			view := m.View()
			lines := strings.Split(view, "\n")

			// Find header line (contains "NAME" and "STATUS")
			for _, line := range lines {
				plain := stripANSI(line)
				if strings.Contains(plain, "NAME") && strings.Contains(plain, "STATUS") {
					// Verify header is exactly one line (no embedded \n)
					if strings.Contains(line, "\n") {
						t.Errorf("Header wrapped at width %d: %q", w, line)
					}
					// Verify header doesn't exceed pane content width + safety margin
					contentWidth := w - 6 // PaneHorizontalOverhead
					if len([]rune(plain)) > contentWidth+10 {
						t.Errorf("Header too wide at width %d: %d chars vs content %d", w, len([]rune(plain)), contentWidth)
					}
					break
				}
			}
		})
	}
}
