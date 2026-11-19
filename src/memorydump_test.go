package main

import (
	"strings"
	"testing"

	"github.com/ha1tch/zen80/memory"
)

func TestMemorydumpCoversEntireAddressSpace(t *testing.T) {
	mem := memory.NewRAM()
	mem.Write(0x0000, 0x12)
	mem.Write(0xFFFF, 0x34)

	var output strings.Builder
	memorydump(&output, mem, 0, 1<<16)

	got := output.String()
	if !strings.HasPrefix(got, "0000  12 ") {
		t.Fatalf("memory dump does not begin at 0000: %q", got[:min(len(got), 32)])
	}
	lines := strings.Split(got, "\n")
	if len(lines) != 1<<12 || !strings.HasPrefix(lines[len(lines)-1], "fff0  ") {
		t.Fatalf("memory dump does not contain all 4096 rows through ffff")
	}
	lastLine := lines[len(lines)-1]
	if !strings.HasPrefix(lastLine, "fff0  ") || !strings.Contains(lastLine, "00 00 00 00 00 00 00 00 00 00 00 00 00 00 00 34") {
		t.Fatalf("final memory row does not include byte at ffff: %q", lastLine)
	}
}
