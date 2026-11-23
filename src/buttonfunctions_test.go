package main

import (
	"testing"

	"github.com/rivo/tview"
)

func TestSetRunControlsEnabledLeavesPauseAndExitAvailableWhileRunning(t *testing.T) {
	buttons := tview.NewForm().
		AddButton("Step", nil).
		AddButton("Run", nil).
		AddButton("Pause", nil).
		AddButton("Reset", nil).
		AddButton("Exit", nil).
		AddButton("Load", nil)

	setRunControlsEnabled(buttons, false)

	if !buttons.GetButton(1).IsDisabled() {
		t.Error("Run button should be disabled while running")
	}
	if buttons.GetButton(pauseButtonIndex).IsDisabled() {
		t.Error("Pause button should be enabled while running")
	}
	if buttons.GetButton(exitButtonIndex).IsDisabled() {
		t.Error("Exit button should remain enabled while running")
	}

	setRunControlsEnabled(buttons, true)
	if !buttons.GetButton(pauseButtonIndex).IsDisabled() {
		t.Error("Pause button should be disabled after execution stops")
	}
	if buttons.GetButton(1).IsDisabled() {
		t.Error("Run button should be enabled after execution stops")
	}
}
