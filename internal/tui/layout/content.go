package layout

import (
	"strings"

	"github.com/NlCKDEV/kubectl-medic/internal/theme"
	"github.com/NlCKDEV/kubectl-medic/internal/tui/constants"
	"github.com/charmbracelet/lipgloss"
)

// ContentBlock represents a block of content with consistent styling and padding
type ContentBlock struct {
	Title     string // Optional title (uses SectionHeaderStyle)
	Content   string // Main content
	LeftPad   int    // Left padding in spaces
	RightPad  int    // Right padding in spaces
	TopPad    int    // Top padding in lines
	BottomPad int    // Bottom padding in lines
	Width     int    // Maximum width for wrapping
}

// Render formats a content block with proper styling, padding, and visual hierarchy
// Returns string with styled content and internal padding applied
func (cb ContentBlock) Render() string {
	var lines []string

	// Add top padding (blank lines)
	for i := 0; i < cb.TopPad; i++ {
		lines = append(lines, "")
	}

	// Add title if provided
	if cb.Title != "" {
		styledTitle := theme.SectionHeaderStyle.Render(cb.Title)
		lines = append(lines, cb.padLine(styledTitle))
	}

	// Split content into lines and pad each
	contentLines := strings.Split(cb.Content, "\n")
	for _, line := range contentLines {
		if line == "" {
			// Preserve blank lines for paragraph breaks
			lines = append(lines, "")
		} else {
			lines = append(lines, cb.padLine(line))
		}
	}

	// Add bottom padding (blank lines)
	for i := 0; i < cb.BottomPad; i++ {
		lines = append(lines, "")
	}

	return strings.Join(lines, "\n")
}

// padLine adds left and right padding to a single line
func (cb ContentBlock) padLine(line string) string {
	leftPad := strings.Repeat(" ", cb.LeftPad)
	rightPad := strings.Repeat(" ", cb.RightPad)
	return leftPad + line + rightPad
}

// WrapContentText wraps text to fit within a specific width while preserving paragraph breaks
// Returns array of lines, each fitting within maxWidth
func WrapContentText(text string, maxWidth int) []string {
	if maxWidth < 20 {
		maxWidth = 20 // Absolute minimum
	}

	var result []string
	paragraphs := strings.Split(text, "\n\n")

	for _, para := range paragraphs {
		lines := wrapParagraph(para, maxWidth)
		result = append(result, lines...)
		result = append(result, "") // Blank line between paragraphs
	}

	// Remove trailing blank lines
	for len(result) > 0 && result[len(result)-1] == "" {
		result = result[:len(result)-1]
	}

	return result
}

// wrapParagraph wraps a single paragraph to fit within maxWidth
func wrapParagraph(text string, maxWidth int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{}
	}

	var lines []string
	var currentLine string

	for _, word := range words {
		testLine := currentLine
		if testLine != "" {
			testLine += " "
		}
		testLine += word

		if len(testLine) <= maxWidth {
			currentLine = testLine
		} else {
			// Current line is full, save it and start new line
			if currentLine != "" {
				lines = append(lines, currentLine)
			}
			currentLine = word

			// Handle words longer than maxWidth
			if len(currentLine) > maxWidth {
				lines = append(lines, currentLine)
				currentLine = ""
			}
		}
	}

	// Add final line
	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	return lines
}

// DetailsBlock renders a details section with title, separator, and content
// Used for Pod Details, Containers, Conditions, etc
// Returns formatted content ready for viewport
func DetailsBlock(title string, content string, contentWidth int) string {
	if contentWidth < 20 {
		contentWidth = 20
	}

	var b strings.Builder

	// Section title (uses visual hierarchy color)
	b.WriteString(theme.SectionHeaderStyle.Render(title) + "\n")

	// Separator line - use separator style
	sepWidth := contentWidth - 2 // Account for left/right padding
	b.WriteString(theme.SeparatorStyle.Render(strings.Repeat("─", sepWidth)) + "\n")

	// Content with left padding
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if line == "" {
			b.WriteString("\n")
		} else {
			b.WriteString("  " + line + "\n")
		}
	}

	return b.String()
}

// CenteredText centers text within a given width
// Useful for headers, legends, centered messages
func CenteredText(text string, width int) string {
	textLen := lipgloss.Width(text)
	if textLen >= width {
		return text // Too wide to center
	}

	padding := (width - textLen) / 2
	if padding < 0 {
		padding = 0
	}

	return strings.Repeat(" ", padding) + text
}

// LegendBox creates a formatted command legend/reference box
// Used for help text, shortcuts, command references
func LegendBox(items []string, contentWidth int) string {
	if contentWidth < 30 {
		contentWidth = 30
	}

	var b strings.Builder

	for _, item := range items {
		// Wrap long items
		if len(item) > contentWidth {
			// Try to wrap at space
			parts := strings.SplitN(item, " ", 2)
			if len(parts) == 2 {
				b.WriteString("  " + parts[0] + "\n")
				b.WriteString("    " + parts[1] + "\n")
			} else {
				b.WriteString("  " + item + "\n")
			}
		} else {
			b.WriteString("  " + item + "\n")
		}
	}

	return b.String()
}

// PaneContentPadding returns left/right padding for pane content
// Ensures content doesn't stick directly to pane border
func PaneContentPadding() (left, right int) {
	// Standard padding: 1 space from border, plus internal spacing
	return 1, 1
}

// ContentWidthForPane calculates the actual content width within a pane
// Accounts for border, padding, and any additional margins
func ContentWidthForPane(paneWidth int) int {
	// Pane structure: border(2) + pading(4) + content + padding(4) + border(2) = total
	// But we pass paneWidth already accounting for border
	// So: paneWidth - PaneHorizontalOverhead = usable
	// Then subtract padding for content: usable - (left+right padding)
	contentWidth := paneWidth - constants.PaneHorizontalOverhead - 2 // 2 for left/right padding
	if contentWidth < 20 {
		contentWidth = 20
	}
	return contentWidth
}
