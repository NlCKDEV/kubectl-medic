package pods

import (
	"strings"
	"testing"

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

			widths := m.calculateColumnWidths()

			// Check that all columns have reasonable widths (> 0)
			if widths.name <= 0 {
				t.Errorf("Name column width must be positive, got %d", widths.name)
			}
			if widths.ready <= 0 {
				t.Errorf("Ready column width must be positive, got %d", widths.ready)
			}
			if widths.status <= 0 {
				t.Errorf("Status column width must be positive, got %d", widths.status)
			}
			if widths.restarts <= 0 {
				t.Errorf("Restarts column width must be positive, got %d", widths.restarts)
			}
			if widths.age <= 0 {
				t.Errorf("Age column width must be positive, got %d", widths.age)
			}
		})
	}
}

// TestFilteringPods tests that pod filtering works correctly
func TestFilteringPods(t *testing.T) {
	appState := &state.AppState{
		SelectedNamespace: "default",
		Pods: []state.PodInfo{
			{Name: "nginx-deployment-abc123", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			{Name: "nginx-deployment-def456", Namespace: "default", Status: "Running", Restarts: 0, Age: "5d"},
			{Name: "redis-pod-xyz789", Namespace: "default", Status: "CrashLoopBackOff", Restarts: 10, Age: "3d"},
			{Name: "postgres-pod-123", Namespace: "default", Status: "Running", Restarts: 1, Age: "10d"},
			{Name: "other-namespace-pod", Namespace: "kube-system", Status: "Running", Restarts: 0, Age: "100d"},
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
		Pods: []state.PodInfo{
			{Name: "zebra-pod", Namespace: "default", Status: "Running", Restarts: 1, Age: "5d"},
			{Name: "alpha-pod", Namespace: "default", Status: "CrashLoopBackOff", Restarts: 10, Age: "10d"},
			{Name: "beta-pod", Namespace: "default", Status: "Pending", Restarts: 0, Age: "1d"},
			{Name: "gamma-pod", Namespace: "default", Status: "Running", Restarts: 5, Age: "3d"},
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
		Pods: []state.PodInfo{
			{Name: "very-long-pod-name-that-exceeds-normal-column-width-limits", Namespace: "default", Status: "Running", Ready: "1/1", Restarts: 0, Age: "5d"},
			{Name: "short", Namespace: "default", Status: "CrashLoopBackOff", Ready: "0/1", Restarts: 999, Age: "100d"},
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

			widths := m.calculateColumnWidths()

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
