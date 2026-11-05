package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/ha1tch/zen80/memory"
)

func dumpstack(w io.Writer, mem *memory.RAM, stackPointer uint16, length int) {
	const bytesPerRow = 8

	for offset := 0; offset < length; offset += bytesPerRow {
		if offset > 0 {
			fmt.Fprintln(w)
		}

		end := offset + bytesPerRow
		if end > length {
			end = length
		}

		var values strings.Builder
		for i := offset; i < end; i++ {
			if i > offset {
				values.WriteByte(' ')
			}
			address := uint16(uint32(stackPointer) + uint32(i))
			fmt.Fprintf(&values, "%02x", mem.Read(address))
		}
		fmt.Fprintf(w, "%04x  %s", uint16(uint32(stackPointer)+uint32(offset)), values.String())
	}
}
