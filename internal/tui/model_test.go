package tui

import (
	"testing"
)

// TestLayoutModeSelection tests that layout modes are chosen correctly based on terminal width
func TestLayoutModeSelection(t *testing.T) {
	tests := []struct {
		name         string
		width        int
		expectedMode LayoutMode
	}{
		{
			name:         "Very wide terminal",
			width:        200,
			expectedMode: LayoutThreePane,
		},
		{
			name:         "Wide terminal (threshold)",
			width:        130,
			expectedMode: LayoutThreePane,
		},
		{
			name:         "Just below three-pane threshold",
			width:        129,
			expectedMode: LayoutTwoPane,
		},
		{
			name:         "Medium terminal",
			width:        100,
			expectedMode: LayoutTwoPane,
		},
		{
			name:         "At two-pane threshold",
			width:        85,
			expectedMode: LayoutTwoPane,
		},
		{
			name:         "Just below two-pane threshold",
			width:        84,
			expectedMode: LayoutSinglePane,
		},
		{
			name:         "Narrow terminal",
			width:        60,
			expectedMode: LayoutSinglePane,
		},
		{
			name:         "Very narrow terminal",
			width:        40,
			expectedMode: LayoutSinglePane,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Model{width: tt.width}
			actual := m.determineLayoutMode()
			if actual != tt.expectedMode {
				t.Errorf("width=%d: expected %v, got %v", tt.width, tt.expectedMode, actual)
			}
		})
	}
}

// TestPaneSizeCalculation_DoesNotPanic tests that pane sizing doesn't panic for different layouts
func TestPaneSizeCalculation_DoesNotPanic(t *testing.T) {
	tests := []struct {
		name   string
		width  int
		height int
	}{
		{"Three-pane layout", 150, 50},
		{"Two-pane layout", 100, 40},
		{"Single-pane layout", 60, 30},
		{"Very narrow", 40, 20},
		{"Very wide", 300, 80},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("updatePaneSizes() panicked: %v", r)
				}
			}()

			m := Model{
				width:  tt.width,
				height: tt.height,
			}
			m.updatePaneSizes()
			// If we get here without panicking, the test passes
		})
	}
}

// TestActivePaneInitialization tests that the active pane is set correctly on initialization
func TestActivePaneInitialization(t *testing.T) {
	m := NewModel(nil) // nil client is OK for this test

	if m.activePane != PaneNamespace {
		t.Errorf("Expected initial active pane to be PaneNamespace, got %d", m.activePane)
	}
}
