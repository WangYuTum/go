// Derived from Inferno utils/6c/txt.c
// http://code.google.com/p/inferno-os/source/browse/utils/6c/txt.c
//
//	Copyright © 1994-1999 Lucent Technologies Inc.  All rights reserved.
//	Portions Copyright © 1995-1997 C H Forsyth (forsyth@terzarima.net)
//	Portions Copyright © 1997-1999 Vita Nuova Limited
//	Portions Copyright © 2000-2007 Vita Nuova Holdings Limited (www.vitanuova.com)
//	Portions Copyright © 2004,2006 Bruce Ellis
//	Portions Copyright © 2005-2007 C H Forsyth (forsyth@terzarima.net)
//	Revisions Copyright © 2000-2007 Lucent Technologies Inc. and others
//	Portions Copyright © 2009 The Go Authors.  All rights reserved.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"cmd/internal/obj"
	"cmd/internal/obj/x86"
	"fmt"
)
import "cmd/internal/gc"

// TODO(rsc): Can make this bigger if we move
// the text segment up higher in 6l for all GOOS.
// At the same time, can raise StackBig in ../../runtime/stack.h.
var unmappedzero int64 = 4096

var resvd = []int{
	x86.REG_DI, // for movstring
	x86.REG_SI, // for movstring

	x86.REG_AX, // for divide
	x86.REG_CX, // for shift
	x86.REG_DX, // for divide
	x86.REG_SP, // for stack
}

func ginit() {
	var i int

	for i = 0; i < len(reg); i++ {
		reg[i] = 1
	}
	for i = x86.REG_AX; i <= x86.REG_R15; i++ {
		reg[i] = 0
	}
	for i = x86.REG_X0; i <= x86.REG_X15; i++ {
		reg[i] = 0
	}

	for i = 0; i < len(resvd); i++ {
		reg[resvd[i]]++
	}

	if gc.Nacl {
		reg[x86.REG_BP]++
		reg[x86.REG_R15]++
	} else if obj.Framepointer_enabled != 0 {
		// BP is part of the calling convention of framepointer_enabled.
		reg[x86.REG_BP]++
	}
}

func gclean() {
	var i int

	for i = 0; i < len(resvd); i++ {
		reg[resvd[i]]--
	}
	if gc.Nacl {
		reg[x86.REG_BP]--
		reg[x86.REG_R15]--
	} else if obj.Framepointer_enabled != 0 {
		reg[x86.REG_BP]--
	}

	for i = x86.REG_AX; i <= x86.REG_R15; i++ {
		if reg[i] != 0 {
			gc.Yyerror("reg %v left allocated\n", gc.Ctxt.Rconv(i))
		}
	}
	for i = x86.REG_X0; i <= x86.REG_X15; i++ {
		if reg[i] != 0 {
			gc.Yyerror("reg %v left allocated\n", gc.Ctxt.Rconv(i))
		}
	}
}

func anyregalloc() bool {
	var i int
	var j int

	for i = x86.REG_AX; i <= x86.REG_R15; i++ {
		if reg[i] == 0 {
			goto ok
		}
		for j = 0; j < len(resvd); j++ {
			if resvd[j] == i {
				goto ok
			}
		}
		return true
	ok:
	}

	return false
}

var regpc [x86.REG_R15 + 1 - x86.REG_AX]uint32

/*
 * allocate register of type t, leave in n.
 * if o != N, o is desired fixed register.
 * caller must regfree(n).
 */
func regalloc(n *gc.Node, t *gc.Type, o *gc.Node) {
	var i int
	var et int

	if t == nil {
		gc.Fatal("regalloc: t nil")
	}
	et = int(gc.Simtype[t.Etype])

	switch et {
	case gc.TINT8,
		gc.TUINT8,
		gc.TINT16,
		gc.TUINT16,
		gc.TINT32,
		gc.TUINT32,
		gc.TINT64,
		gc.TUINT64,
		gc.TPTR32,
		gc.TPTR64,
		gc.TBOOL:
		if o != nil && o.Op == gc.OREGISTER {
			i = int(o.Val.U.Reg)
			if i >= x86.REG_AX && i <= x86.REG_R15 {
				goto out
			}
		}

		for i = x86.REG_AX; i <= x86.REG_R15; i++ {
			if reg[i] == 0 {
				regpc[i-x86.REG_AX] = uint32(obj.Getcallerpc(&n))
				goto out
			}
		}

		gc.Flusherrors()
		for i = 0; i+x86.REG_AX <= x86.REG_R15; i++ {
			fmt.Printf("%d %p\n", i, regpc[i])
		}
		gc.Fatal("out of fixed registers")

	case gc.TFLOAT32,
		gc.TFLOAT64:
		if o != nil && o.Op == gc.OREGISTER {
			i = int(o.Val.U.Reg)
			if i >= x86.REG_X0 && i <= x86.REG_X15 {
				goto out
			}
		}

		for i = x86.REG_X0; i <= x86.REG_X15; i++ {
			if reg[i] == 0 {
				goto out
			}
		}
		gc.Fatal("out of floating registers")

	case gc.TCOMPLEX64,
		gc.TCOMPLEX128:
		gc.Tempname(n, t)
		return
	}

	gc.Fatal("regalloc: unknown type %v", gc.Tconv(t, 0))
	return

out:
	reg[i]++
	gc.Nodreg(n, t, i)
}

func regfree(n *gc.Node) {
	var i int

	if n.Op == gc.ONAME {
		return
	}
	if n.Op != gc.OREGISTER && n.Op != gc.OINDREG {
		gc.Fatal("regfree: not a register")
	}
	i = int(n.Val.U.Reg)
	if i == x86.REG_SP {
		return
	}
	if i < 0 || i >= len(reg) {
		gc.Fatal("regfree: reg out of range")
	}
	if reg[i] <= 0 {
		gc.Fatal("regfree: reg not allocated")
	}
	reg[i]--
	if reg[i] == 0 && x86.REG_AX <= i && i <= x86.REG_R15 {
		regpc[i-x86.REG_AX] = 0
	}
}

/*
 * generate
 *	as $c, reg
 */
func gconreg(as int, c int64, reg int) {
	var nr gc.Node

	switch as {
	case x86.AADDL,
		x86.AMOVL,
		x86.ALEAL:
		gc.Nodreg(&nr, gc.Types[gc.TINT32], reg)

	default:
		gc.Nodreg(&nr, gc.Types[gc.TINT64], reg)
	}

	ginscon(as, c, &nr)
}

/*
 * generate
 *	as $c, n
 */
func ginscon(as int, c int64, n2 *gc.Node) {
	var n1 gc.Node
	var ntmp gc.Node

	switch as {
	case x86.AADDL,
		x86.AMOVL,
		x86.ALEAL:
		gc.Nodconst(&n1, gc.Types[gc.TINT32], c)

	default:
		gc.Nodconst(&n1, gc.Types[gc.TINT64], c)
	}

	if as != x86.AMOVQ && (c < -(1<<31) || c >= 1<<31) {
		// cannot have 64-bit immediate in ADD, etc.
		// instead, MOV into register first.
		regalloc(&ntmp, gc.Types[gc.TINT64], nil)

		gins(x86.AMOVQ, &n1, &ntmp)
		gins(as, &ntmp, n2)
		regfree(&ntmp)
		return
	}

	gins(as, &n1, n2)
}

