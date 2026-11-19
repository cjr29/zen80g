package main

import (
	"testing"

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
