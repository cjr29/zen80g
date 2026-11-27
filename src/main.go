package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

const (
	// The address of the interrupt service routine for the demo program.
	interruptVectorTable = 0xEE00
	// The interrupt vector for the demo program.
	vector = 0x00
	gray   = tcell.ColorGray

	stepButtonIndex  = 0
	pauseButtonIndex = 2
	exitButtonIndex  = 4
)

var buttons *tview.Form
var running atomic.Bool
var pcAddressInput *tview.InputField

func main() {
	/****************************************************************/
	/***********      Set up Z80 emulator    ************************/
	/****************************************************************/
	programPath := flag.String("program", "", "path to a raw Z80 binary to run")
	flag.Parse()

	// Create memory and I/O
	mem := memory.NewRAM()
	terminalController := newInterruptController()
	cpu := z80.New(mem, terminalController)
	// Set Interrupt Mode to Mode 2, and Interrupt Vector to 0xE0 (default)
	cpu.IM = 2
	// Set the terminal's interrupt vector and install its handler address in the Z80's Mode 2 vector table.
	terminalController.SetMode2Handler(mem, cpu.I, vector, interruptVectorTable)

	// Load a default program for demo purposes. This program adds 5 and 3 and halts.
	program := []uint8{
		0x3E, 0x05, // LD A, 5
		0x06, 0x03, // LD B, 3
		0x80, // ADD A, B
		0x76, // HALT
	}

	// Load program from file if specified as option. If not, use the default program above.
	startupStatus := ""
	if *programPath != "" {
		var err error
		program, err = loadProgramFromFile(*programPath)
		if err != nil {
			log.Fatal(err)
		}
		startupStatus = fmt.Sprintf("Loaded program %q (%d bytes)", *programPath, len(program))
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
		SetScrollable(true)
	processorMemory.SetBorder(true).SetTitle("Memory")
	processorMemory.SetBackgroundColor(gray)

	registers := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetScrollable(false)
	registers.SetBorder(true).SetTitle("Registers")
	registers.SetBackgroundColor(gray)

	stack := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetDynamicColors(true).
		SetWrap(false).
		SetScrollable(true)
	stack.SetBorder(true).SetTitle("Stack")
	stack.SetBackgroundColor(gray)

	status := tview.NewTextView().
		SetTextAlign(tview.AlignLeft).
		SetScrollable(true)
	status.SetBackgroundColor(gray)
	terminalOutput := &terminalOutputWriter{status: status}
	terminalController.SetTerminalOutputHandler(terminalOutput.WriteCharacter)
	if startupStatus != "" {
		appendStatus(status, startupStatus)
	}

	terminalInput := tview.NewInputField().
		SetLabel("> ")
	terminalInput.SetLabelStyle(
		terminalInput.GetLabelStyle().Background(gray),
	)
	terminalInput.SetLabelColor(tcell.ColorYellow)
	terminalInput.SetBackgroundColor(gray)
	terminalInput.SetFieldBackgroundColor(gray)
	terminalInput.SetFieldTextColor(tcell.ColorWhite)
	terminalPane := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(status, 0, 1, false).
		AddItem(terminalInput, 1, 0, true)
	terminalPane.SetBorder(true).SetTitle("Terminal")
	terminalPane.SetBackgroundColor(gray)
	terminalInput.SetDoneFunc(func(key tcell.Key) {
		if key != tcell.KeyEnter {
			return
		}
		inputLine := terminalInput.GetText()
		terminalOutput.WriteInputLine(inputLine)
		terminalInput.SetText("")
		// Check for special commands
		command := strings.TrimSpace(inputLine)
		switch command {
		case "!!reset":
			resetCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, pcAddressInput)
		case "!!step":
			stepCPU(cpu, terminalController, terminalOutput, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, pcAddressInput)
		case "!!run":
			if !running.CompareAndSwap(false, true) {
				appendStatus(status, "CPU is already running.")
				return
			}
			appendStatus(status, fmt.Sprintf("Running from address %04X.", cpu.PC))
			setRunControlsEnabled(buttons, false)
			app.SetFocus(buttons.GetButton(pauseButtonIndex))
			go runCPU(cpu, terminalController, terminalOutput, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, buttons, &running, pcAddressInput)
		case "!!pause":
			if !running.CompareAndSwap(true, false) {
				appendStatus(status, "CPU is not running.")
				return
			}
		case "!!exit":
			running.Store(false)
			app.Stop()
		case "!!int":
			// Trigger the terminal's interrupt handler
			//cpu.INT = true
			appendStatus(status, fmt.Sprintf("Triggering interrupt at vector %02X.", vector))
		// case "#":
		// 	// Command to Z80 real-time monitor. For now, just echo the command to the status window.
		// 	appendStatus(status, fmt.Sprintf("Z80 Monitor Command: %s", inputLine))
		default:
			terminalController.QueueTerminalLine(inputLine)
		}
	})

	pages := tview.NewPages()

	addressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Load at %04X: ", startAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", startAddress)).
		SetPlaceholderTextColor(tcell.ColorWhite).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	addressInput.SetBackgroundColor(gray)
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
		appendStatus(status, fmt.Sprintf("Program load address set to %04X. Current program is unchanged.", startAddress))
	})

	// Create a form for loading a Z80 binary program from a file.
	programPathInput := tview.NewInputField().
		SetLabel("Z80 binary path: ").
		SetFieldWidth(60)
	programPathInput.SetBackgroundColor(gray)
	loadProgramForm := tview.NewForm().
		AddFormItem(programPathInput).
		AddButton("Load", func() {
			path := strings.TrimSpace(programPathInput.GetText())
			if path == "" {
				appendStatus(status, "Enter the path to a Z80 binary file.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			loadedProgram, err := loadProgramFromFile(path)
			if err != nil {
				appendStatus(status, err.Error())
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}
			if len(loadedProgram) > (1<<16)-int(startAddress) {
				appendStatus(status, fmt.Sprintf("Program does not fit in memory at address %04X.", startAddress))
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			processorMemory.Clear()
			program = loadedProgram
			cpu.Reset()
			registers.Clear()
			mem.Clear()
			mem.Load(startAddress, program)
			cpu.PC = startAddress
			refreshMemoryDump(processorMemory, mem)
			updatePCAddressInput(pcAddressInput, cpu.PC)
			registerdump(registers, cpu)
			appendStatus(status, fmt.Sprintf("Loaded %q at address %04X (%d bytes).", path, startAddress, len(program)))
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		})
	loadProgramForm.SetBorder(true).SetTitle("Load Z80 Program")
	loadProgramForm.SetBackgroundColor(gray)
	loadProgramForm.SetCancelFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(buttons)
	})

	displayAddressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Display at %04X: ", displayAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", displayAddress)).
		SetPlaceholderTextColor(tcell.ColorWhite).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	displayAddressInput.SetBackgroundColor(gray)
	var scrollMemoryToRow func(row int)
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
			displayAddressInput.SetText("")
			return
		}

		displayAddress = newAddress
		displayAddressInput.SetLabel(fmt.Sprintf("Display at %04X: ", displayAddress))
		displayAddressInput.SetPlaceholder(fmt.Sprintf("%04X", displayAddress))
		displayAddressInput.SetText("")
		refreshMemoryDump(processorMemory, mem)
		scrollMemoryToRow(int(displayAddress) / 16)
		appendStatus(status, fmt.Sprintf("Displaying memory from address %04X.", displayAddress))
	})

	scrollMemoryToRow = func(row int) {
		const memoryRows = 1 << 12
		_, _, _, height := processorMemory.GetInnerRect()
		if height < 1 {
			height = 1
		}
		maxRow := memoryRows - height
		if maxRow < 0 {
			maxRow = 0
		}
		if row < 0 {
			row = 0
		} else if row > maxRow {
			row = maxRow
		}

		_, column := processorMemory.GetScrollOffset()
		processorMemory.ScrollTo(row, column)
		displayAddress = uint16(row * 16)
		displayAddressInput.SetLabel(fmt.Sprintf("Display at %04X: ", displayAddress))
		displayAddressInput.SetPlaceholder(fmt.Sprintf("%04X", displayAddress))
		displayAddressInput.SetText("")
	}

	processorMemory.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		row, _ := processorMemory.GetScrollOffset()
		_, _, _, height := processorMemory.GetInnerRect()
		pageSize := height
		if pageSize < 1 {
			pageSize = 1
		}
		switch event.Key() {
		case tcell.KeyUp:
			row--
		case tcell.KeyDown:
			row++
		case tcell.KeyPgUp, tcell.KeyCtrlB:
			row -= pageSize
		case tcell.KeyPgDn, tcell.KeyCtrlF:
			row += pageSize
		case tcell.KeyHome:
			row = 0
		case tcell.KeyEnd:
			row = (1 << 12) - height
		case tcell.KeyRune:
			switch event.Rune() {
			case 'k':
				row--
			case 'j':
				row++
			case 'g':
				row = 0
			case 'G':
				row = (1 << 12) - height
			default:
				return event
			}
		default:
			return event
		}
		scrollMemoryToRow(row)
		return nil
	})
	processorMemory.SetMouseCapture(func(action tview.MouseAction, event *tcell.EventMouse) (tview.MouseAction, *tcell.EventMouse) {
		x, y := event.Position()
		if !processorMemory.InRect(x, y) {
			return action, event
		}
		row, _ := processorMemory.GetScrollOffset()
		switch action {
		case tview.MouseScrollUp:
			row--
		case tview.MouseScrollDown:
			row++
		default:
			return action, event
		}
		scrollMemoryToRow(row)
		return tview.MouseConsumed, nil
	})

	stackAddressInput := tview.NewInputField().
		SetLabel(fmt.Sprintf("Stack at %04X: ", stackStartAddress)).
		SetPlaceholder(fmt.Sprintf("%04X", stackStartAddress)).
		SetPlaceholderTextColor(tcell.ColorWhite).
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
		appendStatus(status, fmt.Sprintf("Stack start address set to %04X.", stackStartAddress))
	})

	pcAddressInput = tview.NewInputField().
		SetLabel(fmt.Sprintf("PC %04X: ", cpu.PC)).
		SetPlaceholder(fmt.Sprintf("%04X", cpu.PC)).
		SetPlaceholderTextColor(tcell.ColorWhite).
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
		appendStatus(status, fmt.Sprintf("Program counter set to %04X.", cpu.PC))
	})

	// Generate Buttons form with Step, Run, Reset, Exit, and Load Program buttons.
	buttons = tview.NewForm().
		SetHorizontal(true).
		SetButtonsAlign(tview.AlignLeft).
		SetButtonBackgroundColor(tcell.ColorGray).
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
			stepCPU(cpu, terminalController, terminalOutput, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, pcAddressInput)
		}).
		AddButton("[green]Run", func() {
			if !running.CompareAndSwap(false, true) {
				return
			}
			appendStatus(status, fmt.Sprintf("Running from address %04X.", cpu.PC))
			setRunControlsEnabled(buttons, false)
			app.SetFocus(buttons.GetButton(pauseButtonIndex))
			go runCPU(cpu, terminalController, terminalOutput, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, buttons, &running, pcAddressInput)
		}).
		AddButton("[yellow]Pause", func() {
			if !running.CompareAndSwap(true, false) {
				return
			}
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
				Foreground(tcell.ColorWhite).
				Background(tcell.ColorBlack),
		)
	buttons.SetFocus(0)
	buttons.SetBackgroundColor(gray)
	buttons.SetBorder(true).SetTitle("Commands")

	// Create tview grid layout for the main page. The grid has 3 rows and 3 columns.
	// The first row is for the buttons form, the second row is for the processor views,
	// and the third row is for the terminal pane. The first column is for the processor
	// memory text view, the second column is for the registers text view, and the third column
	// is for the stack text view.
	contentGrid := tview.NewGrid().
		SetRows(0).
		SetColumns(75, 32, -4).
		SetBorders(true).
		SetMinSize(15, 0)

	// Layout for screens narrower than 100 cells (processorMemory and stack are hidden).
	contentGrid.AddItem(processorMemory, 0, 0, 0, 0, 0, 0, false).
		AddItem(registers, 0, 0, 1, 3, 0, 0, false).
		AddItem(stack, 0, 0, 0, 0, 0, 0, false)

	// Layout for screens wide enough to show all three panes.
	contentGrid.AddItem(processorMemory, 0, 0, 1, 1, 0, 100, true).
		AddItem(registers, 0, 1, 1, 1, 0, 100, false).
		AddItem(stack, 0, 2, 1, 1, 0, 100, true)

	grid := tview.NewGrid().
		SetRows(7, 0, 15).
		SetColumns(75, 32, -4).
		SetBorders(true).
		AddItem(buttons, 0, 0, 1, 3, 0, 0, true).
		AddItem(contentGrid, 1, 0, 1, 3, 0, 0, false).
		AddItem(terminalPane, 2, 0, 1, 3, 0, 0, true)

	grid.SetBackgroundColor(gray)
	contentGrid.SetBackgroundColor(gray)

	pages.AddPage("main", grid, true, true).
		AddPage("loadProgram", loadProgramForm, true, false)

	refreshMemoryDump(processorMemory, mem)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)

	if err := app.SetRoot(pages, true).
		SetFocus(terminalInput).
		EnableMouse(true).
		Run(); err != nil {
		log.Fatal(err)
	}
}
