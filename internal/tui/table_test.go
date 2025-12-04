package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/NlCKDEV/kubectl-medic/internal/theme"
)

// TestBuildTableRow_NoWrapping verifies that table rows never contain newlines
func TestBuildTableRow_NoWrapping(t *testing.T) {
	columns := []TableColumn{
		{Value: "test-namespace-with-a-very-long-name", Width: 20, Align: AlignLeft},
		{Value: "Active", Width: 10, Align: AlignLeft},
		{Value: "100d", Width: 5, Align: AlignLeft},
	}

	row := BuildTableRow("  ", columns, false, theme.TableRowSelectedStyle)

	if strings.Contains(row, "\n") {
		t.Errorf("Row contains newline: %q", row)
	}
	if strings.Contains(row, "\r") {
		t.Errorf("Row contains carriage return: %q", row)
	}
}

// TestBuildTableRow_SelectionDoesNotChangeWidth verifies selection doesn't add width
func TestBuildTableRow_SelectionDoesNotChangeWidth(t *testing.T) {
	columns := []TableColumn{
		{Value: "test-namespace", Width: 20, Align: AlignLeft},
		{Value: "Active", Width: 10, Align: AlignLeft},
	}

	unselected := BuildTableRow("  ", columns, false, theme.TableRowSelectedStyle)
	selected := BuildTableRow("  ", columns, true, theme.TableRowSelectedStyle)

	// Strip ANSI for comparison
	unselectedPlain := stripANSI(unselected)
	selectedPlain := stripANSI(selected)

	if lipgloss.Width(unselectedPlain) != lipgloss.Width(selectedPlain) {
		t.Errorf("Selection changed width: unselected=%d, selected=%d",
			lipgloss.Width(unselectedPlain), lipgloss.Width(selectedPlain))
	}
}

// TestBuildTableRow_GutterWidth verifies gutter is preserved correctly
func TestBuildTableRow_GutterWidth(t *testing.T) {
	columns := []TableColumn{
		{Value: "test", Width: 10, Align: AlignLeft},
	}

	// Test with 2-char gutter (using spaces for reliable testing)
	row := BuildTableRow("  ", columns, false, theme.TableRowSelectedStyle)
	plain := stripANSI(row)

	// Row should start with the gutter (2 spaces)
	if !strings.HasPrefix(plain, "  ") {
		t.Errorf("Row doesn't preserve gutter: %q", plain)
	}

	// Test with arrow gutter
	rowArrow := BuildTableRow("> ", columns, false, theme.TableRowSelectedStyle)
	plainArrow := stripANSI(rowArrow)
	if !strings.HasPrefix(plainArrow, "> ") {
		t.Errorf("Row doesn't preserve arrow gutter: %q", plainArrow)
	}
}

// TestBuildTableRow_ColumnAlignment verifies left/right alignment
func TestBuildTableRow_ColumnAlignment(t *testing.T) {
	columns := []TableColumn{
		{Value: "abc", Width: 6, Align: AlignLeft},
		{Value: "123", Width: 6, Align: AlignRight},
	}

	row := BuildTableRow("", columns, false, theme.TableRowSelectedStyle)
	plain := stripANSI(row)

	// Left-aligned: "abc   " (6 chars)
	// Single space separator: " " (1 char)
	// Right-aligned: "   123" (6 chars)
	// Total: 13 chars
	expected := "abc       123" // 6 + 1 + 6 = 13 chars
	if plain != expected {
		t.Errorf("Alignment incorrect: got %q (len=%d), want %q (len=%d)", plain, len(plain), expected, len(expected))
	}
}

// TestBuildTableRow_ZeroWidthColumnsSkipped verifies zero-width columns are skipped
func TestBuildTableRow_ZeroWidthColumnsSkipped(t *testing.T) {
	columns := []TableColumn{
		{Value: "visible", Width: 7, Align: AlignLeft},
		{Value: "hidden", Width: 0, Align: AlignLeft}, // Should be skipped
		{Value: "also", Width: 4, Align: AlignLeft},
	}

	row := BuildTableRow("", columns, false, theme.TableRowSelectedStyle)
	plain := stripANSI(row)

	if strings.Contains(plain, "hidden") {
		t.Errorf("Zero-width column should be skipped: %q", plain)
	}
}

// TestCalculateRowWidth verifies row width calculation
func TestCalculateRowWidth(t *testing.T) {
	tests := []struct {
		name         string
		gutterWidth  int
		columnWidths []int
		expected     int
	}{
		{
			name:         "Empty columns",
			gutterWidth:  2,
			columnWidths: []int{},
			expected:     2,
		},
		{
			name:         "Single column",
			gutterWidth:  2,
			columnWidths: []int{10},
			expected:     12, // 2 + 10
		},
		{
			name:         "Multiple columns with separators",
			gutterWidth:  2,
			columnWidths: []int{10, 8, 5},
			expected:     27, // 2 + 10 + 1 + 8 + 1 + 5
		},
		{
			name:         "With zero-width columns",
			gutterWidth:  2,
			columnWidths: []int{10, 0, 5},
			expected:     18, // 2 + 10 + 1 + 5 (zero-width skipped but still counted for separator)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateRowWidth(tt.gutterWidth, tt.columnWidths)
			if got != tt.expected {
				t.Errorf("CalculateRowWidth() = %d, want %d", got, tt.expected)
			}
		})
	}
}

// TestClampColumnWidths verifies column width clamping
func TestClampColumnWidths(t *testing.T) {
	tests := []struct {
		name         string
		columnWidths []int
		gutterWidth  int
		maxWidth     int
		wantClamped  bool
	}{
		{
			name:         "No clamping needed",
			columnWidths: []int{20, 10, 5},
			gutterWidth:  2,
			maxWidth:     50,
			wantClamped:  false,
		},
		{
			name:         "Clamping required",
			columnWidths: []int{20, 10, 5},
			gutterWidth:  2,
			maxWidth:     30,
			wantClamped:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, clamped := ClampColumnWidths(tt.columnWidths, tt.gutterWidth, tt.maxWidth)
			if clamped != tt.wantClamped {
				t.Errorf("ClampColumnWidths() clamped = %v, want %v", clamped, tt.wantClamped)
			}

			// If clamped, verify result fits
			if clamped {
				total := CalculateRowWidth(tt.gutterWidth, result)
				if total > tt.maxWidth {
					t.Errorf("Clamped result still too wide: %d > %d", total, tt.maxWidth)
				}
			}
		})
	}
}

// TestTableRowStylesHaveNoPadding verifies theme styles don't add padding
func TestTableRowStylesHaveNoPadding(t *testing.T) {
	testText := "test"

	// Apply styles and check they don't change visible width
	styles := map[string]lipgloss.Style{
		"TableRowStyle":         theme.TableRowStyle,
		"TableRowSelectedStyle": theme.TableRowSelectedStyle,
		"TableHeaderStyle":      theme.TableHeaderStyle,
		"ListItemSelectedStyle": theme.ListItemSelectedStyle,
	}

	for name, style := range styles {
		t.Run(name, func(t *testing.T) {
			styled := style.Render(testText)
			styledPlain := stripANSI(styled)

			if lipgloss.Width(styledPlain) != lipgloss.Width(testText) {
				t.Errorf("%s changes width: original=%d, styled=%d",
					name, lipgloss.Width(testText), lipgloss.Width(styledPlain))
			}
		})
	}
}

// Note: stripANSI is defined in model_test.go in the same package
