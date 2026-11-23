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
DEST:   EQU 0x4000  ; Destination address for data move tests

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

; First, copy the DATA byte into 64 bytes beginning at Dest
; After each copy, push the B register value onto the stack
COPY:   LD  B,36 ; set loop counter to 36 characters
        LD  HL,DEST ; load destination address
        LD  A,(HASH) ; load A with value to copy

COPY_LOOP:
        LD  (HL),A ; store contents of A in address pointed to by IX + increment in D
        INC HL
        DJNZ COPY_LOOP ; decrement counter in B and loop

        ; finished copy loop, now repeat forever for debugging interrupts
        JP COPY ; repeat forever the main loop to allow for testing of interrupt handler

        HALT   ; done - should never reach here. HALT as a precaution.

; Start Interrupt handler code
IHDLR_TERMINAL:
        DI
        PUSH AF
        PUSH DE 
        LD DE, (INT_COUNT)     
        IN A,(0)        ; get next available character from terminal port 0
        INC DE          ; increment interrupt count
        LD (INT_COUNT), DE
        ; Append char to input buffer for monitor program use
        OUT (0),A       ; write the character to port 0 (terminal display)
        POP AF
        EI
        RETI

; Data area - CONSTANTS
HASH:           DB "#"
DATA:           DB "abcdefghijklmnopqrstuvwxyz0123456789", 0
NEWLINE:        DB 0x0A
INT_COUNT:      DW 0x0000       ; var - stroes count of interrupts

; Subroutines
;
; CHECK_MONITOR_CMD
; Should this line be directed to the system monitor?
; Tets character in A for '#'
CHECK_MON_CMD:
        PUSH AF ; Save status for return
        CP 0x23 ; Is this a # sign?
        JP Z, MON_CMD ; Send to Monitor Command Handler
        POP AF
        RET     ; Not monitor command, return to interrupt handler
MON_CMD:
        Call MONITOR_CMD_HNDLR   ; Handle monitor command
        RET     ; Return to interrupt handler

; MONITOR_CMD_HNDLR
MONITOR_CMD_HNDLR:
        PUSH AF
        LD A, '#'
        OUT (0), A    ; Write a monitor line start symbol
 LOOP_READ_CMD:       OUT (0), A      ; Write the command letters
        IN A, (0)       ; Get next char
        OUT (0), A
        CP A, 0x0A    ; If line feed, done
        JP NZ, LOOP_READ_CMD           ; Not end, keep reading
        POP AF
        RET