/*
 * set up nodes representing 2^63
 */
var bigi gc.Node

var bigf gc.Node

var bignodes_did int

func bignodes() {
	if bignodes_did != 0 {
		return
	}
	bignodes_did = 1

	gc.Nodconst(&bigi, gc.Types[gc.TUINT64], 1)
	gc.Mpshiftfix(bigi.Val.U.Xval, 63)

	bigf = bigi
	bigf.Type = gc.Types[gc.TFLOAT64]
	bigf.Val.Ctype = gc.CTFLT
	bigf.Val.U.Fval = new(gc.Mpflt)
	gc.Mpmovefixflt(bigf.Val.U.Fval, bigi.Val.U.Xval)
}

/*
 * generate move:
 *	t = f
 * hard part is conversions.
 */
func gmove(f *gc.Node, t *gc.Node) {
	var a int
	var ft int
	var tt int
	var cvt *gc.Type
	var r1 gc.Node
	var r2 gc.Node
	var r3 gc.Node
	var r4 gc.Node
	var zero gc.Node
	var one gc.Node
	var con gc.Node
	var p1 *obj.Prog
	var p2 *obj.Prog

	if gc.Debug['M'] != 0 {
		fmt.Printf("gmove %v -> %v\n", gc.Nconv(f, obj.FmtLong), gc.Nconv(t, obj.FmtLong))
	}

	ft = gc.Simsimtype(f.Type)
	tt = gc.Simsimtype(t.Type)
	cvt = t.Type

	if gc.Iscomplex[ft] != 0 || gc.Iscomplex[tt] != 0 {
		gc.Complexmove(f, t)
		return
	}

	// cannot have two memory operands
	if gc.Ismem(f) && gc.Ismem(t) {
		goto hard
	}

	// convert constant to desired type
	if f.Op == gc.OLITERAL {
		gc.Convconst(&con, t.Type, &f.Val)
		f = &con
		ft = tt // so big switch will choose a simple mov

		// some constants can't move directly to memory.
		if gc.Ismem(t) {
			// float constants come from memory.
			if gc.Isfloat[tt] != 0 {
				goto hard
			}

			// 64-bit immediates are really 32-bit sign-extended
			// unless moving into a register.
			if gc.Isint[tt] != 0 {
				if gc.Mpcmpfixfix(con.Val.U.Xval, gc.Minintval[gc.TINT32]) < 0 {
					goto hard
				}
				if gc.Mpcmpfixfix(con.Val.U.Xval, gc.Maxintval[gc.TINT32]) > 0 {
					goto hard
				}
			}
		}
	}

	// value -> value copy, only one memory operand.
	// figure out the instruction to use.
	// break out of switch for one-instruction gins.
	// goto rdst for "destination must be register".
	// goto hard for "convert to cvt type first".
	// otherwise handle and return.

	switch uint32(ft)<<16 | uint32(tt) {
	default:
		gc.Fatal("gmove %v -> %v", gc.Tconv(f.Type, obj.FmtLong), gc.Tconv(t.Type, obj.FmtLong))

		/*
		 * integer copy and truncate
		 */
	case gc.TINT8<<16 | gc.TINT8, // same size
		gc.TINT8<<16 | gc.TUINT8,
		gc.TUINT8<<16 | gc.TINT8,
		gc.TUINT8<<16 | gc.TUINT8,
		gc.TINT16<<16 | gc.TINT8,
		// truncate
		gc.TUINT16<<16 | gc.TINT8,
		gc.TINT32<<16 | gc.TINT8,
		gc.TUINT32<<16 | gc.TINT8,
		gc.TINT64<<16 | gc.TINT8,
		gc.TUINT64<<16 | gc.TINT8,
		gc.TINT16<<16 | gc.TUINT8,
		gc.TUINT16<<16 | gc.TUINT8,
		gc.TINT32<<16 | gc.TUINT8,
		gc.TUINT32<<16 | gc.TUINT8,
		gc.TINT64<<16 | gc.TUINT8,
		gc.TUINT64<<16 | gc.TUINT8:
		a = x86.AMOVB

	case gc.TINT16<<16 | gc.TINT16, // same size
		gc.TINT16<<16 | gc.TUINT16,
		gc.TUINT16<<16 | gc.TINT16,
		gc.TUINT16<<16 | gc.TUINT16,
		gc.TINT32<<16 | gc.TINT16,
		// truncate
		gc.TUINT32<<16 | gc.TINT16,
		gc.TINT64<<16 | gc.TINT16,
		gc.TUINT64<<16 | gc.TINT16,
		gc.TINT32<<16 | gc.TUINT16,
		gc.TUINT32<<16 | gc.TUINT16,
		gc.TINT64<<16 | gc.TUINT16,
		gc.TUINT64<<16 | gc.TUINT16:
		a = x86.AMOVW

	case gc.TINT32<<16 | gc.TINT32, // same size
		gc.TINT32<<16 | gc.TUINT32,
		gc.TUINT32<<16 | gc.TINT32,
		gc.TUINT32<<16 | gc.TUINT32:
		a = x86.AMOVL

	case gc.TINT64<<16 | gc.TINT32, // truncate
		gc.TUINT64<<16 | gc.TINT32,
		gc.TINT64<<16 | gc.TUINT32,
		gc.TUINT64<<16 | gc.TUINT32:
		a = x86.AMOVQL

	case gc.TINT64<<16 | gc.TINT64, // same size
		gc.TINT64<<16 | gc.TUINT64,
		gc.TUINT64<<16 | gc.TINT64,
		gc.TUINT64<<16 | gc.TUINT64:
		a = x86.AMOVQ

		/*
		 * integer up-conversions
		 */
	case gc.TINT8<<16 | gc.TINT16, // sign extend int8
		gc.TINT8<<16 | gc.TUINT16:
		a = x86.AMOVBWSX

		goto rdst

	case gc.TINT8<<16 | gc.TINT32,
		gc.TINT8<<16 | gc.TUINT32:
		a = x86.AMOVBLSX
		goto rdst

	case gc.TINT8<<16 | gc.TINT64,
		gc.TINT8<<16 | gc.TUINT64:
		a = x86.AMOVBQSX
		goto rdst

	case gc.TUINT8<<16 | gc.TINT16, // zero extend uint8
		gc.TUINT8<<16 | gc.TUINT16:
		a = x86.AMOVBWZX

		goto rdst

	case gc.TUINT8<<16 | gc.TINT32,
		gc.TUINT8<<16 | gc.TUINT32:
		a = x86.AMOVBLZX
		goto rdst

	case gc.TUINT8<<16 | gc.TINT64,
		gc.TUINT8<<16 | gc.TUINT64:
		a = x86.AMOVBQZX
		goto rdst

	case gc.TINT16<<16 | gc.TINT32, // sign extend int16
		gc.TINT16<<16 | gc.TUINT32:
		a = x86.AMOVWLSX

		goto rdst

	case gc.TINT16<<16 | gc.TINT64,
		gc.TINT16<<16 | gc.TUINT64:
		a = x86.AMOVWQSX
		goto rdst

	case gc.TUINT16<<16 | gc.TINT32, // zero extend uint16
		gc.TUINT16<<16 | gc.TUINT32:
		a = x86.AMOVWLZX

		goto rdst

	case gc.TUINT16<<16 | gc.TINT64,
		gc.TUINT16<<16 | gc.TUINT64:
		a = x86.AMOVWQZX
		goto rdst

	case gc.TINT32<<16 | gc.TINT64, // sign extend int32
		gc.TINT32<<16 | gc.TUINT64:
		a = x86.AMOVLQSX

		goto rdst

		// AMOVL into a register zeros the top of the register,
	// so this is not always necessary, but if we rely on AMOVL
	// the optimizer is almost certain to screw with us.
	case gc.TUINT32<<16 | gc.TINT64, // zero extend uint32
		gc.TUINT32<<16 | gc.TUINT64:
		a = x86.AMOVLQZX

		goto rdst

		/*
		* float to integer
		 */
	case gc.TFLOAT32<<16 | gc.TINT32:
		a = x86.ACVTTSS2SL

		goto rdst

	case gc.TFLOAT64<<16 | gc.TINT32:
		a = x86.ACVTTSD2SL
		goto rdst

	case gc.TFLOAT32<<16 | gc.TINT64:
		a = x86.ACVTTSS2SQ
		goto rdst

	case gc.TFLOAT64<<16 | gc.TINT64:
		a = x86.ACVTTSD2SQ
		goto rdst

		// convert via int32.
	case gc.TFLOAT32<<16 | gc.TINT16,
		gc.TFLOAT32<<16 | gc.TINT8,
		gc.TFLOAT32<<16 | gc.TUINT16,
		gc.TFLOAT32<<16 | gc.TUINT8,
		gc.TFLOAT64<<16 | gc.TINT16,
		gc.TFLOAT64<<16 | gc.TINT8,
		gc.TFLOAT64<<16 | gc.TUINT16,
		gc.TFLOAT64<<16 | gc.TUINT8:
		cvt = gc.Types[gc.TINT32]

		goto hard

		// convert via int64.
	case gc.TFLOAT32<<16 | gc.TUINT32,
		gc.TFLOAT64<<16 | gc.TUINT32:
		cvt = gc.Types[gc.TINT64]

		goto hard

		// algorithm is:
	//	if small enough, use native float64 -> int64 conversion.
	//	otherwise, subtract 2^63, convert, and add it back.
	case gc.TFLOAT32<<16 | gc.TUINT64,
		gc.TFLOAT64<<16 | gc.TUINT64:
		a = x86.ACVTTSS2SQ

		if ft == gc.TFLOAT64 {
			a = x86.ACVTTSD2SQ
		}
		bignodes()
		regalloc(&r1, gc.Types[ft], nil)
		regalloc(&r2, gc.Types[tt], t)
		regalloc(&r3, gc.Types[ft], nil)
		regalloc(&r4, gc.Types[tt], nil)
		gins(optoas(gc.OAS, f.Type), f, &r1)
		gins(optoas(gc.OCMP, f.Type), &bigf, &r1)
		p1 = gc.Gbranch(optoas(gc.OLE, f.Type), nil, +1)
		gins(a, &r1, &r2)
		p2 = gc.Gbranch(obj.AJMP, nil, 0)
		gc.Patch(p1, gc.Pc)
		gins(optoas(gc.OAS, f.Type), &bigf, &r3)
		gins(optoas(gc.OSUB, f.Type), &r3, &r1)
		gins(a, &r1, &r2)
		gins(x86.AMOVQ, &bigi, &r4)
		gins(x86.AXORQ, &r4, &r2)
		gc.Patch(p2, gc.Pc)
		gmove(&r2, t)
		regfree(&r4)
		regfree(&r3)
		regfree(&r2)
		regfree(&r1)
		return

		/*
		 * integer to float
		 */
	case gc.TINT32<<16 | gc.TFLOAT32:
		a = x86.ACVTSL2SS

		goto rdst

	case gc.TINT32<<16 | gc.TFLOAT64:
		a = x86.ACVTSL2SD
		goto rdst

	case gc.TINT64<<16 | gc.TFLOAT32:
		a = x86.ACVTSQ2SS
		goto rdst

	case gc.TINT64<<16 | gc.TFLOAT64:
		a = x86.ACVTSQ2SD
		goto rdst

		// convert via int32
	case gc.TINT16<<16 | gc.TFLOAT32,
		gc.TINT16<<16 | gc.TFLOAT64,
		gc.TINT8<<16 | gc.TFLOAT32,
		gc.TINT8<<16 | gc.TFLOAT64,
		gc.TUINT16<<16 | gc.TFLOAT32,
		gc.TUINT16<<16 | gc.TFLOAT64,
		gc.TUINT8<<16 | gc.TFLOAT32,
		gc.TUINT8<<16 | gc.TFLOAT64:
		cvt = gc.Types[gc.TINT32]

		goto hard

		// convert via int64.
	case gc.TUINT32<<16 | gc.TFLOAT32,
		gc.TUINT32<<16 | gc.TFLOAT64:
		cvt = gc.Types[gc.TINT64]

		goto hard

		// algorithm is:
	//	if small enough, use native int64 -> uint64 conversion.
	//	otherwise, halve (rounding to odd?), convert, and double.
	case gc.TUINT64<<16 | gc.TFLOAT32,
		gc.TUINT64<<16 | gc.TFLOAT64:
		a = x86.ACVTSQ2SS

		if tt == gc.TFLOAT64 {
			a = x86.ACVTSQ2SD
		}
		gc.Nodconst(&zero, gc.Types[gc.TUINT64], 0)
		gc.Nodconst(&one, gc.Types[gc.TUINT64], 1)
		regalloc(&r1, f.Type, f)
		regalloc(&r2, t.Type, t)
		regalloc(&r3, f.Type, nil)
		regalloc(&r4, f.Type, nil)
		gmove(f, &r1)
		gins(x86.ACMPQ, &r1, &zero)
		p1 = gc.Gbranch(x86.AJLT, nil, +1)
		gins(a, &r1, &r2)
		p2 = gc.Gbranch(obj.AJMP, nil, 0)
		gc.Patch(p1, gc.Pc)
		gmove(&r1, &r3)
		gins(x86.ASHRQ, &one, &r3)
		gmove(&r1, &r4)
		gins(x86.AANDL, &one, &r4)
		gins(x86.AORQ, &r4, &r3)
		gins(a, &r3, &r2)
		gins(optoas(gc.OADD, t.Type), &r2, &r2)
		gc.Patch(p2, gc.Pc)
		gmove(&r2, t)
		regfree(&r4)
		regfree(&r3)
		regfree(&r2)
		regfree(&r1)
		return

		/*
		 * float to float
		 */
	case gc.TFLOAT32<<16 | gc.TFLOAT32:
		a = x86.AMOVSS

	case gc.TFLOAT64<<16 | gc.TFLOAT64:
		a = x86.AMOVSD

	case gc.TFLOAT32<<16 | gc.TFLOAT64:
		a = x86.ACVTSS2SD
		goto rdst

	case gc.TFLOAT64<<16 | gc.TFLOAT32:
		a = x86.ACVTSD2SS
		goto rdst
	}

	gins(a, f, t)
	return

	// requires register destination
rdst:
	regalloc(&r1, t.Type, t)

	gins(a, f, &r1)
	gmove(&r1, t)
	regfree(&r1)
	return

	// requires register intermediate
hard:
	regalloc(&r1, cvt, t)

	gmove(f, &r1)
	gmove(&r1, t)
	regfree(&r1)
	return
}

