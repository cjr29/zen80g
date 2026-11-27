; test1
; 
;
; Constants
IHDLR_UNKNOWN:                  EQU 0x00
IHDLR_TERMINAL_VECTOR:          EQU 0x00
MONITOR_ROM:                    EQU 0x0000
INTERRUPT_VECTOR_TABLE:         EQU 0X0100
INTERRUPT_VECTOR_TABLE_MSB:     EQU 0x01
TERMINAL_PORT                   EQU 0x00
NULL:                           EQU 0x00
EOS:                            EQU 0x00
HASH:                           EQU '#'
NEWLINE:                        EQU 0x0A
RIGHT_ANGLE_BRACKET             EQU '>'
SPACE:                          EQU ' '
INBUFFER_SIZE                   EQU 80
IQSIZE                          EQU 10  ; input queue size, 10 words

;****************************************************
;********************** ENTRY ***********************
;****************************************************
        JP MAIN
; Skip space to IVT
DEFS 256-3

;****************************************************
;************** INTERRUPT VECTOR TABLE **************
; 0x0100 - 0x01ff reserved for interrupt vector table
; Interrupt Vector Table
; First 3 words - 2 byte address per vector on even boundary
DW IHDLR_TERMINAL       ; vector 0 - interrupt on keyboard Enter pressed
DW IHDLR_UNKNOWN        ; vector 2
DW IHDLR_UNKNOWN        ; vector 4
; Reserve remainder of block for IV table
DEFS 256-6

;****************************************************
;*************** SYSTEM MONITOR *********************
;****************************************************
; Monitor code starts here
INPUTFLAG: DB 0 ; 0: no input, non-zero: input

MAIN:   
        ; Iniitialize monitor
        DI      ; Disable int during initialization
        LD A, INTERRUPT_VECTOR_TABLE_MSB
        LD I, A ; Set Interrupt Vector to MSB of the Interrupt Vector Table address
        IM 2    ; Set interrupt mode to 2
        EI      ; Enable interrupts

; Main monitor loop, just enter a loop waiting for a caommand
WAIT:
        ; Watch for input commands, Disable interrupts while testing flag
        ; to avoid race condition where interrupt routine sets the flag after
        ; it is checked but before the main loop clears it causing
        ; notifications to be lost
        NOP
        NOP
        NOP
        NOP
        LD A, (INPUTFLAG)
        OR A                   ; Z=1 if INPUTFLAG is zero
        ;CALL NZ, PROCESS_LINE ; TBD - Need input queue before enabling
        LD A, 0
        LD (INPUTFLAG), A       ; Clear terminal data flag
        JP WAIT                 ; Keep waiting

;***********************************************************
;****** Interrupt Handler **********************************
;*** for TERMINAL_PORT
;***********************************************************
; Start Interrupt handler code
IHDLR_TERMINAL:
        DI
        PUSH AF
        CALL READ_LINE      ; Read the characters from the terminal
        CALL PRINT_LINE     ; Echo the line to the terminal
        POP AF
        EI
        RETI

;***********************************************************
;******** Subroutine: READ_LINE ***************************
;*** Reads chars from terminal buffer into INBUFFER
;*** Terminates string with EOS
;*** Max line size is 80 chars
;***********************************************************
INBUFFER: DEFS 81         ; 80 chars plus EOS for terminal input

READ_LINE:
        PUSH AF
        PUSH IX
        PUSH BC
        LD IX, INBUFFER
        LD   B,0

read_char:
        IN   A,(TERMINAL_PORT)
        CP   NEWLINE         ; LF?
        JR   Z,got_lf        ; Don't store the LF

        LD   (IX+0),A        ; Store character
        INC  IX              ; Advance write pointer
        INC  B               ; Increase character count

        LD   A,B
        CP   INBUFFER_SIZE
        JR   Z,buffer_full
        JR   read_char

got_lf:
        LD   (IX+0),EOS     ; Terminate string with EOS
        JR   done_read_line

buffer_full:
        LD   (IX-1),EOS     ; buffer full, terminate with EOS anyway

done_read_line:
        LD A, 0x01
        LD (INPUTFLAG), A      ; Set flag we have data
        POP BC
        POP IX
        POP AF
        RET                  ; INBUFFER contains string terminated by EOS

;**********************************************************
;****** Subroutine: PROCESS_LINE **************************
;* Retrieves and tests terminal lines *********************
;* If prefixed by '#', treats as Command
;* Else, just echos the line to the terminal
;**********************************************************
PROCESS_LINE:
        ; Test for monitor command, if so, call monitor handler
        ; otherwise no need to do anything
        PUSH AF
        PUSH IX
        PUSH BC
        LD IX, INBUFFER
        LD A, (IX+0) ; Get first character from input buffer
        ; Check for command (TBD)
        ; Consider implementing a jump table with RST p instructions
        ; to vector to a Command service routine

PROCESS_LINE_DONE:
        CALL PRINT_LINE
        POP BC
        POP IX
        POP AF
        RET


;***********************************************************
;******** Subroutine: PRINT_LINE ***************************
;*** IX: Input - points to buffer
;*** IY: points to outbut buffer
;*** Processes input string until EOS char
;*** Writes to TERMINAL_OUT and ends line with NL char
;***********************************************************
PRINT_LINE:
        PUSH AF
        PUSH IX
        LD IX, INBUFFER
L_PRINT_LINE:
        LD A, (IX+0)
        CP EOS         ; End of string?
        JP Z, DONE_PRINT_LINE
        OUT (TERMINAL_PORT), A
        INC IX
        JR L_PRINT_LINE         ; Loop
DONE_PRINT_LINE:                ; Finish by writing a NL to terminal
        LD A, NEWLINE
        OUT (TERMINAL_PORT), A
        POP IX
        POP AF
        RET

;***********************************************************
;************* Subroutine: PUSH_INPUT **********************
;*** Pushes address on input queue
;*** IY = Address to be pushed
;***********************************************************


;***********************************************************
;************* Subroutine: POP_INPUT **********************
;*** Pops address off input queue
;*** IY = returned address
;***********************************************************


;***********************************************************
;******** Input Queue ***************************
;*** Contains 2-byte pointers to the line in the Heap
;*** Queue grows low to high
;*** Subroutines manage pushing and popping
;*** addresses from the queue
;*** Limited to 10 lines (2-byte addresses)
;***********************************************************
IQPTR:          DW      INPUT_QUEUE     ; Initialized to poiint to input queue
IQCNT:          DB      0               ; Count of items in queue
INPUT_QUEUE:    DEFS    2 * IQSIZE      ; limit to 10 addresses



;***********************************************************
;******** String Heap ***************************
;*** All strings are terminated by EOS
;*** Heap grows continuously
;*** Strings are all 80 chars + EOS
;*** Used for queuing input lines from terminal
;*** The heap is sized to 10 items max
;***********************************************************
STRING_HEAP: DEFS (INBUFFER_SIZE + 1) * IQSIZE