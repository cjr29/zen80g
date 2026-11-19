package main

import (
	"fmt"
	"sync"

	zenio "github.com/ha1tch/zen80/io"
	"github.com/ha1tch/zen80/z80"
)

var _ z80.InterruptController = (*interruptController)(nil)

type interruptController struct {
	*zenio.MappedIO

	mu               sync.RWMutex
	interruptVector  uint8
	mode0Instruction []uint8
}

func newInterruptController() *interruptController {
	return &interruptController{
		MappedIO:         zenio.NewMappedIO(),
		interruptVector:  0xFF,
		mode0Instruction: []uint8{0xFF},
	}
}

func (c *interruptController) GetInterruptVector() uint8 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.interruptVector
}

func (c *interruptController) SetInterruptVector(vector uint8) {
	c.mu.Lock()
	c.interruptVector = vector
	c.mu.Unlock()
}

// SetMode2Handler sets the interrupting device's vector and installs its
// handler address in the Z80's Mode 2 vector table.
func (c *interruptController) SetMode2Handler(mem z80.MemoryInterface, iRegister, vector uint8, handlerAddress uint16) error {
	if mem == nil {
		return fmt.Errorf("Mode 2 interrupt memory is nil")
	}

	tableAddress := uint16(iRegister)<<8 | uint16(vector&0xFE)
	mem.Write(tableAddress, uint8(handlerAddress))
	mem.Write(tableAddress+1, uint8(handlerAddress>>8))
	c.SetInterruptVector(vector)
	return nil
}

func (c *interruptController) GetMode0Instruction() []uint8 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]uint8(nil), c.mode0Instruction...)
}

func (c *interruptController) SetMode0Instruction(instruction []uint8) error {
	if len(instruction) < 1 || len(instruction) > 4 {
		return fmt.Errorf("mode 0 interrupt instruction must contain 1 to 4 bytes, got %d", len(instruction))
	}

	c.mu.Lock()
	c.mode0Instruction = append([]uint8(nil), instruction...)
	c.mu.Unlock()
	return nil
}
