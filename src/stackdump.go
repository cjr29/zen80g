package main

import (
	"fmt"
	"io"

	"github.com/ha1tch/zen80/memory"
)

func dumpstack(w io.Writer, mem *memory.RAM, stackPointer, stackStart uint16, rowCount int) {
	const bytesPerWord = 2

	fmt.Fprintf(w, "SP: %04x, [stack start: %04x]", stackPointer, stackStart)
	for row := 0; row < rowCount; row++ {
		wordAddress := int(stackPointer) + row*bytesPerWord
		if wordAddress+bytesPerWord > int(stackStart) || wordAddress+bytesPerWord > 1<<16 {
			break
		}
		fmt.Fprintln(w)

		if row == 0 {
			fmt.Fprint(w, "[yellow]")
		}
		lowByte := mem.Read(uint16(wordAddress))
		highByte := mem.Read(uint16(wordAddress + 1))
		word := uint16(highByte)<<8 | uint16(lowByte)
		fmt.Fprintf(w, "%04x  %04x", wordAddress, word)
		if row == 0 {
			fmt.Fprint(w, "[-]")
		}
	}
}
