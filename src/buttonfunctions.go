package main

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func stepCPU(cpu *z80.Z80,
	app *tview.Application,
	status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	pcAddressInput *tview.InputField) {

	cycles := cpu.Step()
	updatePCAddressInput(pcAddressInput, cpu.PC)
	processorMemory.Clear()
	registers.Clear()
	appendStatus(status, fmt.Sprintf("Single Step, cycles: %d", cycles))
	refreshMemoryDump(processorMemory, mem)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)
}

func runCPU(cpu *z80.Z80, app *tview.Application, status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	buttons *tview.Form, running *atomic.Bool, pcAddressInput *tview.InputField) {
	// cycles := 0
	lastRefresh := time.Now()
	for !cpu.Halted && running.Load() {
		//cycles = cycles + cpu.Step()
		cpu.Step()
		if time.Since(lastRefresh) >= 10000*time.Millisecond {
			app.QueueUpdateDraw(func() {
				if !running.Load() {
					return
				}
				// appendStatus(status, fmt.Sprintf("Running, cycles: %d", cycles))
				appendStatus(status, fmt.Sprintf("Running ..."))
				processorMemory.Clear()
				registers.Clear()
				updatePCAddressInput(pcAddressInput, cpu.PC)
				refreshMemoryDump(processorMemory, mem)
				refreshStackDump(stack, mem, cpu, stackStartAddress)
				registerdump(registers, cpu)
			})
			if !running.Load() {
				return
			}
			lastRefresh = time.Now()
		}
	}
	if !running.Load() {
		return
	}

	app.QueueUpdateDraw(func() {
		if !running.Load() {
			return
		}
		processorMemory.Clear()
		registers.Clear()
		updatePCAddressInput(pcAddressInput, cpu.PC)
		// appendStatus(status, fmt.Sprintf("Halted, cycles: %d", cycles))
		appendStatus(status, fmt.Sprintf("Halted, PC: %04X", cpu.PC))
		refreshMemoryDump(processorMemory, mem)
		refreshStackDump(stack, mem, cpu, stackStartAddress)
		registerdump(registers, cpu)
		setRunControlsEnabled(buttons, true)
		running.Store(false)
		app.SetFocus(buttons)
	})
}

func resetCPU(cpu *z80.Z80, app *tview.Application, status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM, stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	pcAddressInput *tview.InputField) {
	mem.Clear()
	cpu.Reset()
	cpu.A, cpu.F = 0, 0
	cpu.B, cpu.C = 0, 0
	cpu.D, cpu.E = 0, 0
	cpu.H, cpu.L = 0, 0
	cpu.A_, cpu.F_ = 0, 0
	cpu.B_, cpu.C_ = 0, 0
	cpu.D_, cpu.E_ = 0, 0
	cpu.H_, cpu.L_ = 0, 0
	cpu.IXH, cpu.IXL = 0, 0
	cpu.IYH, cpu.IYL = 0, 0
	cpu.WZ = 0
	cpu.SP = stackStartAddress
	cpu.PC = 0
	updatePCAddressInput(pcAddressInput, cpu.PC)
	cpu.NMI = false
	cpu.INT = false
	registers.Clear()
	processorMemory.Clear()
	refreshMemoryDump(processorMemory, mem)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	stack.ScrollToBeginning()
	registerdump(registers, cpu)
	appendStatus(status, "CPU reset; memory cleared.")
}

func appendStatus(status *tview.TextView, message string) {
	fmt.Fprintln(status, message)
	status.ScrollToEnd()
}

func refreshMemoryDump(view *tview.TextView, mem *memory.RAM) {
	scrollRow, scrollColumn := view.GetScrollOffset()
	view.Clear()
	memorydump(view, mem, 0, 1<<16)
	view.ScrollTo(scrollRow, scrollColumn)
}

func refreshStackDump(view *tview.TextView, mem *memory.RAM, cpu *z80.Z80, stackStartAddress uint16) {
	scrollRow, scrollColumn := view.GetScrollOffset()
	view.Clear()
	rowCount := 0
	if stackStartAddress > cpu.SP {
		rowCount = (int(stackStartAddress) - int(cpu.SP)) / 2
	}
	dumpstack(view, mem, cpu.SP, stackStartAddress, rowCount)
	view.ScrollTo(scrollRow, scrollColumn)
}

func isHexAddressInput(text string) bool {
	if len(text) > 4 {
		return false
	}
	for _, char := range text {
		if !(char >= '0' && char <= '9' ||
			char >= 'a' && char <= 'f' ||
			char >= 'A' && char <= 'F') {
			return false
		}
	}
	return true
}

func setRunControlsEnabled(buttons *tview.Form, enabled bool) {
	const exitButtonIndex = 3
	for i := 0; i < buttons.GetButtonCount(); i++ {
		if i != exitButtonIndex {
			buttons.GetButton(i).SetDisabled(!enabled)
		}
	}
}

func updatePCAddressInput(input *tview.InputField, pc uint16) {
	input.SetLabel(fmt.Sprintf("PC %04X: ", pc))
	input.SetPlaceholder(fmt.Sprintf("%04X", pc))
}

func loadProgramFromFile(path string) ([]uint8, error) {
	program, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read program %q: %w", path, err)
	}
	if len(program) > 1<<16 {
		return nil, fmt.Errorf("program %q is %d bytes; maximum size is 65536 bytes", path, len(program))
	}
	return program, nil
}
