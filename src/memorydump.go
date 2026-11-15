package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/ha1tch/zen80/memory"
)

func memorydump(w io.Writer, mem *memory.RAM, startAddress uint16, length int) {
	const rowSize = 16

	for i := 0; i < length; i += rowSize {
		if i > 0 {
			fmt.Fprintln(w)
		}
		end := i + rowSize
		if end > length {
			end = length
		}

		var hexPart strings.Builder
		for j := i; j < end; j++ {
			if j > i {
				hexPart.WriteByte(' ')
			}
			b := mem.Read(uint16(uint32(startAddress) + uint32(j)))
			fmt.Fprintf(&hexPart, "%02x", b)
		}
		rowAddress := uint16(uint32(startAddress) + uint32(i))
		fmt.Fprintf(w, "%04x  %-47s  ", rowAddress, hexPart.String())

		for j := i; j < end; j++ {
			b := mem.Read(uint16(uint32(startAddress) + uint32(j)))
			if b >= 0x20 && b <= 0x7e {
				fmt.Fprintf(w, "%c", b)
			} else {
				fmt.Fprint(w, ".")
			}
		}
	}
}
