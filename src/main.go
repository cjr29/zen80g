package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func main() {
	/****************************************************************/
	/***********      Set up Z80 emulator    ************************/
	/****************************************************************/
	programPath := flag.String("program", "", "path to a raw Z80 binary to run")
	flag.Parse()

	// Create memory and I/O
	mem := memory.NewRAM()
	io := io.NewNullIO()
	cpu := z80.New(mem, io)

	// Load a default program for demo purposes. This program adds 5 and 3 and halts.
	program := []uint8{
		0x3E, 0x05, // LD A, 5
		0x06, 0x03, // LD B, 3
		0x80, // ADD A, B
		0x76, // HALT
	}

	// Load program from file if specified as option. If not, use the default program above.
	if *programPath != "" {
		var err error
		program, err = loadProgramFromFile(*programPath)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("Loaded program %q (%d bytes)\n", *programPath, len(program))
	}
	startAddress := uint16(0)
	displayAddress := startAddress
	stackStartAddress := uint16(0xFFFF)
	mem.Load(startAddress, program)

	/****************************************************************/
	/********************      Set up GUI    ************************/
	/****************************************************************/
	app := tview.NewApplication().SetTitle("zen80 - Z80 Emulator")

	//processorMemory := viewgrid("Memory")
	processorMemory := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(false)
	processorMemory.SetBorder(true).SetTitle("Memory")

	registers := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true)
	registers.SetBorder(true).SetTitle("Registers")

	stack := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(false)
	stack.SetBorder(true).SetTitle("Stack")

	status := tview.NewTextView().SetTextAlign(tview.AlignLeft)
	status.SetText(fmt.Sprintf("CPU status: ")).SetTextColor(tcell.ColorRed)

	pages := tview.NewPages()
	var buttons *tview.Form
	var running atomic.Bool
	var pcAddressInput *tview.InputField
	addressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Load at %04X: ", startAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", startAddress)).
		SetPlaceholderTextColor(tcell.ColorBlack).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	addressInput.SetChangedFunc(func(text string) {
		if len(text) != 4 {
			return
		}
		address, err := strconv.ParseUint(text, 16, 16)
		if err != nil {
			return
		}
		newAddress := uint16(address)
		if newAddress == startAddress {
			return
		}
		if running.Load() {
			addressInput.SetText("")
			return
		}

		startAddress = newAddress
		addressInput.SetLabel(fmt.Sprintf("Load at %04X: ", startAddress))
		addressInput.SetPlaceholder(fmt.Sprintf("%04X", startAddress))
		addressInput.SetText("")
		status.SetText(fmt.Sprintf("Program load address set to %04X. Current program is unchanged.", startAddress))
	})

	programPathInput := tview.NewInputField().
		SetLabel("Z80 binary path: ").
		SetFieldWidth(60)
	loadProgramForm := tview.NewForm().
		AddFormItem(programPathInput).
		AddButton("Load", func() {
			path := strings.TrimSpace(programPathInput.GetText())
			if path == "" {
				status.SetText("Enter the path to a Z80 binary file.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			loadedProgram, err := loadProgramFromFile(path)
			if err != nil {
				status.SetText(err.Error())
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}
			if len(loadedProgram) > (1<<16)-int(startAddress) {
				status.SetText(fmt.Sprintf("Program does not fit in memory at address %04X.", startAddress))
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			program = loadedProgram
			mem.Clear()
			mem.Load(startAddress, program)
			cpu.Reset()
			cpu.PC = startAddress
			processorMemory.Clear()
			registers.Clear()
			refreshMemoryDump(processorMemory, mem, displayAddress)
			updatePCAddressInput(pcAddressInput, cpu.PC)
			registerdump(registers, cpu)
			status.SetText(fmt.Sprintf("Loaded %q at address %04X (%d bytes).", path, startAddress, len(program)))
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		})
	loadProgramForm.SetBorder(true).SetTitle("Load Z80 Program")
	loadProgramForm.SetCancelFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(buttons)
	})

	displayAddressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Display at %04X: ", displayAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", displayAddress)).
		SetPlaceholderTextColor(tcell.ColorBlack).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	displayAddressInput.SetChangedFunc(func(text string) {
		if len(text) != 4 {
			return
		}
		address, err := strconv.ParseUint(text, 16, 16)
		if err != nil {
			return
		}
		newAddress := uint16(address)
		if newAddress == displayAddress {
			return
		}
		if running.Load() {
			displayAddressInput.SetText("")
			return
		}

		displayAddress = newAddress
		displayAddressInput.SetLabel(fmt.Sprintf("Display at %04X: ", displayAddress))
		displayAddressInput.SetPlaceholder(fmt.Sprintf("%04X", displayAddress))
		displayAddressInput.SetText("")
		refreshMemoryDump(processorMemory, mem, displayAddress)
		status.SetText(fmt.Sprintf("Displaying memory from address %04X.", displayAddress))
	})

	stackAddressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Stack at %04X: ", stackStartAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", stackStartAddress)).
		SetPlaceholderTextColor(tcell.ColorBlack).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	stackAddressInput.SetChangedFunc(func(text string) {
		if len(text) != 4 {
			return
		}
		address, err := strconv.ParseUint(text, 16, 16)
		if err != nil {
			return
		}
		newAddress := uint16(address)
		if newAddress == stackStartAddress {
			return
		}
		if running.Load() {
			stackAddressInput.SetText("")
			return
		}

		stackStartAddress = newAddress
		cpu.SP = stackStartAddress
		stackAddressInput.SetLabel(fmt.Sprintf("Stack at %04X: ", stackStartAddress))
		stackAddressInput.SetPlaceholder(fmt.Sprintf("%04X", stackStartAddress))
		stackAddressInput.SetText("")
		refreshStackDump(stack, mem, cpu, stackStartAddress)
		registers.Clear()
		registerdump(registers, cpu)
		status.SetText(fmt.Sprintf("Stack start address set to %04X.", stackStartAddress))
	})

	pcAddressInput = tview.NewInputField().
		SetLabel(fmt.Sprintf("PC %04X: ", cpu.PC)).
		SetPlaceholder(fmt.Sprintf("%04X", cpu.PC)).
		SetPlaceholderTextColor(tcell.ColorBlack).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	pcAddressInput.SetChangedFunc(func(text string) {
		if len(text) != 4 {
			return
		}
		address, err := strconv.ParseUint(text, 16, 16)
		if err != nil {
			return
		}
		newPC := uint16(address)
		if newPC == cpu.PC {
			pcAddressInput.SetText("")
			return
		}
		if running.Load() {
			pcAddressInput.SetText("")
			return
		}

		cpu.PC = newPC
		updatePCAddressInput(pcAddressInput, cpu.PC)
		pcAddressInput.SetText("")
		registers.Clear()
		registerdump(registers, cpu)
		status.SetText(fmt.Sprintf("Program counter set to %04X.", cpu.PC))
	})

	// Two buttons inside one grid cell.
	buttons = tview.NewForm().
		SetHorizontal(true).
		SetButtonsAlign(tview.AlignLeft).
		SetButtonBackgroundColor(tcell.ColorBlack).
		SetFieldBackgroundColor(tcell.ColorWhite).
		SetFieldTextColor(tcell.ColorBlack).
		AddFormItem(addressInput).
		AddFormItem(displayAddressInput).
		AddFormItem(stackAddressInput).
		AddFormItem(pcAddressInput).
		AddButton("[white]Step", func() {
			if running.Load() {
				return
			}
			stepCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, pcAddressInput)
		}).
		AddButton("[green]Run", func() {
			if !running.CompareAndSwap(false, true) {
				return
			}
			status.SetText(fmt.Sprintf("Running from address %04X.", cpu.PC))
			setRunControlsEnabled(buttons, false)
			app.SetFocus(buttons.GetButton(3))
			go runCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, buttons, &running, pcAddressInput)
		}).
		AddButton("[yellow]Reset", func() {
			if running.Load() {
				return
			}
			resetCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, pcAddressInput)
		}).
		AddButton("[red]Exit", func() {
			running.Store(false)
			app.Stop()
		}).
		AddButton("[white]Load Program", func() {
			programPathInput.SetText("")
			loadProgramForm.SetFocus(0)
			pages.SwitchToPage("loadProgram")
			app.SetFocus(loadProgramForm)
		}).
		SetButtonActivatedStyle(
			tcell.StyleDefault.
				Foreground(tcell.ColorBlack).
				Background(tcell.ColorYellow),
		)
	buttons.SetFocus(0)
	buttons.SetBorder(true).SetTitle("Commands")

	grid := tview.NewGrid().
		SetRows(7, 0, 3).
		SetColumns(75, 32, -4).
		SetBorders(true).
		AddItem(buttons, 0, 0, 1, 3, 0, 0, true).
		AddItem(status, 2, 0, 1, 3, 0, 0, false)

	// Layout for screens narrower than 100 cells (processorMemory and stack are hidden).
	grid.AddItem(processorMemory, 0, 0, 0, 0, 0, 0, false).
		AddItem(registers, 1, 0, 1, 3, 0, 0, false).
		AddItem(stack, 0, 0, 0, 0, 0, 0, false)

	// Layout for screens wider than 100 cells.
	grid.AddItem(processorMemory, 1, 0, 1, 1, 0, 100, false).
		AddItem(registers, 1, 1, 1, 1, 0, 100, false).
		AddItem(stack, 1, 2, 1, 1, 0, 100, false)

	pages.AddPage("main", grid, true, true).
		AddPage("loadProgram", loadProgramForm, true, false)

	refreshMemoryDump(processorMemory, mem, displayAddress)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)

	if err := app.SetRoot(pages, true).
		SetFocus(grid).
		EnableMouse(true).
		Run(); err != nil {
		log.Fatal(err)
	}
}

