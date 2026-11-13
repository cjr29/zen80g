; SectorZ - A Tiny C Compiler in Z80 Assembly
; Compiles "Barely C" source to Z80 machine code
; Target: retro-z80-emulator (MC6850 ACIA on ports $80/$81)
;
; Assemble with: z80asm -o sectorz.bin sectorz.asm

; ============================================================
; Constants
; ============================================================

tramp_base:	equ	$D000	; Function trampoline table (64 entries x 4)
var_base_hi:	equ	$D1	; High byte of variable storage base
semi_buf:	equ	$D200	; Semicolon pushback flag (1 byte)
num_flag:	equ	$D201	; Nonzero if current token is a number
eof_flag:	equ	$D202	; Nonzero if EOF detected
code_start:	equ	$D300	; Generated code output area
stack_top:	equ	$D000	; Stack pointer initial value

; ACIA ports (MC6850)
acia_stat:	equ	$80	; Status register
acia_data:	equ	$81	; Data register

; Token hash values
tok_int:	equ	11716
tok_void:	equ	64714
tok_if:		equ	1152
tok_while:	equ	60497
tok_asm:	equ	10959
tok_rparen:	equ	41
tok_rbrace:	equ	125
tok_lparen:	equ	40
tok_star:	equ	42
tok_amp:	equ	38

; Main function trampoline address
; hash("main") = 54324 = $D434, low byte $34, AND $FC = $34
main_tramp:	equ	tramp_base + $34

	org	$0000

; ============================================================
; Entry Point
; ============================================================

entry:
	ld	sp, stack_top
	ld	hl, code_start		; HL = code output pointer
	call	compile
	call	main_tramp		; Run main() via trampoline
	halt

; ============================================================
; compile - Parse top-level declarations (int / void)
; ============================================================

compile:
	call	tok_next
	call	cp_de_imm
	dw	tok_int
	jr	z, do_int
	call	cp_de_imm
	dw	tok_void
	jr	z, do_void
	ret				; Unknown/EOF -> done

do_int:
	; int varname ;
	call	tok_next		; Read variable name (discard)
	call	tok_next		; Read ";" (discard)
	jr	compile

do_void:
	; void funcname ( ) { stmts }
	call	tok_next		; Read function name -> DE
	; Set up trampoline: JP to current code position
	call	func_addr		; DE = trampoline address
	push	de			; Save trampoline addr
	ld	a, $C3			; JP opcode
	ld	(de), a
	inc	de
	ld	a, l
	ld	(de), a
	inc	de
	ld	a, h
	ld	(de), a
	pop	de			; Clean stack
	call	tok_next		; Read "("
	call	tok_next		; Read ")"
	call	tok_next		; Read "{"
	call	compile_stmts
	; Emit RET at end of function body
	ld	(hl), $C9
	inc	hl
	jr	compile

; ============================================================
; compile_stmts - Parse statements until "}"
; ============================================================

compile_stmts:
	call	tok_next
	; Check EOF
	ld	a, d
	or	e
	ret	z
	; Check "}"
	call	cp_de_imm
	dw	tok_rbrace
	ret	z
	; Check "asm"
	call	cp_de_imm
	dw	tok_asm
	jr	z, do_asm
	; Check "if"
	call	cp_de_imm
	dw	tok_if
	jr	z, do_if
	; Check "while"
	call	cp_de_imm
	dw	tok_while
	jr	z, do_while
	; Check "*" (deref assign)
	call	cp_de_imm
	dw	tok_star
	jr	z, do_deref_assign
	; Default: identifier - could be call or assignment
	push	de			; Save name token
	call	tok_next		; Read "(" or "="
	call	cp_de_imm
	dw	tok_lparen
	jr	z, do_call
	; Fall through to do_assign (we just consumed "=")

; ============================================================
; do_assign - variable = expr ;
; ============================================================

do_assign:
	; Stack: [name_token, ...]
	call	compile_expr		; Compile RHS (consumes ";")
	pop	de			; Get variable name
	ld	a, $22			; LD (addr),HL opcode
	call	emit_var
	jp	compile_stmts

; ============================================================
; do_call - funcname ( ) ;
; ============================================================

do_call:
	pop	de			; Get function name
	call	func_addr		; DE = trampoline addr
	ld	a, $CD			; CALL opcode
	call	emit3
	call	tok_next		; Read ")"
	call	tok_next		; Read ";"
	jp	compile_stmts

; ============================================================
; do_asm - asm ( byte byte ... ) ;
; ============================================================

