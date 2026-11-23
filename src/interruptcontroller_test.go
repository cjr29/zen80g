package main

import (
	"testing"

	"github.com/rivo/tview"

	"github.com/ha1tch/zen80/memory"
	"github.com/ha1tch/zen80/z80"
)

func TestInterruptControllerMode2Dispatch(t *testing.T) {
	mem := memory.NewRAM()
	controller := newInterruptController()
	const (
		iRegister     = 0xE0
		vector        = 0xE3
		handler       = 0x1234
		returnAddress = 0x4567
	)
	if err := controller.SetMode2Handler(mem, iRegister, vector, handler); err != nil {
		t.Fatal(err)
	}

	cpu := z80.New(mem, controller)
	cpu.IM = 2
	cpu.I = iRegister
	cpu.IFF1 = true
	cpu.IFF2 = true
	cpu.INT = true
	cpu.PC = returnAddress
	cpu.SP = 0xFFFF

	if cycles := cpu.Step(); cycles != z80.IM2_CYCLES {
		t.Fatalf("Step() cycles = %d, want %d", cycles, z80.IM2_CYCLES)
	}
	if cpu.PC != handler {
		t.Fatalf("PC = %#04x, want handler %#04x", cpu.PC, handler)
	}
	if cpu.SP != 0xFFFD {
		t.Fatalf("SP = %#04x, want %#04x", cpu.SP, 0xFFFD)
	}
	if got, want := mem.Read(cpu.SP), uint8(returnAddress&0xFF); got != want {
		t.Fatalf("stack low byte = %#02x, want %#02x", got, want)
	}
	if got, want := mem.Read(cpu.SP+1), uint8(returnAddress>>8); got != want {
		t.Fatalf("stack high byte = %#02x, want %#02x", got, want)
	}
}

func TestInterruptControllerMode2RequiresMemory(t *testing.T) {
	controller := newInterruptController()
	if err := controller.SetMode2Handler(nil, 0xE0, 0xFF, 0x1234); err == nil {
		t.Fatal("SetMode2Handler(nil, ...) returned nil error")
	}
}

func TestInterruptControllerReadsTerminalInputOneByteAtATime(t *testing.T) {
	controller := newInterruptController()
	controller.QueueTerminalLine("Hi")

	for _, want := range []uint8{'H', 'i', '\n'} {
		if got := controller.In(0x4100); got != want {
			t.Fatalf("In(%02X) = %02X, want %02X", terminalInputPort, got, want)
		}
	}
	if controller.HasTerminalInput() {
		t.Fatal("terminal input remains after all queued characters were read")
	}
	if got := controller.In(uint16(terminalInputPort)); got != 0 {
		t.Fatalf("empty terminal input read = %02X, want 00", got)
	}
	if got := controller.In(0x0001); got != 0xFF {
		t.Fatalf("unmapped port read = %02X, want FF", got)
	}
}

func TestMode2HandlerCanReadTerminalInput(t *testing.T) {
	mem := memory.NewRAM()
	controller := newInterruptController()
	const (
		iRegister     = 0x20
		vector        = 0x02
		handler       = 0x2100
		returnAddress = 0x0300
	)
	if err := controller.SetMode2Handler(mem, iRegister, vector, handler); err != nil {
		t.Fatal(err)
	}
	mem.Write(handler, 0xDB) // IN A,(0)
	mem.Write(handler+1, 0x00)
	controller.QueueTerminalLine("K")

	cpu := z80.New(mem, controller)
	cpu.IM = 2
	cpu.I = iRegister
	cpu.IFF1 = true
	cpu.IFF2 = true
	cpu.INT = true
	cpu.PC = returnAddress
	cpu.SP = 0xFFFF

	cpu.Step() // Accept the Mode 2 interrupt.
	if cpu.PC != handler {
		t.Fatalf("PC after interrupt = %#04x, want %#04x", cpu.PC, handler)
	}
	cpu.Step() // Execute IN A,(0).
	if cpu.A != 'K' {
		t.Fatalf("A after IN = %02X, want %02X", cpu.A, 'K')
	}
	if !controller.HasTerminalInput() {
		t.Fatal("newline terminator should still be buffered")
	}
}

func TestTerminalInputDrivesInterruptUntilBufferDrained(t *testing.T) {
	controller := newInterruptController()
	cpu := z80.New(memory.NewRAM(), controller)
	active := false

	updateTerminalInterrupt(cpu, controller, &active)
	if cpu.INT || active {
		t.Fatal("terminal input should not interrupt before Enter submits a line")
	}

	controller.QueueTerminalLine("A")
	updateTerminalInterrupt(cpu, controller, &active)
	if !cpu.INT || !active {
		t.Fatal("submitted terminal line should assert the interrupt")
	}
	controller.In(uint16(terminalInputPort))
	controller.In(uint16(terminalInputPort))
	updateTerminalInterrupt(cpu, controller, &active)
	if cpu.INT || active {
		t.Fatal("drained input should deassert the terminal interrupt")
	}
}

func TestZ80OutWritesAccumulatorToTerminal(t *testing.T) {
	mem := memory.NewRAM()
	controller := newInterruptController()
	status := tview.NewTextView()
	output := &terminalOutputWriter{status: status}
	controller.SetTerminalOutputHandler(output.WriteCharacter)
	mem.Load(0x0100, []uint8{0xD3, 0x00}) // OUT (0),A
	cpu := z80.New(mem, controller)
	cpu.PC = 0x0100
	cpu.A = 'Z'

	cpu.Step()
	if got, want := status.GetText(true), "Z"; got != want {
		t.Fatalf("terminal text = %q, want %q", got, want)
	}
}
