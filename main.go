package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync/atomic"

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
	cpu := z80.New(mem, io)

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
	addressInput := tview.NewInputField().
		SetLabel("Program start address (4 hex digits): ").
		SetPlaceholder(fmt.Sprintf("%04X", startAddress)).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	addressForm := tview.NewForm().
		AddFormItem(addressInput).
		AddButton("Load", func() {
			text := addressInput.GetText()
			address, err := strconv.ParseUint(text, 16, 16)
			if len(text) != 4 || err != nil {
				status.SetText("Address must be exactly four hexadecimal digits.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}
			if len(program) > (1<<16)-int(address) {
				status.SetText("Program does not fit in memory at that starting address.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			startAddress = uint16(address)
			displayAddress = startAddress
			mem.Clear()
			mem.Load(startAddress, program)
			cpu.Reset()
			cpu.PC = startAddress
			processorMemory.Clear()
			status.SetText(fmt.Sprintf("Program loaded at address %04X.", startAddress))
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		})
	addressForm.SetBorder(true).SetTitle("Set Starting Address")
	addressForm.SetCancelFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(buttons)
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
		SetLabel("Memory display address (4 hex digits): ").
		SetPlaceholder(fmt.Sprintf("%04X", displayAddress)).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	displayAddressForm := tview.NewForm().
		AddFormItem(displayAddressInput).
		AddButton("Display", func() {
			text := displayAddressInput.GetText()
			address, err := strconv.ParseUint(text, 16, 16)
			if len(text) != 4 || err != nil {
				status.SetText("Address must be exactly four hexadecimal digits.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			displayAddress = uint16(address)
			refreshMemoryDump(processorMemory, mem, displayAddress)
			status.SetText(fmt.Sprintf("Displaying memory from address %04X.", displayAddress))
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		})
	displayAddressForm.SetBorder(true).SetTitle("Set Memory Display Address")
	displayAddressForm.SetCancelFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(buttons)
	})

	stackAddressInput := tview.NewInputField().
		SetLabel("Stack start address (4 hex digits): ").
		SetPlaceholder(fmt.Sprintf("%04X", stackStartAddress)).
		SetFieldWidth(4).
		SetAcceptanceFunc(func(textToCheck string, _ rune) bool {
			return isHexAddressInput(textToCheck)
		})
	stackAddressForm := tview.NewForm().
		AddFormItem(stackAddressInput).
		AddButton("Set", func() {
			text := stackAddressInput.GetText()
			address, err := strconv.ParseUint(text, 16, 16)
			if len(text) != 4 || err != nil {
				status.SetText("Address must be exactly four hexadecimal digits.")
				pages.SwitchToPage("main")
				app.SetFocus(buttons)
				return
			}

			stackStartAddress = uint16(address)
			cpu.SP = stackStartAddress
			refreshStackDump(stack, mem, cpu, stackStartAddress)
			registers.Clear()
			registerdump(registers, cpu)
			status.SetText(fmt.Sprintf("Stack start address set to %04X.", stackStartAddress))
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		}).
		AddButton("Cancel", func() {
			pages.SwitchToPage("main")
			app.SetFocus(buttons)
		})
	stackAddressForm.SetBorder(true).SetTitle("Set Stack Start Address")
	stackAddressForm.SetCancelFunc(func() {
		pages.SwitchToPage("main")
		app.SetFocus(buttons)
	})

	// Two buttons inside one grid cell.
	buttons = tview.NewForm().
		SetHorizontal(true).
		SetButtonsAlign(tview.AlignLeft).
		SetButtonBackgroundColor(tcell.ColorBlack).
		AddButton("[blue]Step", func() {
			if running.Load() {
				return
			}
			stepCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress)
		}).
		AddButton("[green]Run", func() {
			if !running.CompareAndSwap(false, true) {
				return
			}
			setRunControlsEnabled(buttons, false)
			app.SetFocus(buttons.GetButton(3))
			go runCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress, buttons, &running)
		}).
		AddButton("[yellow]Reset", func() {
			if running.Load() {
				return
			}
			resetCPU(cpu, app, status, processorMemory, mem, stack, registers, displayAddress, stackStartAddress)
		}).
		AddButton("[red]EXIT", func() {
			running.Store(false)
			app.Stop()
		}).
		AddButton("[white]Program Load Address", func() {
			addressInput.SetText("")
			addressInput.SetPlaceholder(fmt.Sprintf("%04X", startAddress))
			addressForm.SetFocus(0)
			pages.SwitchToPage("address")
			app.SetFocus(addressForm)
		}).
		AddButton("[white]Memory Display Address", func() {
			displayAddressInput.SetText("")
			displayAddressInput.SetPlaceholder(fmt.Sprintf("%04X", displayAddress))
			displayAddressForm.SetFocus(0)
			pages.SwitchToPage("displayAddress")
			app.SetFocus(displayAddressForm)
		}).
		AddButton("[white]Load Program", func() {
			programPathInput.SetText("")
			loadProgramForm.SetFocus(0)
			pages.SwitchToPage("loadProgram")
			app.SetFocus(loadProgramForm)
		}).
		AddButton("[white]Stack Start Address", func() {
			stackAddressInput.SetText("")
			stackAddressInput.SetPlaceholder(fmt.Sprintf("%04X", stackStartAddress))
			stackAddressForm.SetFocus(0)
			pages.SwitchToPage("stackAddress")
			app.SetFocus(stackAddressForm)
		}).
		SetButtonActivatedStyle(
			tcell.StyleDefault.
				Foreground(tcell.ColorBlack).
				Background(tcell.ColorYellow),
		)
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
		AddPage("address", addressForm, true, false).
		AddPage("loadProgram", loadProgramForm, true, false).
		AddPage("displayAddress", displayAddressForm, true, false).
		AddPage("stackAddress", stackAddressForm, true, false)

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
	registers *tview.TextView, displayAddress, stackStartAddress uint16) {

	cycles := cpu.Step()
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
	buttons *tview.Form, running *atomic.Bool) {
	cycles := 0
	for !cpu.Halted && running.Load() {
		cycles = cpu.Step()
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
	registers *tview.TextView, displayAddress, stackStartAddress uint16) {
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
	cpu.NMI = false
	cpu.INT = false
	status.Clear()
	registers.Clear()
	refreshMemoryDump(processorMemory, mem, displayAddress)
	refreshStackDump(stack, mem, cpu, stackStartAddress)
	registerdump(registers, cpu)
	fmt.Fprintln(status, "CPU reset; memory cleared.")
}