do_asm:
	call	tok_next		; Read "("
do_asm_loop:
	call	tok_next		; Read value or ")"
	call	cp_de_imm
	dw	tok_rparen
	jr	z, do_asm_done
	ld	(hl), e			; Emit low byte
	inc	hl
	jr	do_asm_loop
do_asm_done:
	call	tok_next		; Read ";"
	jp	compile_stmts

; ============================================================
; do_if - if ( expr ) { stmts }
; ============================================================

do_if:
	call	tok_next		; Read "("
	call	compile_expr		; Compile condition (consumes ")")
	; Emit: LD A,H / OR L / JP Z,????
	call	emit_test
	push	hl			; Save patch address
	inc	hl
	inc	hl
	call	tok_next		; Read "{"
	call	compile_stmts		; Compile body (consumes "}")
	; Patch forward jump to here
	pop	de
	ld	a, l
	ld	(de), a
	inc	de
	ld	a, h
	ld	(de), a
	jp	compile_stmts

; ============================================================
; do_while - while ( expr ) { stmts }
; ============================================================

do_while:
	call	tok_next		; Read "("
	push	hl			; Save loop-back address
	call	compile_expr		; Compile condition (consumes ")")
	; Emit: LD A,H / OR L / JP Z,????
	call	emit_test
	push	hl			; Save forward-patch address
	inc	hl
	inc	hl
	call	tok_next		; Read "{"
	call	compile_stmts		; Compile body (consumes "}")
	; Emit JP loop_back
	ld	(hl), $C3		; JP opcode
	inc	hl
	pop	de			; Forward-patch address
	pop	bc			; Loop-back address
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	; Patch forward jump to here
	ld	a, l
	ld	(de), a
	inc	de
	ld	a, h
	ld	(de), a
	jp	compile_stmts

; ============================================================
; do_deref_assign - * addr_expr = val_expr ;
; ============================================================

do_deref_assign:
	call	compile_expr		; Compile addr (consumes "=")
	; Emit PUSH HL
	ld	(hl), $E5
	inc	hl
	call	compile_expr		; Compile value (consumes ";")
	; Emit: POP DE / EX DE,HL / LD (HL),E / INC HL / LD (HL),D
	ld	(hl), $D1		; POP DE
	inc	hl
	ld	(hl), $EB		; EX DE,HL
	inc	hl
	ld	(hl), $73		; LD (HL),E
	inc	hl
	ld	(hl), $23		; INC HL
	inc	hl
	ld	(hl), $72		; LD (HL),D
	inc	hl
	jp	compile_stmts

; ============================================================
; emit_test - Emit condition test: LD A,H / OR L / JP Z,xxxx
;   Leaves HL pointing at the 2-byte address field to patch
; ============================================================

emit_test:
	ld	(hl), $7C		; LD A,H
	inc	hl
	ld	(hl), $B5		; OR L
	inc	hl
	ld	(hl), $CA		; JP Z
	inc	hl
	ret

; ============================================================
; compile_expr - Compile binary expression
;   Left-to-right, no precedence. Consumes trailing terminator.
; ============================================================

compile_expr:
	call	compile_unary
ce_op_loop:
	call	tok_next		; Read potential operator
	; Save code pointer, scan op table with HL
	push	hl
	ld	hl, op_table
ce_scan:
	ld	c, (hl)
	inc	hl
	ld	b, (hl)
	inc	hl
	; Check sentinel
	ld	a, b
	or	c
	jr	z, ce_not_op
	; Compare BC with DE
	ld	a, e
	cp	c
	jr	nz, ce_skip
	ld	a, d
	cp	b
	jr	z, ce_found
ce_skip:
	inc	hl			; Skip helper addr
	inc	hl
	jr	ce_scan

ce_found:
	; HL points to helper address in table
	ld	c, (hl)
	inc	hl
	ld	b, (hl)			; BC = helper address
	pop	hl			; Restore code pointer
	; Emit PUSH HL (save left operand in generated code)
	ld	(hl), $E5
	inc	hl
	push	bc			; Save helper address
	call	compile_unary		; Compile right operand
	pop	bc			; Restore helper address
	; Emit: POP DE / EX DE,HL / CALL helper
	ld	(hl), $D1		; POP DE
	inc	hl
	ld	(hl), $EB		; EX DE,HL
	inc	hl
	ld	(hl), $CD		; CALL
	inc	hl
	ld	(hl), c
	inc	hl
	ld	(hl), b
	inc	hl
	jr	ce_op_loop

