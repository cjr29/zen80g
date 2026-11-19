; test1
; Copies a "#" character 64 times into memory
; As it copies into memory, it also pushes onto the stack
; Result will be 64 bytes of "#" and 64 words on the stack
; since the stack operates on words
;
; Constants
IHDLR_UNKNOWN: EQU 0x00
IHDLR_TERMINAL_VECTOR: EQU 0x02
MONITOR_ROM:   EQU 0x0000
INTERRUPT_VECTOR_TABLE: EQU 0X0200
INTERRUPT_VECTOR_TABLE_MSB: EQU 0x02
DEST1:   EQU 0x4000  ; Destination address for data move tests
DEST2:  EQU 0x5000      ; Destination for block move

; 0x0000 - 0x01ff reserved or simulated ROM with System Monitor
DEFS 512
; Interrupt Vector Table
        DW IHDLR_UNKNOWN        ; vector 0
        DW IHDLR_TERMINAL       ; vector 2 - interrupt on keyboard Enter pressed
        DW IHDLR_UNKNOWN        ; vector 4
; Reserve remainder of block for IV table
DEFS 256-6

; Main code starts here
MAIN:       ; Main program entry point 0x300

        LD A, INTERRUPT_VECTOR_TABLE_MSB
        LD I, A ; Set Interrupt Vector to MSB of the Interrupt Vector Table address
        EI      ; Enable interrupts

; First, copy the DATA byte into 64 bytes beginning at Dest
; After each copy, push the B register value onto the stack
        LD  B,64 ; set loop counter to 64
        LD  C, 0
        LD  IX,DEST1 ; load destination address minus 2 bytes to initialize address
        LD  A,(DATA_HASH) ; load A with value to copy

LOOP:
        LD  (IX),A ; store contents of A in address pointed to by IX + increment in D
        INC IX
        PUSH BC
        DJNZ LOOP ; decrement counter in B and loop

        JP MAIN ; repeat forever the main loop to allow for testing of interrupt handler

        HALT   ; done - should never reach here

; Start Interrupt handler code
IHDLR_TERMINAL:
        DI
        PUSH AF
        ; do stuff, for now let us know you got here by placing a 0xff into D register
        LD D, 0xff
        CALL MOVEDATA   ; Run a subroutine within the handler
        POP AF
        EI
        RETI

; Start Move data block subroutine
MOVEDATA:
        ; Copy string from DATA to Destination
        LD  HL, DATA + 2    ; Point to source (2 bytes beyond start of string)
        LD  DE, DEST2 ; Point to destination
        LD  BC, (DATA)      ; Get length of string (stored in first two bytes of data - little endian)
        LDIR                ; Move BC bytes from (HL) to (DE)
        RETI


; Data area
DATA_HASH:   DB "#"
DATA:
    DEFW 0x0035
    DB "abcdefghijklmnopqrstuvwxyz0123456789", 0