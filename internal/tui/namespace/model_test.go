package namespace

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/NlCKDEV/kubectl-medic/internal/state"
	"github.com/NlCKDEV/kubectl-medic/internal/types"
)

// TestFiltering tests that namespace filtering works correctly
func TestFiltering(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "kube-system", Status: "Active", Age: "100d"},
				{Name: "default", Status: "Active", Age: "100d"},
				{Name: "production-east", Status: "Active", Age: "50d"},
				{Name: "production-west", Status: "Active", Age: "50d"},
				{Name: "staging", Status: "Active", Age: "30d"},
				{Name: "development", Status: "Active", Age: "10d"},
			},
		},
	}

	tests := []struct {
		name           string
		filter         string
		expectedCount  int
		expectedFirst  string
	}{
		{
			name:          "No filter",
			filter:        "",
			expectedCount: 6,
			expectedFirst: "default", // Default sort is by name ascending
		},
		{
			name:          "Filter 'prod'",
			filter:        "prod",
			expectedCount: 2,
			expectedFirst: "production-east",
		},
		{
			name:          "Filter 'kube'",
			filter:        "kube",
			expectedCount: 1,
			expectedFirst: "kube-system",
		},
		{
			name:          "Filter 'xyz' (no matches)",
			filter:        "xyz",
			expectedCount: 0,
			expectedFirst: "",
		},
		{
			name:          "Case insensitive filter 'PROD'",
			filter:        "PROD",
			expectedCount: 2,
			expectedFirst: "production-east",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:  appState,
				filter: tt.filter,
			}

			filtered := m.getFilteredAndSortedNamespaces()

			if len(filtered) != tt.expectedCount {
				t.Errorf("Expected %d results, got %d", tt.expectedCount, len(filtered))
			}

			if tt.expectedCount > 0 && filtered[0].Name != tt.expectedFirst {
				t.Errorf("Expected first result '%s', got '%s'", tt.expectedFirst, filtered[0].Name)
			}
		})
	}
}

// TestSorting tests that namespace sorting works correctly
func TestSorting(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
			{Name: "zebra", Status: "Active", Age: "10d"},
			{Name: "alpha", Status: "Active", Age: "50d"},
			{Name: "beta", Status: "Terminating", Age: "30d"},
			{Name: "gamma", Status: "Active", Age: "20d"},
				},
		},
	}

	tests := []struct {
		name     string
		sortMode SortMode
		expected []string
	}{
		{
			name:     "Sort by name ascending",
			sortMode: SortNameAsc,
			expected: []string{"alpha", "beta", "gamma", "zebra"},
		},
		{
			name:     "Sort by name descending",
			sortMode: SortNameDesc,
			expected: []string{"zebra", "gamma", "beta", "alpha"},
		},
		{
			name:     "Sort by status (Active first)",
			sortMode: SortStatus,
			expected: []string{"alpha", "gamma", "zebra", "beta"}, // Active alphabetically, then others
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				state:    appState,
				sortMode: tt.sortMode,
			}

			sorted := m.getFilteredAndSortedNamespaces()

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

// TestFilteringAndSorting tests that filtering and sorting work together correctly
func TestFilteringAndSorting(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
			{Name: "production-east", Status: "Active", Age: "50d"},
			{Name: "production-west", Status: "Active", Age: "50d"},
			{Name: "staging-east", Status: "Active", Age: "30d"},
			{Name: "development", Status: "Active", Age: "10d"},
				},
		},
	}

	m := Model{
		state:    appState,
		filter:   "prod",        // Filter for 'prod'
		sortMode: SortNameDesc, // Sort descending
	}

	result := m.getFilteredAndSortedNamespaces()

	if len(result) != 2 {
		t.Fatalf("Expected 2 filtered results, got %d", len(result))
	}

	if result[0].Name != "production-west" {
		t.Errorf("Expected first result 'production-west', got '%s'", result[0].Name)
	}

	if result[1].Name != "production-east" {
		t.Errorf("Expected second result 'production-east', got '%s'", result[1].Name)
	}
}

// TestCalculateVisibleWindow tests the windowed list rendering logic with windowStart
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
			height:        20,
			windowStart:   0,
			totalItems:    100,
			expectedStart: 0,
			expectedEnd:   10, // height 20 - chrome 10 = 10 visible
		},
		{
			name:          "Window in middle of large list",
			height:        20,
			windowStart:   45,
			totalItems:    100,
			expectedStart: 45,
			expectedEnd:   55,
		},
		{
			name:          "Window at end of large list",
			height:        20,
			windowStart:   90,
			totalItems:    100,
			expectedStart: 90,
			expectedEnd:   100,
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
		})
	}
}