func refreshMemoryDump(view *tview.TextView, mem *memory.RAM, startAddress uint16) {
	const bytesPerRow = 16
	_, _, _, height := view.GetInnerRect()
	length := height * bytesPerRow
	if remaining := (1 << 16) - int(startAddress); length > remaining {
		length = remaining
	}

	view.Clear()
	memorydump(view, mem, startAddress, length)
}

func refreshStackDump(view *tview.TextView, mem *memory.RAM, cpu *z80.Z80, stackStartAddress uint16) {
	_, _, _, height := view.GetInnerRect()
	view.Clear()
	rowCount := height - 1
	if rowCount < 0 {
		rowCount = 0
	}
	dumpstack(view, mem, cpu.SP, stackStartAddress, rowCount)
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

func stepCPU(cpu *z80.Z80,
	app *tview.Application,
	status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	pcAddressInput *tview.InputField) {

	cycles := cpu.Step()
	updatePCAddressInput(pcAddressInput, cpu.PC)
	status.Clear()
	processorMemory.Clear()
	registers.Clear()
	fmt.Fprintln(status, fmt.Sprintf("Single Step, cycles: %d", cycles))
	refreshMemoryDump(processorMemory, mem, displayAddress)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)
}

func runCPU(cpu *z80.Z80, app *tview.Application, status *tview.TextView,
	processorMemory *tview.TextView, mem *memory.RAM,
	stack *tview.TextView,
	registers *tview.TextView, displayAddress, stackStartAddress uint16,
	buttons *tview.Form, running *atomic.Bool, pcAddressInput *tview.InputField) {
	cycles := 0
	lastRefresh := time.Now()
	for !cpu.Halted && running.Load() {
		cycles = cycles + cpu.Step()
		if time.Since(lastRefresh) >= 100*time.Millisecond {
			app.QueueUpdateDraw(func() {
				if !running.Load() {
					return
				}
				status.Clear()
				fmt.Fprintf(status, "Running, cycles: %d\n", cycles)
				processorMemory.Clear()
				registers.Clear()
				updatePCAddressInput(pcAddressInput, cpu.PC)
				refreshMemoryDump(processorMemory, mem, displayAddress)
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
		status.Clear()
		processorMemory.Clear()
		registers.Clear()
		updatePCAddressInput(pcAddressInput, cpu.PC)
		fmt.Fprintf(status, "Halted, cycles: %d\n", cycles)
		refreshMemoryDump(processorMemory, mem, displayAddress)
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
	status.Clear()
	registers.Clear()
	processorMemory.Clear()
	refreshMemoryDump(processorMemory, mem, displayAddress)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)
	fmt.Fprintln(status, "CPU reset; memory cleared.")
}
