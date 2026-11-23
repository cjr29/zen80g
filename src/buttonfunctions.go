package main

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func stepCPU(cpu *z80.Z80, controller *interruptController, output *terminalOutputWriter,
	app *tview.Application,
	status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	pcAddressInput *tview.InputField) {

	terminalInterruptActive := false
	updateTerminalInterrupt(cpu, controller, &terminalInterruptActive)
	cycles := cpu.Step()
	updateTerminalInterrupt(cpu, controller, &terminalInterruptActive)
	if output.TakeScrollRequest() {
		status.ScrollToEnd()
	}
	updatePCAddressInput(pcAddressInput, cpu.PC)
	processorMemory.Clear()
	registers.Clear()
	appendStatus(status, fmt.Sprintf("Single Step, cycles: %d", cycles))
	refreshMemoryDump(processorMemory, mem)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)
}

func runCPU(cpu *z80.Z80, controller *interruptController, output *terminalOutputWriter, app *tview.Application, status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	buttons *tview.Form, running *atomic.Bool, pcAddressInput *tview.InputField) {

	// cycles := 0
	lastRefresh := time.Now()
	terminalInterruptActive := false
	for running.Load() {
		updateTerminalInterrupt(cpu, controller, &terminalInterruptActive)
		if cpu.Halted && !cpu.INT {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		//cycles = cycles + cpu.Step()
		cpu.Step()
		if output.TakeScrollRequest() {
			app.QueueUpdateDraw(func() {
				status.ScrollToEnd()
			})
		}
		updateTerminalInterrupt(cpu, controller, &terminalInterruptActive)
		if time.Since(lastRefresh) >= 5000*time.Millisecond {
			app.QueueUpdateDraw(func() {
				if !running.Load() {
					return
				}
				// appendStatus(status, fmt.Sprintf("Running, cycles: %d", cycles))
				appendStatus(status, fmt.Sprintf("Running ... %v", time.Now().Format("15:04:05")))
				processorMemory.Clear()
				registers.Clear()
				updatePCAddressInput(pcAddressInput, cpu.PC)
				refreshMemoryDump(processorMemory, mem)
				refreshStackDump(stack, mem, cpu, stackStartAddress)
				registerdump(registers, cpu)
			})
			lastRefresh = time.Now()
		}
	}

	paused := !running.Load() && !cpu.Halted
	app.QueueUpdateDraw(func() {
		processorMemory.Clear()
		registers.Clear()
		updatePCAddressInput(pcAddressInput, cpu.PC)
		if paused {
			appendStatus(status, fmt.Sprintf("Paused at PC %04X.", cpu.PC))
		} else {
			appendStatus(status, fmt.Sprintf("Halted, PC: %04X", cpu.PC))
		}
		refreshMemoryDump(processorMemory, mem)
		refreshStackDump(stack, mem, cpu, stackStartAddress)
		registerdump(registers, cpu)
		setRunControlsEnabled(buttons, true)
		running.Store(false)
		app.SetFocus(buttons.GetButton(stepButtonIndex))
	})
}

func updateTerminalInterrupt(cpu *z80.Z80, controller *interruptController, active *bool) {
	if controller.HasTerminalInput() {
		*active = true
		cpu.INT = true
	} else if *active {
		cpu.INT = false
		*active = false
	}
}

type terminalOutputWriter struct {
	status                    *tview.TextView
	mu                        sync.Mutex
	previousWasCarriageReturn bool
	scrollPending             atomic.Bool
}

func (w *terminalOutputWriter) WriteCharacter(character uint8) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if character == '\n' && w.previousWasCarriageReturn {
		w.previousWasCarriageReturn = false
		return
	}
	w.previousWasCarriageReturn = character == '\r'

	if character == '\r' || character == '\n' {
		_, _ = w.status.Write([]byte{'\n'})
	} else {
		_, _ = w.status.Write([]byte{character})
	}
	w.scrollPending.Store(true)
}

func (w *terminalOutputWriter) TakeScrollRequest() bool {
	return w.scrollPending.Swap(false)
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
	for i := 0; i < buttons.GetButtonCount(); i++ {
		switch i {
		case pauseButtonIndex:
			buttons.GetButton(i).SetDisabled(enabled)
		case exitButtonIndex:
			buttons.GetButton(i).SetDisabled(false)
		default:
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