ce_not_op:
	; Token was not an operator (terminator consumed)
	pop	hl			; Restore code pointer
	ret

; ============================================================
; compile_unary - Compile unary expression
;   Handles: ( expr ), *expr, &var, number, variable
; ============================================================

compile_unary:
	call	tok_next
	; Check "("
	call	cp_de_imm
	dw	tok_lparen
	jr	z, cu_paren
	; Check "*" (dereference)
	call	cp_de_imm
	dw	tok_star
	jr	z, cu_deref
	; Check "&" (address-of)
	call	cp_de_imm
	dw	tok_amp
	jr	z, cu_addr
	; Check if number
	ld	a, (num_flag)
	or	a
	jr	nz, cu_number
	; Variable: emit LD HL,(var_addr)
	ld	a, $2A
	jp	emit_var

cu_number:
	; Number: emit LD HL, value
	ld	a, $21
	jp	emit3

cu_paren:
	; ( subexpr ) - compile_expr consumes ")"
	jp	compile_expr

cu_deref:
	; * expr - dereference
	call	compile_unary
	; Emit: LD E,(HL) / INC HL / LD D,(HL) / EX DE,HL
	ld	(hl), $5E		; LD E,(HL)
	inc	hl
	ld	(hl), $23		; INC HL
	inc	hl
	ld	(hl), $56		; LD D,(HL)
	inc	hl
	ld	(hl), $EB		; EX DE,HL
	inc	hl
	ret

cu_addr:
	; & var - address of variable
	call	tok_next		; Read variable name
	; Compute var address in DE
	ld	a, e
	and	$FE
	ld	e, a
	ld	d, var_base_hi
	; Emit LD HL, var_addr
	ld	a, $21
	jp	emit3

; ============================================================
; emit_var - Compute variable address and emit 3-byte instruction
;   A = opcode, DE = token hash -> emits opcode + var_addr
; ============================================================

emit_var:
	push	af
	ld	a, e
	and	$FE
	ld	e, a
	ld	d, var_base_hi
	pop	af
	; Fall through to emit3

; ============================================================
; emit3 - Emit 3-byte instruction: A, E, D
; ============================================================

emit3:
	ld	(hl), a
	inc	hl
	ld	(hl), e
	inc	hl
	ld	(hl), d
	inc	hl
	ret

; ============================================================
; func_addr - Compute trampoline address for function
;   DE = token hash -> DE = trampoline address
; ============================================================

func_addr:
	ld	a, e
	and	$FC			; 64 entries x 4 bytes
	ld	e, a
	ld	d, tramp_base / 256
	ret

; ============================================================
; cp_de_imm - Compare DE with inline 16-bit immediate
;   Usage: CALL cp_de_imm / DW value / JR Z, match
;   Sets Z flag if equal. Preserves all registers except F.
; ============================================================

cp_de_imm:
	ex	(sp), hl		; HL = return addr (-> immediate), stack gets old HL
	ld	a, (hl)
	inc	hl
	cp	e
	jr	nz, cp_de_ne
	ld	a, (hl)
	cp	d
cp_de_ne:
	inc	hl			; Advance past high byte regardless
	ex	(sp), hl		; Restore HL, update return address
	ret

; ============================================================
; tok_next - Read next token, result in DE
;   Preserves HL (code pointer).
;   Sets num_flag nonzero if token is a number.
; ============================================================

tok_next:
	push	hl
	; Check semicolon buffer
	ld	a, (semi_buf)
	or	a
	jr	z, tn_no_semi
	xor	a
	ld	(semi_buf), a
	ld	de, 59			; ";"
	pop	hl
	ret

tn_no_semi:
	ld	de, 0
	xor	a
	ld	(num_flag), a		; Clear number flag
	ld	b, a			; B = 0 (hash mode)

	; Skip whitespace
tn_skip:
	call	getch
	or	a
	jr	z, tn_done		; EOF
	cp	33
	jr	c, tn_skip		; whitespace (< '!')

	; First char of token in A
	; Detect number (first char is '0'-'9')
	cp	'0'
	jr	c, tn_hash		; < '0'
	cp	'9' + 1
	jr	nc, tn_hash		; > '9'
	; Number mode
	ld	b, '0'			; Subtract '0' from each char
	push	af
	ld	a, 1
	ld	(num_flag), a
	pop	af

	; Hash/accumulate loop
