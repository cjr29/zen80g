        ORG 0x0000
        JP  main ; Jump to starting address

DATA:
        DB  "#"

dest:   EQU 0x0100  ; Destination address

main:       ; Main program entry point

; First, copy the DATA byte into 64 bytes beginning at Dest
; After each copy, push the B register value onto the stack
        LD  B,64 ; set loop counter to 64
        LD  C, 0
        LD  IX,dest ; load destination address minus 2 bytes to initialize address
        LD  A,(DATA) ; load A with value to copy

loop:
        LD  (IX),A ; store contents of A in address pointed to by IX + increment in D
        INC IX
        PUSH BC
        DJNZ loop ; decrement counter in B and loop

        HALT   ; done