func samaddr(f *gc.Node, t *gc.Node) bool {
	if f.Op != t.Op {
		return false
	}

	switch f.Op {
	case gc.OREGISTER:
		if f.Val.U.Reg != t.Val.U.Reg {
			break
		}
		return true
	}

	return false
}

/*
 * generate one instruction:
 *	as f, t
 */
func gins(as int, f *gc.Node, t *gc.Node) *obj.Prog {
	var w int32
	var p *obj.Prog
	var af obj.Addr
	//	Node nod;

	var at obj.Addr

	//	if(f != N && f->op == OINDEX) {
	//		regalloc(&nod, &regnode, Z);
	//		v = constnode.vconst;
	//		cgen(f->right, &nod);
	//		constnode.vconst = v;
	//		idx.reg = nod.reg;
	//		regfree(&nod);
	//	}
	//	if(t != N && t->op == OINDEX) {
	//		regalloc(&nod, &regnode, Z);
	//		v = constnode.vconst;
	//		cgen(t->right, &nod);
	//		constnode.vconst = v;
	//		idx.reg = nod.reg;
	//		regfree(&nod);
	//	}

	switch as {
	case x86.AMOVB,
		x86.AMOVW,
		x86.AMOVL,
		x86.AMOVQ,
		x86.AMOVSS,
		x86.AMOVSD:
		if f != nil && t != nil && samaddr(f, t) {
			return nil
		}

	case x86.ALEAQ:
		if f != nil && gc.Isconst(f, gc.CTNIL) {
			gc.Fatal("gins LEAQ nil %v", gc.Tconv(f.Type, 0))
		}
	}

	af = obj.Addr{}
	at = obj.Addr{}
	if f != nil {
		gc.Naddr(f, &af, 1)
	}
	if t != nil {
		gc.Naddr(t, &at, 1)
	}
	p = gc.Prog(as)
	if f != nil {
		p.From = af
	}
	if t != nil {
		p.To = at
	}
	if gc.Debug['g'] != 0 {
		fmt.Printf("%v\n", p)
	}

	w = 0
	switch as {
	case x86.AMOVB:
		w = 1

	case x86.AMOVW:
		w = 2

	case x86.AMOVL:
		w = 4

	case x86.AMOVQ:
		w = 8
	}

	if w != 0 && ((f != nil && af.Width < int64(w)) || (t != nil && at.Width > int64(w))) {
		gc.Dump("f", f)
		gc.Dump("t", t)
		gc.Fatal("bad width: %v (%d, %d)\n", p, af.Width, at.Width)
	}

	if p.To.Type == obj.TYPE_ADDR && w > 0 {
		gc.Fatal("bad use of addr: %v", p)
	}

	return p
}

