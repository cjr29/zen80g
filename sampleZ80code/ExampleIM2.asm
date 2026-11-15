    org     0000h
    jp      Start
    defs    253              ; pad to address 0100h

; The table occupies 0100h–01FFh.
; It must be in writable RAM because Start initializes it.

IM2Table:
    defs    256

Start:
    di
    ld      sp, 0FF00h ; use a safe stack area in your RAM map

; Initialize all 128 even-vector entries to DefaultISR.
; Each entry is a two-byte address.
    ld      hl, 0100h
    ld      de, DefaultISR
    ld      b,  128

FillTable:
    ld      (hl),   e ; low byte of handler address
    inc     hl
    ld      (hl),   d ; high byte
    inc     hl
    djnz    FillTable

; Replace the entries for vectors 06h and 08h.
; Vector 06h indexes table bytes 0106h–0107h.
; Vector 08h indexes table bytes 0108h–0109h.
    ld      hl, TimerISR
    ld      (0106h),    hl

    ld      hl, SerialISR
    ld      (0108h),    hl

; I supplies the high byte of the table's page.
    ld      a,  01h
    ld      i,  a
    im      2
    ei

MainLoop:
    halt ; wait for an interrupt
    jr      MainLoop

; Example handler: set a timer flag at 4000h.
TimerISR:
    push    af
    ld      a,  (4000h)
    inc     a
    ld      (4000h),    a
; Acknowledge/clear the timer device here, if required.
    pop     af
    ei
    reti

; Example handler: set a serial flag at 4001h.
SerialISR:
    push    af
    ld      a,  (4001h)
    inc     a
    ld      (4001h),    a
; Read/acknowledge the serial device here, if required.
    pop     af
    ei
    reti

; Trap unexpected vectors rather than returning to an unhandled source.
DefaultISR:
    di
DefaultLoop:
    jr      DefaultLoop