package util

import "strings"

// Truncate truncates a string to a maximum length without adding ellipsis
func Truncate(s string, max int) string {
	if max < 0 {
		max = 0
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// Ellipsize truncates a string to a maximum length and adds "…" if truncated
func Ellipsize(s string, max int) string {
	if max < 1 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	return s[:max-1] + "…"
}

// PadRight pads a string to a fixed width with spaces on the right
func PadRight(s string, width int) string {
	if len(s) >= width {
		return Ellipsize(s, width)
	}
	return s + strings.Repeat(" ", width-len(s))
}

// PadLeft pads a string to a fixed width with spaces on the left
func PadLeft(s string, width int) string {
	if len(s) >= width {
		return Ellipsize(s, width)
	}
	return strings.Repeat(" ", width-len(s)) + s
}

// TruncateMiddle truncates a string in the middle, keeping start and end
// Useful for long paths or identifiers
func TruncateMiddle(s string, max int) string {
	if max < 3 {
		return Ellipsize(s, max)
	}
	if len(s) <= max {
		return s
	}

	leftLen := (max - 1) / 2
	rightLen := max - leftLen - 1

	return s[:leftLen] + "…" + s[len(s)-rightLen:]
}

// FormatHelpLine joins help items with " | " and intelligently truncates
// if the result exceeds maxWidth. Priority items (earlier in list) are kept.
// Example: FormatHelpLine(60, "↑/↓ move", "Enter select", "/ filter", "s sort")
func FormatHelpLine(maxWidth int, items ...string) string {
	if maxWidth < 10 {
		// Too narrow, just show first item truncated
		if len(items) > 0 {
			return Ellipsize(items[0], maxWidth)
		}
		return ""
	}

	// Try full line first
	full := strings.Join(items, " | ")
	if len(full) <= maxWidth {
		return full
	}

	// Progressively drop lower-priority items (from end)
	for i := len(items) - 1; i > 0; i-- {
		partial := strings.Join(items[:i], " | ")
		if len(partial) <= maxWidth {
			return partial
		}
	}

	// If even first item is too long, truncate it
	if len(items) > 0 {
		return Ellipsize(items[0], maxWidth)
	}

	return ""
}
