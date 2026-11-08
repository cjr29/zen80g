ORG     0x0000
    jp  main    ; Jump to main entry point

DATA:
    DEFW 0x0036
    DB "abcdefghijklmnopqrstuvwxyz0123456789", 0

Destination: EQU 0x0100

main:       ; Main program entry point
; Copy string from DATA to Destination
LD  HL, DATA + 2    ; Point to source (2 bytes beyound start of string)
LD  DE, Destination ; Point to destination
LD  BC, (DATA)      ; Get length of string (stored in first two bytes of data - little endian)
LDIR                ; Move BC bytes from (HL) to (DE)

HALT