func fixlargeoffset(n *gc.Node) {
	var a gc.Node

	if n == nil {
		return
	}
	if n.Op != gc.OINDREG {
		return
	}
	if n.Val.U.Reg == x86.REG_SP { // stack offset cannot be large
		return
	}
	if n.Xoffset != int64(int32(n.Xoffset)) {
		// offset too large, add to register instead.
		a = *n

		a.Op = gc.OREGISTER
		a.Type = gc.Types[gc.Tptr]
		a.Xoffset = 0
		gc.Cgen_checknil(&a)
		ginscon(optoas(gc.OADD, gc.Types[gc.Tptr]), n.Xoffset, &a)
		n.Xoffset = 0
	}
}

/*
 * return Axxx for Oxxx on type t.
 */
func optoas(op int, t *gc.Type) int {
	var a int

	if t == nil {
		gc.Fatal("optoas: t is nil")
	}

	a = obj.AXXX
	switch uint32(op)<<16 | uint32(gc.Simtype[t.Etype]) {
	default:
		gc.Fatal("optoas: no entry %v-%v", gc.Oconv(int(op), 0), gc.Tconv(t, 0))

	case gc.OADDR<<16 | gc.TPTR32:
		a = x86.ALEAL

	case gc.OADDR<<16 | gc.TPTR64:
		a = x86.ALEAQ

	case gc.OEQ<<16 | gc.TBOOL,
		gc.OEQ<<16 | gc.TINT8,
		gc.OEQ<<16 | gc.TUINT8,
		gc.OEQ<<16 | gc.TINT16,
		gc.OEQ<<16 | gc.TUINT16,
		gc.OEQ<<16 | gc.TINT32,
		gc.OEQ<<16 | gc.TUINT32,
		gc.OEQ<<16 | gc.TINT64,
		gc.OEQ<<16 | gc.TUINT64,
		gc.OEQ<<16 | gc.TPTR32,
		gc.OEQ<<16 | gc.TPTR64,
		gc.OEQ<<16 | gc.TFLOAT32,
		gc.OEQ<<16 | gc.TFLOAT64:
		a = x86.AJEQ

	case gc.ONE<<16 | gc.TBOOL,
		gc.ONE<<16 | gc.TINT8,
		gc.ONE<<16 | gc.TUINT8,
		gc.ONE<<16 | gc.TINT16,
		gc.ONE<<16 | gc.TUINT16,
		gc.ONE<<16 | gc.TINT32,
		gc.ONE<<16 | gc.TUINT32,
		gc.ONE<<16 | gc.TINT64,
		gc.ONE<<16 | gc.TUINT64,
		gc.ONE<<16 | gc.TPTR32,
		gc.ONE<<16 | gc.TPTR64,
		gc.ONE<<16 | gc.TFLOAT32,
		gc.ONE<<16 | gc.TFLOAT64:
		a = x86.AJNE

	case gc.OLT<<16 | gc.TINT8,
		gc.OLT<<16 | gc.TINT16,
		gc.OLT<<16 | gc.TINT32,
		gc.OLT<<16 | gc.TINT64:
		a = x86.AJLT

	case gc.OLT<<16 | gc.TUINT8,
		gc.OLT<<16 | gc.TUINT16,
		gc.OLT<<16 | gc.TUINT32,
		gc.OLT<<16 | gc.TUINT64:
		a = x86.AJCS

	case gc.OLE<<16 | gc.TINT8,
		gc.OLE<<16 | gc.TINT16,
		gc.OLE<<16 | gc.TINT32,
		gc.OLE<<16 | gc.TINT64:
		a = x86.AJLE

	case gc.OLE<<16 | gc.TUINT8,
		gc.OLE<<16 | gc.TUINT16,
		gc.OLE<<16 | gc.TUINT32,
		gc.OLE<<16 | gc.TUINT64:
		a = x86.AJLS

	case gc.OGT<<16 | gc.TINT8,
		gc.OGT<<16 | gc.TINT16,
		gc.OGT<<16 | gc.TINT32,
		gc.OGT<<16 | gc.TINT64:
		a = x86.AJGT

	case gc.OGT<<16 | gc.TUINT8,
		gc.OGT<<16 | gc.TUINT16,
		gc.OGT<<16 | gc.TUINT32,
		gc.OGT<<16 | gc.TUINT64,
		gc.OLT<<16 | gc.TFLOAT32,
		gc.OLT<<16 | gc.TFLOAT64:
		a = x86.AJHI

	case gc.OGE<<16 | gc.TINT8,
		gc.OGE<<16 | gc.TINT16,
		gc.OGE<<16 | gc.TINT32,
		gc.OGE<<16 | gc.TINT64:
		a = x86.AJGE

	case gc.OGE<<16 | gc.TUINT8,
		gc.OGE<<16 | gc.TUINT16,
		gc.OGE<<16 | gc.TUINT32,
		gc.OGE<<16 | gc.TUINT64,
		gc.OLE<<16 | gc.TFLOAT32,
		gc.OLE<<16 | gc.TFLOAT64:
		a = x86.AJCC

	case gc.OCMP<<16 | gc.TBOOL,
		gc.OCMP<<16 | gc.TINT8,
		gc.OCMP<<16 | gc.TUINT8:
		a = x86.ACMPB

	case gc.OCMP<<16 | gc.TINT16,
		gc.OCMP<<16 | gc.TUINT16:
		a = x86.ACMPW

	case gc.OCMP<<16 | gc.TINT32,
		gc.OCMP<<16 | gc.TUINT32,
		gc.OCMP<<16 | gc.TPTR32:
		a = x86.ACMPL

	case gc.OCMP<<16 | gc.TINT64,
		gc.OCMP<<16 | gc.TUINT64,
		gc.OCMP<<16 | gc.TPTR64:
		a = x86.ACMPQ

	case gc.OCMP<<16 | gc.TFLOAT32:
		a = x86.AUCOMISS

	case gc.OCMP<<16 | gc.TFLOAT64:
		a = x86.AUCOMISD

	case gc.OAS<<16 | gc.TBOOL,
		gc.OAS<<16 | gc.TINT8,
		gc.OAS<<16 | gc.TUINT8:
		a = x86.AMOVB

	case gc.OAS<<16 | gc.TINT16,
		gc.OAS<<16 | gc.TUINT16:
		a = x86.AMOVW

	case gc.OAS<<16 | gc.TINT32,
		gc.OAS<<16 | gc.TUINT32,
		gc.OAS<<16 | gc.TPTR32:
		a = x86.AMOVL

	case gc.OAS<<16 | gc.TINT64,
		gc.OAS<<16 | gc.TUINT64,
		gc.OAS<<16 | gc.TPTR64:
		a = x86.AMOVQ

	case gc.OAS<<16 | gc.TFLOAT32:
		a = x86.AMOVSS

	case gc.OAS<<16 | gc.TFLOAT64:
		a = x86.AMOVSD

	case gc.OADD<<16 | gc.TINT8,
		gc.OADD<<16 | gc.TUINT8:
		a = x86.AADDB

	case gc.OADD<<16 | gc.TINT16,
		gc.OADD<<16 | gc.TUINT16:
		a = x86.AADDW

	case gc.OADD<<16 | gc.TINT32,
		gc.OADD<<16 | gc.TUINT32,
		gc.OADD<<16 | gc.TPTR32:
		a = x86.AADDL

	case gc.OADD<<16 | gc.TINT64,
		gc.OADD<<16 | gc.TUINT64,
		gc.OADD<<16 | gc.TPTR64:
		a = x86.AADDQ

	case gc.OADD<<16 | gc.TFLOAT32:
		a = x86.AADDSS

	case gc.OADD<<16 | gc.TFLOAT64:
		a = x86.AADDSD

	case gc.OSUB<<16 | gc.TINT8,
		gc.OSUB<<16 | gc.TUINT8:
		a = x86.ASUBB

	case gc.OSUB<<16 | gc.TINT16,
		gc.OSUB<<16 | gc.TUINT16:
		a = x86.ASUBW

	case gc.OSUB<<16 | gc.TINT32,
		gc.OSUB<<16 | gc.TUINT32,
		gc.OSUB<<16 | gc.TPTR32:
		a = x86.ASUBL

	case gc.OSUB<<16 | gc.TINT64,
		gc.OSUB<<16 | gc.TUINT64,
		gc.OSUB<<16 | gc.TPTR64:
		a = x86.ASUBQ

	case gc.OSUB<<16 | gc.TFLOAT32:
		a = x86.ASUBSS

	case gc.OSUB<<16 | gc.TFLOAT64:
		a = x86.ASUBSD

	case gc.OINC<<16 | gc.TINT8,
		gc.OINC<<16 | gc.TUINT8:
		a = x86.AINCB

	case gc.OINC<<16 | gc.TINT16,
		gc.OINC<<16 | gc.TUINT16:
		a = x86.AINCW

	case gc.OINC<<16 | gc.TINT32,
		gc.OINC<<16 | gc.TUINT32,
		gc.OINC<<16 | gc.TPTR32:
		a = x86.AINCL

	case gc.OINC<<16 | gc.TINT64,
		gc.OINC<<16 | gc.TUINT64,
		gc.OINC<<16 | gc.TPTR64:
		a = x86.AINCQ

	case gc.ODEC<<16 | gc.TINT8,
		gc.ODEC<<16 | gc.TUINT8:
		a = x86.ADECB

	case gc.ODEC<<16 | gc.TINT16,
		gc.ODEC<<16 | gc.TUINT16:
		a = x86.ADECW

	case gc.ODEC<<16 | gc.TINT32,
		gc.ODEC<<16 | gc.TUINT32,
		gc.ODEC<<16 | gc.TPTR32:
		a = x86.ADECL

	case gc.ODEC<<16 | gc.TINT64,
		gc.ODEC<<16 | gc.TUINT64,
		gc.ODEC<<16 | gc.TPTR64:
		a = x86.ADECQ

	case gc.OMINUS<<16 | gc.TINT8,
		gc.OMINUS<<16 | gc.TUINT8:
		a = x86.ANEGB

	case gc.OMINUS<<16 | gc.TINT16,
		gc.OMINUS<<16 | gc.TUINT16:
		a = x86.ANEGW

	case gc.OMINUS<<16 | gc.TINT32,
		gc.OMINUS<<16 | gc.TUINT32,
		gc.OMINUS<<16 | gc.TPTR32:
		a = x86.ANEGL

	case gc.OMINUS<<16 | gc.TINT64,
		gc.OMINUS<<16 | gc.TUINT64,
		gc.OMINUS<<16 | gc.TPTR64:
		a = x86.ANEGQ

	case gc.OAND<<16 | gc.TINT8,
		gc.OAND<<16 | gc.TUINT8:
		a = x86.AANDB

	case gc.OAND<<16 | gc.TINT16,
		gc.OAND<<16 | gc.TUINT16:
		a = x86.AANDW

	case gc.OAND<<16 | gc.TINT32,
		gc.OAND<<16 | gc.TUINT32,
		gc.OAND<<16 | gc.TPTR32:
		a = x86.AANDL

	case gc.OAND<<16 | gc.TINT64,
		gc.OAND<<16 | gc.TUINT64,
		gc.OAND<<16 | gc.TPTR64:
		a = x86.AANDQ

	case gc.OOR<<16 | gc.TINT8,
		gc.OOR<<16 | gc.TUINT8:
		a = x86.AORB

	case gc.OOR<<16 | gc.TINT16,
		gc.OOR<<16 | gc.TUINT16:
		a = x86.AORW

	case gc.OOR<<16 | gc.TINT32,
		gc.OOR<<16 | gc.TUINT32,
		gc.OOR<<16 | gc.TPTR32:
		a = x86.AORL

	case gc.OOR<<16 | gc.TINT64,
		gc.OOR<<16 | gc.TUINT64,
		gc.OOR<<16 | gc.TPTR64:
		a = x86.AORQ

	case gc.OXOR<<16 | gc.TINT8,
		gc.OXOR<<16 | gc.TUINT8:
		a = x86.AXORB

	case gc.OXOR<<16 | gc.TINT16,
		gc.OXOR<<16 | gc.TUINT16:
		a = x86.AXORW

	case gc.OXOR<<16 | gc.TINT32,
		gc.OXOR<<16 | gc.TUINT32,
		gc.OXOR<<16 | gc.TPTR32:
		a = x86.AXORL

	case gc.OXOR<<16 | gc.TINT64,
		gc.OXOR<<16 | gc.TUINT64,
		gc.OXOR<<16 | gc.TPTR64:
		a = x86.AXORQ

	case gc.OLROT<<16 | gc.TINT8,
		gc.OLROT<<16 | gc.TUINT8:
		a = x86.AROLB

	case gc.OLROT<<16 | gc.TINT16,
		gc.OLROT<<16 | gc.TUINT16:
		a = x86.AROLW

	case gc.OLROT<<16 | gc.TINT32,
		gc.OLROT<<16 | gc.TUINT32,
		gc.OLROT<<16 | gc.TPTR32:
		a = x86.AROLL

	case gc.OLROT<<16 | gc.TINT64,
		gc.OLROT<<16 | gc.TUINT64,
		gc.OLROT<<16 | gc.TPTR64:
		a = x86.AROLQ

	case gc.OLSH<<16 | gc.TINT8,
		gc.OLSH<<16 | gc.TUINT8:
		a = x86.ASHLB

	case gc.OLSH<<16 | gc.TINT16,
		gc.OLSH<<16 | gc.TUINT16:
		a = x86.ASHLW

	case gc.OLSH<<16 | gc.TINT32,
		gc.OLSH<<16 | gc.TUINT32,
		gc.OLSH<<16 | gc.TPTR32:
		a = x86.ASHLL

	case gc.OLSH<<16 | gc.TINT64,
		gc.OLSH<<16 | gc.TUINT64,
		gc.OLSH<<16 | gc.TPTR64:
		a = x86.ASHLQ

	case gc.ORSH<<16 | gc.TUINT8:
		a = x86.ASHRB

	case gc.ORSH<<16 | gc.TUINT16:
		a = x86.ASHRW

	case gc.ORSH<<16 | gc.TUINT32,
		gc.ORSH<<16 | gc.TPTR32:
		a = x86.ASHRL

	case gc.ORSH<<16 | gc.TUINT64,
		gc.ORSH<<16 | gc.TPTR64:
		a = x86.ASHRQ

	case gc.ORSH<<16 | gc.TINT8:
		a = x86.ASARB

	case gc.ORSH<<16 | gc.TINT16:
		a = x86.ASARW

	case gc.ORSH<<16 | gc.TINT32:
		a = x86.ASARL

	case gc.ORSH<<16 | gc.TINT64:
		a = x86.ASARQ

	case gc.ORROTC<<16 | gc.TINT8,
		gc.ORROTC<<16 | gc.TUINT8:
		a = x86.ARCRB

	case gc.ORROTC<<16 | gc.TINT16,
		gc.ORROTC<<16 | gc.TUINT16:
		a = x86.ARCRW

	case gc.ORROTC<<16 | gc.TINT32,
		gc.ORROTC<<16 | gc.TUINT32:
		a = x86.ARCRL

	case gc.ORROTC<<16 | gc.TINT64,
		gc.ORROTC<<16 | gc.TUINT64:
		a = x86.ARCRQ

	case gc.OHMUL<<16 | gc.TINT8,
		gc.OMUL<<16 | gc.TINT8,
		gc.OMUL<<16 | gc.TUINT8:
		a = x86.AIMULB

	case gc.OHMUL<<16 | gc.TINT16,
		gc.OMUL<<16 | gc.TINT16,
		gc.OMUL<<16 | gc.TUINT16:
		a = x86.AIMULW

	case gc.OHMUL<<16 | gc.TINT32,
		gc.OMUL<<16 | gc.TINT32,
		gc.OMUL<<16 | gc.TUINT32,
		gc.OMUL<<16 | gc.TPTR32:
		a = x86.AIMULL

	case gc.OHMUL<<16 | gc.TINT64,
		gc.OMUL<<16 | gc.TINT64,
		gc.OMUL<<16 | gc.TUINT64,
		gc.OMUL<<16 | gc.TPTR64:
		a = x86.AIMULQ

	case gc.OHMUL<<16 | gc.TUINT8:
		a = x86.AMULB

	case gc.OHMUL<<16 | gc.TUINT16:
		a = x86.AMULW

	case gc.OHMUL<<16 | gc.TUINT32,
		gc.OHMUL<<16 | gc.TPTR32:
		a = x86.AMULL

	case gc.OHMUL<<16 | gc.TUINT64,
		gc.OHMUL<<16 | gc.TPTR64:
		a = x86.AMULQ

	case gc.OMUL<<16 | gc.TFLOAT32:
		a = x86.AMULSS

	case gc.OMUL<<16 | gc.TFLOAT64:
		a = x86.AMULSD

	case gc.ODIV<<16 | gc.TINT8,
		gc.OMOD<<16 | gc.TINT8:
		a = x86.AIDIVB

	case gc.ODIV<<16 | gc.TUINT8,
		gc.OMOD<<16 | gc.TUINT8:
		a = x86.ADIVB

	case gc.ODIV<<16 | gc.TINT16,
		gc.OMOD<<16 | gc.TINT16:
		a = x86.AIDIVW

	case gc.ODIV<<16 | gc.TUINT16,
		gc.OMOD<<16 | gc.TUINT16:
		a = x86.ADIVW

	case gc.ODIV<<16 | gc.TINT32,
		gc.OMOD<<16 | gc.TINT32:
		a = x86.AIDIVL

	case gc.ODIV<<16 | gc.TUINT32,
		gc.ODIV<<16 | gc.TPTR32,
		gc.OMOD<<16 | gc.TUINT32,
		gc.OMOD<<16 | gc.TPTR32:
		a = x86.ADIVL

	case gc.ODIV<<16 | gc.TINT64,
		gc.OMOD<<16 | gc.TINT64:
		a = x86.AIDIVQ

	case gc.ODIV<<16 | gc.TUINT64,
		gc.ODIV<<16 | gc.TPTR64,
		gc.OMOD<<16 | gc.TUINT64,
		gc.OMOD<<16 | gc.TPTR64:
		a = x86.ADIVQ

	case gc.OEXTEND<<16 | gc.TINT16:
		a = x86.ACWD

	case gc.OEXTEND<<16 | gc.TINT32:
		a = x86.ACDQ

	case gc.OEXTEND<<16 | gc.TINT64:
		a = x86.ACQO

	case gc.ODIV<<16 | gc.TFLOAT32:
		a = x86.ADIVSS

	case gc.ODIV<<16 | gc.TFLOAT64:
		a = x86.ADIVSD
	}

	return a
}

