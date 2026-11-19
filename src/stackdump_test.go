package main

import (
	"strings"
	"testing"

	"github.com/ha1tch/zen80/memory"
)

func TestDumpstackDisplaysFromStackPointerToStackStart(t *testing.T) {
	mem := memory.NewRAM()
	mem.Write(0x000E, 0x34)
	mem.Write(0x000F, 0x12)
	mem.Write(0x0010, 0x78)
	mem.Write(0x0011, 0x56)

	var output strings.Builder
	dumpstack(&output, mem, 0x000E, 0x0012, 2)

	got := output.String()
	first := strings.Index(got, "000e  1234")
	second := strings.Index(got, "0010  5678")
	if first < 0 || second < 0 || first >= second {
		t.Fatalf("stack rows are not in ascending address order from SP: %q", got)
	}
}

func TestDumpstackDoesNotWrapWordAtHighestAddress(t *testing.T) {
	mem := memory.NewRAM()
	mem.Write(0xFFFD, 0x11)
	mem.Write(0xFFFE, 0x22)
	mem.Write(0xFFFF, 0xAB)
	mem.Write(0x0000, 0xEF)

	var output strings.Builder
	dumpstack(&output, mem, 0xFFFD, 0xFFFF, 2)

	if got, want := output.String(), "SP: fffd, [stack start: ffff]\n[yellow]fffd  2211[-]"; got != want {
		t.Fatalf("dumpstack() = %q, want %q", got, want)
	}
}