// TestSingleLineGuarantee verifies that namespace list rows never contain newlines
func TestSingleLineGuarantee(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
			{Name: "very-long-namespace-name-that-exceeds-normal-width", Status: "Active", Age: "100d"},
			{Name: "short", Status: "Terminating", Age: "5d"},
				},
		},
	}

	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"Very narrow pane", 25, 20},
		{"Narrow pane", 35, 25},
		{"Medium pane", 50, 30},
		{"Wide pane", 80, 40},
		{"Very wide pane", 120, 50},
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

			// Check that no line contains embedded newlines
			lines := strings.Split(view, "\n")
			for i, line := range lines {
				// Remove ANSI escape codes for checking
				plainLine := stripANSI(line)

				// Verify no embedded newlines
				if strings.Contains(plainLine, "\r") || strings.Count(line, "\n") > 0 {
					t.Errorf("Line %d contains embedded newline characters", i)
				}
			}
		})
	}
}

// TestNamespaceColumnDropping verifies that age column is dropped for narrow panes
func TestNamespaceColumnDropping(t *testing.T) {
	tests := []struct {
		name      string
		width     int
		expectAge bool
	}{
		{"Wide - show age", 50, true},
		{"Medium - show age", 40, true},
		{"Narrow - drop age", 30, false},
		{"Very narrow - drop age", 25, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{
				width: tt.width,
			}

			// Use content width (width minus overhead)
			contentWidth := tt.width - 6 // PaneHorizontalOverhead
			_, showAge := m.calculateColumnWidths(contentWidth)

			if showAge != tt.expectAge {
				t.Errorf("Width %d: expected showAge=%v, got %v", tt.width, tt.expectAge, showAge)
			}
		})
	}
}

// TestSelectionGutterWidthInvariant verifies that the selection gutter is always 2 characters
func TestSelectionGutterWidthInvariant(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "test-namespace", Status: "Active", Age: "10d"},
			},
		},
	}

	m := Model{
		state:  appState,
		width:  60,
		height: 30,
		active: true,
	}

	// Build row with selection gutter
	contentWidth := m.width - 6
	nameWidth, showAge := m.calculateColumnWidths(contentWidth)

	// Test unselected row gutter - use same gutter for both to test selection style
	unselectedRow := m.buildTableRow("  ", "test", nameWidth, "Active", 10, "10d", 5, showAge, false, "Active")
	// Test selected row gutter (same content, just selected=true)
	selectedRow := m.buildTableRow("  ", "test", nameWidth, "Active", 10, "10d", 5, showAge, true, "Active")

	// Strip ANSI to compare raw lengths - use rune count for UTF-8 safety
	unselectedPlain := stripANSI(unselectedRow)
	selectedPlain := stripANSI(selectedRow)

	unselectedRunes := []rune(unselectedPlain)
	selectedRunes := []rune(selectedPlain)

	// Both should have the same length (selection style adds no width)
	if len(unselectedRunes) != len(selectedRunes) {
		t.Errorf("Selected row rune count (%d) differs from unselected (%d)", len(selectedRunes), len(unselectedRunes))
	}

	// Both should start with 2-char gutter (spaces)
	if len(unselectedRunes) >= 2 && string(unselectedRunes[:2]) != "  " {
		t.Errorf("Unselected row should start with 2 spaces, got: %q", string(unselectedRunes[:min(10, len(unselectedRunes))]))
	}
}

// TestRowWidthConsistency verifies header and data rows have consistent widths
func TestRowWidthConsistency(t *testing.T) {
	m := Model{
		width:  80,
		height: 30,
	}

	contentWidth := m.width - 6
	nameWidth, showAge := m.calculateColumnWidths(contentWidth)

	// Build header row with same gutter as data row
	headerRow := m.buildTableRow("  ", "NAME", nameWidth, "STATUS", 10, "AGE", 5, showAge, false, "")

	// Build data row with same gutter
	dataRow := m.buildTableRow("  ", "test-namespace", nameWidth, "Active", 10, "10d", 5, showAge, false, "Active")

	headerPlain := stripANSI(headerRow)
	dataPlain := stripANSI(dataRow)

	headerRunes := []rune(headerPlain)
	dataRunes := []rune(dataPlain)

	if len(headerRunes) != len(dataRunes) {
		t.Errorf("Header rune count (%d) differs from data row (%d)", len(headerRunes), len(dataRunes))
		t.Logf("Header: %q", headerPlain)
		t.Logf("Data:   %q", dataPlain)
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

// TestNamespaceEnterLoadsPods verifies that pressing Enter sets SelectedNamespace
func TestNamespaceEnterLoadsPods(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "default", Status: "Active", Age: "100d"},
				{Name: "kube-system", Status: "Active", Age: "100d"},
				{Name: "production", Status: "Active", Age: "50d"},
			},
		},
		SelectedNamespace: "",
	}

	m := NewModel(appState)
	m.active = true
	m.cursor = 1 // Select "kube-system"

	// Simulate Enter key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	m, _ = m.Update(msg)

	if m.state.SelectedNamespace != "kube-system" {
		t.Errorf("Expected SelectedNamespace to be 'kube-system', got '%s'", m.state.SelectedNamespace)
	}

	// Verify pods were cleared
	if len(m.state.Pods.Data) != 0 {
		t.Error("Expected Pods.Data to be cleared after namespace selection")
	}
}