const (
	ODynam   = 1 << 0
	OAddable = 1 << 1
)

var clean [20]gc.Node

var cleani int = 0

func xgen(n *gc.Node, a *gc.Node, o int) bool {
	regalloc(a, gc.Types[gc.Tptr], nil)

	if o&ODynam != 0 {
		if n.Addable != 0 {
			if n.Op != gc.OINDREG {
				if n.Op != gc.OREGISTER {
					return true
				}
			}
		}
	}

	agen(n, a)
	return false
}

func sudoclean() {
	if clean[cleani-1].Op != gc.OEMPTY {
		regfree(&clean[cleani-1])
	}
	if clean[cleani-2].Op != gc.OEMPTY {
		regfree(&clean[cleani-2])
	}
	cleani -= 2
}

/*
 * generate code to compute address of n,
 * a reference to a (perhaps nested) field inside
 * an array or struct.
 * return 0 on failure, 1 on success.
 * on success, leaves usable address in a.
 *
 * caller is responsible for calling sudoclean
 * after successful sudoaddable,
 * to release the register used for a.
 */
func sudoaddable(as int, n *gc.Node, a *obj.Addr) bool {
	var o int
	var i int
	var oary [10]int64
	var v int64
	var w int64
	var n1 gc.Node
	var n2 gc.Node
	var n3 gc.Node
	var n4 gc.Node
	var nn *gc.Node
	var l *gc.Node
	var r *gc.Node
	var reg *gc.Node
	var reg1 *gc.Node
	var p1 *obj.Prog
	var t *gc.Type

	if n.Type == nil {
		return false
	}

	*a = obj.Addr{}

	switch n.Op {
	case gc.OLITERAL:
		if !gc.Isconst(n, gc.CTINT) {
			break
		}
		v = gc.Mpgetfix(n.Val.U.Xval)
		if v >= 32000 || v <= -32000 {
			break
		}
		goto lit

	case gc.ODOT,
		gc.ODOTPTR:
		cleani += 2
		reg = &clean[cleani-1]
		reg1 = &clean[cleani-2]
		reg.Op = gc.OEMPTY
		reg1.Op = gc.OEMPTY
		goto odot

	case gc.OINDEX:
		return false

		// disabled: OINDEX case is now covered by agenr
		// for a more suitable register allocation pattern.
		if n.Left.Type.Etype == gc.TSTRING {
			return false
		}
		goto oindex
	}

	return false

lit:
	switch as {
	default:
		return false

	case x86.AADDB,
		x86.AADDW,
		x86.AADDL,
		x86.AADDQ,
		x86.ASUBB,
		x86.ASUBW,
		x86.ASUBL,
		x86.ASUBQ,
		x86.AANDB,
		x86.AANDW,
		x86.AANDL,
		x86.AANDQ,
		x86.AORB,
		x86.AORW,
		x86.AORL,
		x86.AORQ,
		x86.AXORB,
		x86.AXORW,
		x86.AXORL,
		x86.AXORQ,
		x86.AINCB,
		x86.AINCW,
		x86.AINCL,
		x86.AINCQ,
		x86.ADECB,
		x86.ADECW,
		x86.ADECL,
		x86.ADECQ,
		x86.AMOVB,
		x86.AMOVW,
		x86.AMOVL,
		x86.AMOVQ:
		break
	}

	cleani += 2
	reg = &clean[cleani-1]
	reg1 = &clean[cleani-2]
	reg.Op = gc.OEMPTY
	reg1.Op = gc.OEMPTY
	gc.Naddr(n, a, 1)
	goto yes

odot:
	o = gc.Dotoffset(n, oary[:], &nn)
	if nn == nil {
		goto no
	}

	if nn.Addable != 0 && o == 1 && oary[0] >= 0 {
		// directly addressable set of DOTs
		n1 = *nn

		n1.Type = n.Type
		n1.Xoffset += oary[0]
		gc.Naddr(&n1, a, 1)
		goto yes
	}

	regalloc(reg, gc.Types[gc.Tptr], nil)
	n1 = *reg
	n1.Op = gc.OINDREG
	if oary[0] >= 0 {
		agen(nn, reg)
		n1.Xoffset = oary[0]
	} else {
		cgen(nn, reg)
		gc.Cgen_checknil(reg)
		n1.Xoffset = -(oary[0] + 1)
	}

	for i = 1; i < o; i++ {
		if oary[i] >= 0 {
			gc.Fatal("can't happen")
		}
		gins(movptr, &n1, reg)
		gc.Cgen_checknil(reg)
		n1.Xoffset = -(oary[i] + 1)
	}

	a.Type = obj.TYPE_NONE
	a.Index = obj.TYPE_NONE
	fixlargeoffset(&n1)
	gc.Naddr(&n1, a, 1)
	goto yes

oindex:
	l = n.Left
	r = n.Right
	if l.Ullman >= gc.UINF && r.Ullman >= gc.UINF {
		return false
	}

	// set o to type of array
	o = 0

	if gc.Isptr[l.Type.Etype] != 0 {
		gc.Fatal("ptr ary")
	}
	if l.Type.Etype != gc.TARRAY {
		gc.Fatal("not ary")
	}
	if l.Type.Bound < 0 {
		o |= ODynam
	}

	w = n.Type.Width
	if gc.Isconst(r, gc.CTINT) {
		goto oindex_const
	}

	switch w {
	default:
		return false

	case 1,
		2,
		4,
		8:
		break
	}

	cleani += 2
	reg = &clean[cleani-1]
	reg1 = &clean[cleani-2]
	reg.Op = gc.OEMPTY
	reg1.Op = gc.OEMPTY

	// load the array (reg)
	if l.Ullman > r.Ullman {
		if xgen(l, reg, o) {
			o |= OAddable
		}
	}

	// load the index (reg1)
	t = gc.Types[gc.TUINT64]

	if gc.Issigned[r.Type.Etype] != 0 {
		t = gc.Types[gc.TINT64]
	}
	regalloc(reg1, t, nil)
	regalloc(&n3, r.Type, reg1)
	cgen(r, &n3)
	gmove(&n3, reg1)
	regfree(&n3)

	// load the array (reg)
	if l.Ullman <= r.Ullman {
		if xgen(l, reg, o) {
			o |= OAddable
		}
	}

	// check bounds
	if gc.Debug['B'] == 0 && !n.Bounded {
		// check bounds
		n4.Op = gc.OXXX

		t = gc.Types[gc.Simtype[gc.TUINT]]
		if o&ODynam != 0 {
			if o&OAddable != 0 {
				n2 = *l
				n2.Xoffset += int64(gc.Array_nel)
				n2.Type = gc.Types[gc.Simtype[gc.TUINT]]
			} else {
				n2 = *reg
				n2.Xoffset = int64(gc.Array_nel)
				n2.Op = gc.OINDREG
				n2.Type = gc.Types[gc.Simtype[gc.TUINT]]
			}
		} else {
			if gc.Is64(r.Type) {
				t = gc.Types[gc.TUINT64]
			}
			gc.Nodconst(&n2, gc.Types[gc.TUINT64], l.Type.Bound)
		}

		gins(optoas(gc.OCMP, t), reg1, &n2)
		p1 = gc.Gbranch(optoas(gc.OLT, t), nil, +1)
		if n4.Op != gc.OXXX {
			regfree(&n4)
		}
		ginscall(gc.Panicindex, -1)
		gc.Patch(p1, gc.Pc)
	}

	if o&ODynam != 0 {
		if o&OAddable != 0 {
			n2 = *l
			n2.Xoffset += int64(gc.Array_array)
			n2.Type = gc.Types[gc.Tptr]
			gmove(&n2, reg)
		} else {
			n2 = *reg
			n2.Op = gc.OINDREG
			n2.Xoffset = int64(gc.Array_array)
			n2.Type = gc.Types[gc.Tptr]
			gmove(&n2, reg)
		}
	}

	if o&OAddable != 0 {
		gc.Naddr(reg1, a, 1)
		a.Offset = 0
		a.Scale = int8(w)
		a.Index = a.Reg
		a.Type = obj.TYPE_MEM
		a.Reg = reg.Val.U.Reg
	} else {
		gc.Naddr(reg1, a, 1)
		a.Offset = 0
		a.Scale = int8(w)
		a.Index = a.Reg
		a.Type = obj.TYPE_MEM
		a.Reg = reg.Val.U.Reg
	}

	goto yes

	// index is constant
	// can check statically and
	// can multiply by width statically

oindex_const:
	v = gc.Mpgetfix(r.Val.U.Xval)

	if sudoaddable(as, l, a) {
		goto oindex_const_sudo
	}

	cleani += 2
	reg = &clean[cleani-1]
	reg1 = &clean[cleani-2]
	reg.Op = gc.OEMPTY
	reg1.Op = gc.OEMPTY

	if o&ODynam != 0 {
		regalloc(reg, gc.Types[gc.Tptr], nil)
		agen(l, reg)

		if gc.Debug['B'] == 0 && !n.Bounded {
			n1 = *reg
			n1.Op = gc.OINDREG
			n1.Type = gc.Types[gc.Tptr]
			n1.Xoffset = int64(gc.Array_nel)
			gc.Nodconst(&n2, gc.Types[gc.TUINT64], v)
			gins(optoas(gc.OCMP, gc.Types[gc.Simtype[gc.TUINT]]), &n1, &n2)
			p1 = gc.Gbranch(optoas(gc.OGT, gc.Types[gc.Simtype[gc.TUINT]]), nil, +1)
			ginscall(gc.Panicindex, -1)
			gc.Patch(p1, gc.Pc)
		}

		n1 = *reg
		n1.Op = gc.OINDREG
		n1.Type = gc.Types[gc.Tptr]
		n1.Xoffset = int64(gc.Array_array)
		gmove(&n1, reg)

		n2 = *reg
		n2.Op = gc.OINDREG
		n2.Xoffset = v * w
		fixlargeoffset(&n2)
		a.Type = obj.TYPE_NONE
		a.Index = obj.TYPE_NONE
		gc.Naddr(&n2, a, 1)
		goto yes
	}

	igen(l, &n1, nil)
	if n1.Op == gc.OINDREG {
		*reg = n1
		reg.Op = gc.OREGISTER
	}

	n1.Xoffset += v * w
	fixlargeoffset(&n1)
	a.Type = obj.TYPE_NONE
	a.Index = obj.TYPE_NONE
	gc.Naddr(&n1, a, 1)
	goto yes

oindex_const_sudo:
	if o&ODynam == 0 {
		// array indexed by a constant
		a.Offset += v * w

		goto yes
	}

	// slice indexed by a constant
	if gc.Debug['B'] == 0 && !n.Bounded {
		a.Offset += int64(gc.Array_nel)
		gc.Nodconst(&n2, gc.Types[gc.TUINT64], v)
		p1 = gins(optoas(gc.OCMP, gc.Types[gc.Simtype[gc.TUINT]]), nil, &n2)
		p1.From = *a
		p1 = gc.Gbranch(optoas(gc.OGT, gc.Types[gc.Simtype[gc.TUINT]]), nil, +1)
		ginscall(gc.Panicindex, -1)
		gc.Patch(p1, gc.Pc)
		a.Offset -= int64(gc.Array_nel)
	}

	a.Offset += int64(gc.Array_array)
	reg = &clean[cleani-1]
	if reg.Op == gc.OEMPTY {
		regalloc(reg, gc.Types[gc.Tptr], nil)
	}

	p1 = gins(movptr, nil, reg)
	p1.From = *a

	n2 = *reg
	n2.Op = gc.OINDREG
	n2.Xoffset = v * w
	fixlargeoffset(&n2)
	a.Type = obj.TYPE_NONE
	a.Index = obj.TYPE_NONE
	gc.Naddr(&n2, a, 1)
	goto yes

yes:
	return true

no:
	sudoclean()
	return false
}