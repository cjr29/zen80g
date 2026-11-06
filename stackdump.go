package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/ha1tch/zen80/memory"
)

func dumpstack(w io.Writer, mem *memory.RAM, stackPointer, stackStart uint16, rowCount int) {
	const bytesPerRow = 8

	fmt.Fprintf(w, "SP: %04x", stackPointer)
	for row := 0; row < rowCount; row++ {
		rowAddress := int(stackStart) - row*bytesPerRow
		if rowAddress < 0 {
			break
		}
		fmt.Fprintln(w)

		highlight := int(stackPointer) <= rowAddress && int(stackPointer) >= rowAddress-bytesPerRow+1
		if highlight {
			fmt.Fprint(w, "[yellow]")
		}
		fmt.Fprintf(w, "%04x  ", rowAddress)
		var values strings.Builder
		for i := 0; i < bytesPerRow; i++ {
			if i > 0 {
				values.WriteByte(' ')
			}
			address := uint16(rowAddress - i)
			fmt.Fprintf(&values, "%02x", mem.Read(address))
		}
		fmt.Fprint(w, values.String())
		if highlight {
			fmt.Fprint(w, "[-]")
		}
	}
}
