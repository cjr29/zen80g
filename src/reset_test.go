package main

import (
	"strings"
	"testing"

	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func TestResetClearsStackDumpAndScrollPosition(t *testing.T) {
	mem := memory.NewRAM()
	mem.Write(0xFF00, 0xA5)
	cpu := z80.New(mem, nil)
	cpu.SP = 0xFF00

	stack := tview.NewTextView().SetScrollable(true)
	stack.SetText(strings.Repeat("old stack row\n", 20))
	stack.ScrollTo(10, 0)
	registers := tview.NewTextView()
	processorMemory := tview.NewTextView()
	status := tview.NewTextView()
	pcAddressInput := tview.NewInputField()

	resetCPU(cpu, nil, status, processorMemory, mem, stack, registers, 0, 0xFFFF, pcAddressInput)

	if got, want := stack.GetText(true), "SP: ffff, [stack start: ffff]"; got != want {
		t.Fatalf("stack dump after reset = %q, want %q", got, want)
	}
	if row, column := stack.GetScrollOffset(); row != 0 || column != 0 {
		t.Fatalf("stack scroll offset after reset = (%d, %d), want (0, 0)", row, column)
	}
	if got := mem.Read(0xFF00); got != 0 {
		t.Fatalf("memory after reset = %#02x, want 0", got)
	}
}