tn_hash:
	cp	';'
	jr	z, tn_semi
	or	a
	jr	z, tn_done		; EOF mid-token
	cp	33
	jr	c, tn_done		; Whitespace = end of token

	; DE = DE * 10 + (A - B)
	sub	b			; A = char (or char - '0')
	push	af
	ex	de, hl			; HL = DE (accumulator)
	add	hl, hl			; *2
	ld	d, h
	ld	e, l			; DE = acc*2
	add	hl, hl			; *4
	add	hl, hl			; *8
	add	hl, de			; *10
	pop	af
	ld	e, a
	ld	d, 0
	add	hl, de			; + digit/char
	ex	de, hl			; DE = result

	call	getch
	jr	tn_hash

tn_semi:
	; Semicolon handling
	ld	a, d
	or	e
	jr	nz, tn_semi_buf
	; Semicolon IS the token (first char)
	ld	de, 59
	jr	tn_done
tn_semi_buf:
	; Buffer the semicolon, return current token
	ld	a, 1
	ld	(semi_buf), a

tn_done:
	pop	hl
	ret

; ============================================================
; getch - Read one character from ACIA serial port
;   Returns char in A. Returns 0 on EOF.
; ============================================================

getch:
	ld	a, (eof_flag)
	or	a
	jr	nz, getch_eof
getch_poll:
	in	a, (acia_stat)
	bit	0, a			; RDRF - data available?
	jr	z, getch_poll
	in	a, (acia_data)
	cp	$1A			; Ctrl-Z = EOF marker
	jr	z, getch_set_eof
	ret				; Normal character in A
getch_set_eof:
	ld	a, 1
	ld	(eof_flag), a
getch_eof:
	xor	a
	ret

; ============================================================
; Binary operator table
;   Each entry: DW token_hash, DW helper_address
;   Terminated by DW 0
; ============================================================

op_table:
	dw	43, rt_add		; +
	dw	45, rt_sub		; -
	dw	42, rt_mul		; *
	dw	38, rt_and		; &
	dw	124, rt_or		; |
	dw	94, rt_xor		; ^
	dw	660, rt_shl		; <<
	dw	682, rt_shr		; >>
	dw	671, rt_eq		; ==
	dw	391, rt_ne		; !=
	dw	60, rt_lt		; <
	dw	62, rt_gt		; >
	dw	661, rt_le		; <=
	dw	681, rt_ge		; >=
	dw	0			; Sentinel

; ============================================================
; Runtime helpers - Called by generated code for binary ops
;   HL = left operand, DE = right operand, result in HL
; ============================================================

rt_add:
	add	hl, de
	ret

rt_sub:
	or	a
	sbc	hl, de
	ret

rt_mul:
	; 16-bit multiply: HL = HL * DE
	push	bc
	ld	b, h
	ld	c, l			; BC = HL
	ld	hl, 0
rt_mul_lp:
	srl	d
	rr	e
	jr	nc, rt_mul_na
	add	hl, bc
rt_mul_na:
	ld	a, d
	or	e
	jr	z, rt_mul_dn
	sla	c
	rl	b
	jr	rt_mul_lp
rt_mul_dn:
	pop	bc
	ret

rt_and:
	ld	a, h
	and	d
	ld	h, a
	ld	a, l
	and	e
	ld	l, a
	ret

rt_or:
	ld	a, h
	or	d
	ld	h, a
	ld	a, l
	or	e
	ld	l, a
	ret

rt_xor:
	ld	a, h
	xor	d
	ld	h, a
	ld	a, l
	xor	e
	ld	l, a
	ret

rt_shl:
	ld	a, e
	or	a
	ret	z
rt_shl_lp:
	add	hl, hl
	dec	a
	jr	nz, rt_shl_lp
	ret

rt_shr:
	ld	a, e
	or	a
	ret	z
rt_shr_lp:
	srl	h
	rr	l
	dec	a
	jr	nz, rt_shr_lp
	ret

rt_gt:
	ex	de, hl
rt_lt:
	or	a
	sbc	hl, de
	ld	hl, 0
	ret	nc
	inc	hl
	ret

rt_le:
	ex	de, hl
rt_ge:
	or	a
	sbc	hl, de
	ld	hl, 0
	ret	c
	inc	hl
	ret

rt_eq:
	or	a
	sbc	hl, de
	ld	hl, 0
	ret	nz
	inc	hl
	ret

rt_ne:
	or	a
	sbc	hl, de
	ld	hl, 0
	ret	z
	inc	hl
	ret

compiler_end: