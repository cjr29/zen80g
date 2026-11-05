package main

import (
	"fmt"
	"io"

	"github.com/ha1tch/zen80/z80"
)

func registerdump(w io.Writer, cpu *z80.Z80) {
	fmt.Fprintf(w, 
		"A: %02x F: %02x     A': %02x F': %02x\n"+
		"B: %02x C: %02x     B': %02x C': %02x\n"+
		"D: %02x E: %02x     D': %02x E': %02x\n"+
		"H: %02x L: %02x     H': %02x L': %02x\n"+
		"\nIXH: %02x IXL: %02x\n"+
		"IYH: %02x IYL: %02x\n\n"+
		"SP: %04x PC: %04x\n",
		cpu.A, cpu.F, cpu.A_, cpu.F_, 
		cpu.B, cpu.C, cpu.B_, cpu.C_, 
		cpu.D, cpu.E, cpu.D_, cpu.E_, 
		cpu.H, cpu.L, cpu.H_, cpu.L_,
		cpu.IXH, cpu.IXL, 
		cpu.IYH, cpu.IYL, 
		cpu.SP, cpu.PC)
}
