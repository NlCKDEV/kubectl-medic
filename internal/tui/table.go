package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/NlCKDEV/kubectl-medic/internal/util"
)

// TableColumn represents a single column in a table row.
// Style should be color-only (no padding) to avoid width changes.
type TableColumn struct {
	Value string          // The text value to display
	Width int             // Exact width for this column (pad/truncate to fit)
	Style *lipgloss.Style // Optional color-only style (nil = no styling)
	Align AlignMode       // Alignment mode (left or right)
}

// AlignMode specifies text alignment within a column
type AlignMode int

const (
	AlignLeft AlignMode = iota
	AlignRight
)

// BuildTableRow constructs a table row with consistent column alignment.
// This is the single source of truth for table row rendering.
//
// Pipeline:
//  1. For each column: truncate/pad to exact width
//  2. Apply per-column color-only styling (no padding)
//  3. Prepend gutter (fixed width, e.g., "▶ " or "  ")
//  4. Join columns with single-space separators
//  5. Apply selection styling (color-only, no padding)
//
// Parameters:
//   - gutter: Fixed-width selection indicator ("▶ " or "  ")
//   - columns: Slice of column specifications
//   - isSelected: Whether to apply selection highlighting
//   - selectionStyle: Color-only style for selected rows (must have no padding)
func BuildTableRow(gutter string, columns []TableColumn, isSelected bool, selectionStyle lipgloss.Style) string {
	var parts []string

	for _, col := range columns {
		if col.Width <= 0 {
			continue // Skip zero-width columns
		}

		// Truncate/pad to exact width
		var text string
		if col.Align == AlignRight {
			text = util.PadLeft(col.Value, col.Width)
		} else {
			text = util.PadRight(col.Value, col.Width)
		}

		// Apply color-only styling if provided
		if col.Style != nil {
			text = col.Style.Render(text)
		}

		parts = append(parts, text)
	}

	// Combine gutter + columns with single-space separators
	row := gutter + strings.Join(parts, " ")

	// Apply selection styling (color-only, no padding)
	if isSelected {
		row = selectionStyle.Render(row)
	}

	return row
}

// CalculateRowWidth returns the total visual width of a row given column widths.
// Accounts for gutter width and single-space separators between columns.
func CalculateRowWidth(gutterWidth int, columnWidths []int) int {
	if len(columnWidths) == 0 {
		return gutterWidth
	}

	total := gutterWidth
	visibleCols := 0

	for _, w := range columnWidths {
		if w > 0 {
			total += w
			visibleCols++
		}
	}

	// Add separators (one space between each visible column)
	if visibleCols > 1 {
		total += visibleCols - 1
	}

	return total
}

// ClampColumnWidths adjusts column widths to fit within maxWidth.
// It reduces the expandable column (usually the first/name column) if needed.
// Returns the adjusted widths and whether any clamping occurred.
func ClampColumnWidths(columnWidths []int, gutterWidth, maxWidth int) ([]int, bool) {
	result := make([]int, len(columnWidths))
	copy(result, columnWidths)

	total := CalculateRowWidth(gutterWidth, result)
	if total <= maxWidth {
		return result, false
	}

	// Reduce first column (typically name/expandable) to fit
	overflow := total - maxWidth
	if len(result) > 0 && result[0] > overflow {
		result[0] -= overflow
		return result, true
	}

	// If first column can't absorb all overflow, reduce it to minimum
	if len(result) > 0 {
		result[0] = max(8, result[0]-overflow) // Minimum 8 chars for name
	}

	return result, true
}