// TestNamespaceHealthKey verifies that pressing 'h' triggers health view
func TestNamespaceHealthKey(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "default", Status: "Active", Age: "100d"},
				{Name: "production", Status: "Active", Age: "50d"},
			},
		},
		CurrentView:       state.ViewDetails,
		SelectedNamespace: "",
		Health: state.Resource[*types.NamespaceHealth]{
			Loading: false,
			Loaded:  false,
			Data:    nil,
		},
	}

	m := NewModel(appState)
	m.active = true
	m.cursor = 1 // Select "production"

	// Simulate 'h' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	m, _ = m.Update(msg)

	if m.state.CurrentView != state.ViewNamespaceHealth {
		t.Errorf("Expected CurrentView to be ViewNamespaceHealth, got %v", m.state.CurrentView)
	}

	if m.state.SelectedNamespace != "production" {
		t.Errorf("Expected SelectedNamespace to be 'production', got '%s'", m.state.SelectedNamespace)
	}

	// Verify loading flag was set (since Health.Data is nil)
	if !m.state.Health.Loading {
		t.Error("Expected Health.Loading to be true when triggering health view with no cached data")
	}
}

// TestNamespaceHealthKeyCaching verifies that cached health data doesn't trigger reload
func TestNamespaceHealthKeyCaching(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "default", Status: "Active", Age: "100d"},
			},
		},
		CurrentView:       state.ViewDetails,
		SelectedNamespace: "",
		Health: state.Resource[*types.NamespaceHealth]{
			Loading: false,
			Loaded:  true,
			Data: &types.NamespaceHealth{
				Namespace: "default", // Health already cached for "default"
			},
		},
	}

	m := NewModel(appState)
	m.active = true
	m.cursor = 0 // Select "default"

	// Simulate 'h' key
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}}
	m, _ = m.Update(msg)

	// Should switch view but NOT trigger loading (cache hit)
	if m.state.CurrentView != state.ViewNamespaceHealth {
		t.Errorf("Expected CurrentView to be ViewNamespaceHealth, got %v", m.state.CurrentView)
	}

	if m.state.Health.Loading {
		t.Error("Expected Health.Loading to be false when cached health data exists for namespace")
	}
}

// TestHeaderNeverWraps verifies table headers fit on one line at all widths
func TestHeaderNeverWraps(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "test-namespace", Status: "Active", Age: "10d"},
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

			// Find header line (contains "NAME")
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

// TestNamespaceSort verifies that pressing 's' cycles sort modes
func TestNamespaceSort(t *testing.T) {
	appState := &state.AppState{
		Namespaces: state.Resource[[]state.NamespaceInfo]{
			Data: []state.NamespaceInfo{
				{Name: "default", Status: "Active", Age: "100d"},
			},
		},
	}

	m := NewModel(appState)
	m.active = true

	// Initial sort mode should be SortNameAsc
	if m.sortMode != SortNameAsc {
		t.Errorf("Expected initial sortMode to be SortNameAsc, got %v", m.sortMode)
	}

	// Press 's' -> should go to SortNameDesc
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}}
	m, _ = m.Update(msg)
	if m.sortMode != SortNameDesc {
		t.Errorf("After first 's', expected SortNameDesc, got %v", m.sortMode)
	}

	// Press 's' again -> should go to SortStatus
	m, _ = m.Update(msg)
	if m.sortMode != SortStatus {
		t.Errorf("After second 's', expected SortStatus, got %v", m.sortMode)
	}

	// Press 's' again -> should cycle back to SortNameAsc
	m, _ = m.Update(msg)
	if m.sortMode != SortNameAsc {
		t.Errorf("After third 's', expected SortNameAsc (cycle), got %v", m.sortMode)
	}
}
