package main

import (
	"testing"

	"github.com/rivo/tview"
)

func TestWriteTerminalOutputPreservesTextAndTranslatesCRLF(t *testing.T) {
	status := tview.NewTextView()
	output := &terminalOutputWriter{status: status}

	for _, character := range []byte("hello\r\nworld") {
		output.WriteCharacter(character)
	}

	if got, want := status.GetText(true), "hello\nworld"; got != want {
		t.Fatalf("status text = %q, want %q", got, want)
	}
}

func TestWriteInputLineEchoesAfterPartialTerminalOutput(t *testing.T) {
	status := tview.NewTextView()
	output := &terminalOutputWriter{status: status}
	output.WriteCharacter('O')
	output.WriteCharacter('K')

	output.WriteInputLine("next")

	if got, want := status.GetText(true), "OK\n$> next\n"; got != want {
		t.Fatalf("terminal text = %q, want %q", got, want)
	}
	if !output.TakeScrollRequest() {
		t.Fatal("echoing terminal input should request scrolling")
	}
}
