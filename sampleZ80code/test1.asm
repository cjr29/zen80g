; test1
; Copies a "#" character 64 times into memory
; As it copies into memory, it also pushes onto the stack
; Result will be 64 bytes of "#" and 64 words on the stack
; since the stack operates on words
;
; Constants
IHDLR_UNKNOWN: EQU 0x00
IHDLR_TERMINAL_VECTOR: EQU 0x00
MONITOR_ROM:   EQU 0x0000
INTERRUPT_VECTOR_TABLE: EQU 0X0200
INTERRUPT_VECTOR_TABLE_MSB: EQU 0x02
TERMINAL_PORT   EQU 0x00

; 0x0000 - 0x01ff reserved for simulated ROM with System Monitor
DEFS 512
; 0x0200 - 0x2ff reserved for interrupt vector table
; Interrupt Vector Table
        ; First 3 words - 2 byte address per vector on even boundary
        DW IHDLR_TERMINAL       ; vector 0 - interrupt on keyboard Enter pressed
        DW IHDLR_UNKNOWN        ; vector 2
        DW IHDLR_UNKNOWN        ; vector 4
; Reserve remainder of block for IV table
DEFS 256-6

; Main code starts here
MAIN:       ; Main program entry point 0x300

        DI      ; Disable int during initialization
        LD A, INTERRUPT_VECTOR_TABLE_MSB
        LD I, A ; Set Interrupt Vector to MSB of the Interrupt Vector Table address
        IM 2    ; Set interrupt mode to 2
        EI      ; Enable interrupts

; For debugging interrupts, just enter a tight loop waiting for an IM2
WAIT:   NOP
        NOP
        NOP
        NOP
        NOP
        JP WAIT

; Start Interrupt handler code
IHDLR_TERMINAL:
        DI
        PUSH AF
        PUSH DE 
        LD DE, (INT_COUNT)     
        INC DE          ; increment interrupt count
        LD (INT_COUNT), DE
        CALL PROCESS_LINE      ; Process the characters from the terminal
        POP DE
        POP AF
        EI
        RETI

; Data area - CONSTANTS
HASH:           EQU '#'
NEWLINE:        EQU 0x0A
RIGHT_ANGLE_BRACKET     EQU '>'
INT_COUNT:      DW 0x0000       ; var - stores count of interrupts

; Subroutine that retrieves and tests terminal lines
PROCESS_LINE:
        IN A,(TERMINAL_PORT)        ; get next available character from terminal port 0
        ; Test for monitor command, if so, call monitor handler
        ; otherwise just echo the character until NL
        CP HASH         ; Is this a monitor command line?
        JP Z, MON_CMD   ; Handle as monitor cmd
        ; Not a monitor command, keep reading chars
        ; If this was a NL, we are done, return from interrupt
        CP NEWLINE
        JP Z, END_TERMIO_IH
        ; Not end of terminal line, keep reading chars
        OUT (TERMINAL_PORT),A
        JP FINISH_LINE

MON_CMD:
        Call MONITOR_CMD_HNDLR   ; Handle monitor command
        JP END_TERMIO_IH     ; Return to interrupt handler
; MONITOR_CMD_HNDLR
MONITOR_CMD_HNDLR:
        PUSH AF
        LD A, RIGHT_ANGLE_BRACKET
        OUT (TERMINAL_PORT), A    ; Write a monitor line start symbol
        POP AF
FINISH_LINE:    ; process remaining chars from terminal     
        IN A, (TERMINAL_PORT)       ; Get next char
        OUT (TERMINAL_PORT), A
        CP A, NEWLINE    ; If line feed, done
        JP NZ, FINISH_LINE           ; Not end, keep reading
END_TERMIO_IH:
        OUT (TERMINAL_PORT), A ; output the NL and return
        RET