package vm

import (
	"fmt"
	"math/rand"
	"strings"

	"riftsync/internal/obfuscator/bytecode"
)

const watermark = `--[[
  ███████╗ ██████╗ ██████╗ ███████╗██╗   ██╗██╗██╗   ██╗███╗   ███╗
  ██╔════╝██╔═══██╗██╔══██╗██╔════╝██║   ██║██║██║   ██║████╗ ████║
  ███████╗██║   ██║██████╔╝█████╗  ██║   ██║██║██║   ██║██╔████╔██║
  ╚════██║██║   ██║██╔══██╗██╔══╝  ╚██╗ ██╔╝██║██║   ██║██║╚██╔╝██║
  ███████║╚██████╔╝██║  ██║███████╗ ╚████╔╝ ██║╚██████╔╝██║ ╚═╝ ██║
  ╚══════╝ ╚═════╝ ╚═╝  ╚═╝╚══════╝  ╚═══╝  ╚═╝ ╚═════╝ ╚═╝     ╚═╝
                       S O R E V I U M   V M
                 Developed by Sorevi Labs Studio
             Protected with Virtual Machine Security
--]]
`

func encryptString(s string, key byte) string {
	var sb strings.Builder
	sb.WriteString(`_D("`)
	for i := 0; i < len(s); i++ {
		enc := s[i] ^ (key + byte((i+1)*7))
		sb.WriteString(fmt.Sprintf(`\%d`, enc))
	}
	sb.WriteString(fmt.Sprintf(`",%d)`, key))
	return sb.String()
}

func formatProto(chunk *bytecode.Chunk, strKey byte) string {
	// 1. Pack Instructions
	var rawInsts []string
	for i, inst := range chunk.Instructions {
		opVal := chunk.OpcodeMap[inst.Op]
		var word uint32
		if inst.Op == bytecode.OpLoadK || inst.Op == bytecode.OpGetGlobal || inst.Op == bytecode.OpSetGlobal ||
			inst.Op == bytecode.OpJmp || inst.Op == bytecode.OpNewTable || inst.Op == bytecode.OpClosure ||
			inst.Op == bytecode.OpInitUpval || inst.Op == bytecode.OpGetUpval || inst.Op == bytecode.OpSetUpval {
			bx := uint32(inst.Bx & 0xFFFF)
			word = uint32(opVal&0xFF) | (uint32(inst.A&0xFF) << 8) | (bx << 16)
		} else {
			word = uint32(opVal&0xFF) | (uint32(inst.A&0xFF) << 8) | (uint32(inst.B&0xFF) << 16) | (uint32(inst.C&0xFF) << 24)
		}
		rollingKey := (chunk.XorKey + uint32(i+1)*13) & 0xFFFFFFFF
		raw := word ^ rollingKey
		rawInsts = append(rawInsts, fmt.Sprintf("%d", raw))
	}

	// 2. Format Constants
	var constStrs []string
	for _, k := range chunk.Constants {
		switch k.Type {
		case bytecode.ConstNil:
			constStrs = append(constStrs, "nil")
		case bytecode.ConstBoolFalse:
			constStrs = append(constStrs, "false")
		case bytecode.ConstBoolTrue:
			constStrs = append(constStrs, "true")
		case bytecode.ConstNumber:
			constStrs = append(constStrs, fmt.Sprintf("%g", k.Num))
		case bytecode.ConstString:
			constStrs = append(constStrs, encryptString(k.Str, strKey))
		}
	}

	// 3. Format Sub-Prototypes (child functions/closures)
	var protoStrs []string
	for _, p := range chunk.Protos {
		protoStrs = append(protoStrs, formatProto(p, strKey))
	}

	return fmt.Sprintf("{K={%s},I={%s},X=%d,P={%s}}",
		strings.Join(constStrs, ","),
		strings.Join(rawInsts, ","),
		chunk.XorKey,
		strings.Join(protoStrs, ","),
	)
}

func Generate(chunk *bytecode.Chunk) string {
	strKey := byte(rand.Intn(200) + 25)
	mainProto := formatProto(chunk, strKey)
	opcodes := chunk.OpcodeMap

	vmTemplate := fmt.Sprintf(`return(function()local _b=bit32 local _u=table.unpack local _D=function(s,k)local t={}local l=#s local b=string.byte local c=string.char local x=_b.bxor for i=1,l do t[i]=c(x(b(s,i),(k+i*7)%%256))end return table.concat(t)end local _E=(getfenv and getfenv())or{}local G=setmetatable({},{__index=function(_,k)if _G and _G[k]~=nil then return _G[k]end return _E[k]end,__newindex=function(_,k,v)if _G then _G[k]=v end _E[k]=v end})local _1=%d local _2=%d local _3=%d local _4=%d local _5=%d local _6=%d local _7=%d local _8=%d local _9=%d local _10=%d local _11=%d local _12=%d local _13=%d local _14=%d local _15=%d local _16=%d local _17=%d local _18=%d local _19=%d local _20=%d local _21=%d local _22=%d local _23=%d local _24=%d local _R _R=function(_t,_U,...)local K=_t.K local I=_t.I local X=_t.X local P=_t.P local _r={}local _v={...}for i=1,#_v do _r[i-1]=_v[i]end local _p=1 local _m=#I while _p<=_m do local _w=_b.bxor(I[_p],_b.band(X+_p*13,4294967295))local _o=_b.extract(_w,0,8)local _a=_b.extract(_w,8,8)local _c=_b.extract(_w,24,8)local _bx=_b.extract(_w,16,16)local _sx=_bx-32767 _p=_p+1 if _o==_1 then _r[_a]=K[_bx+1]elseif _o==_2 then _r[_a]=_r[_b.extract(_w,16,8)]elseif _o==_3 then _r[_a]=G[K[_bx+1]]elseif _o==_4 then G[K[_bx+1]]=_r[_a]elseif _o==_5 then _r[_a]=_r[_b.extract(_w,16,8)][_r[_c]]elseif _o==_6 then _r[_a][_r[_b.extract(_w,16,8)]]=_r[_c]elseif _o==_7 then _r[_a]=_r[_b.extract(_w,16,8)]+_r[_c]elseif _o==_8 then _r[_a]=_r[_b.extract(_w,16,8)]-_r[_c]elseif _o==_9 then _r[_a]=_r[_b.extract(_w,16,8)]*_r[_c]elseif _o==_10 then _r[_a]=_r[_b.extract(_w,16,8)]/_r[_c]elseif _o==_11 then _r[_a]=_r[_b.extract(_w,16,8)]%%_r[_c]elseif _o==_12 then _r[_a]=_r[_b.extract(_w,16,8)].._r[_c]elseif _o==_13 then if(_r[_b.extract(_w,16,8)]==_r[_c])~=(_a~=0)then _p=_p+1 end elseif _o==_14 then if(_r[_b.extract(_w,16,8)]<_r[_c])~=(_a~=0)then _p=_p+1 end elseif _o==_15 then if(_r[_b.extract(_w,16,8)]<=_r[_c])~=(_a~=0)then _p=_p+1 end elseif _o==_16 then _p=_p+_sx elseif _o==_17 then local _f=_r[_a]local _n=_b.extract(_w,16,8)-1 local _g={}for i=1,_n do _g[i]=_r[_a+i]end local _s={_f(_u(_g,1,_n))}if _c>1 then for i=1,_c-1 do _r[_a+i-1]=_s[i]end end elseif _o==_18 then local _n=_b.extract(_w,16,8)-1 if _n==0 then return end local _s={}for i=1,_n do _s[i]=_r[_a+i-1]end return _u(_s,1,_n)elseif _o==_19 then _r[_a]={}elseif _o==_20 then local _proto=P[_bx+1] local _childU=setmetatable({},{__index=_U}) _r[_a]=function(...)return _R(_proto,_childU,...)end elseif _o==_21 then for i=1,#_v do _r[_a+i-1]=_v[i]end elseif _o==_22 then _U[K[_bx+1]]={_r[_a]}elseif _o==_23 then local _cell=_U[K[_bx+1]] if _cell then _r[_a]=_cell[1]else _r[_a]=nil end elseif _o==_24 then local _cell=_U[K[_bx+1]] if _cell then _cell[1]=_r[_a]else _U[K[_bx+1]]={_r[_a]}end end end end return _R(%s,{})end)()`,
		opcodes[bytecode.OpLoadK],
		opcodes[bytecode.OpMove],
		opcodes[bytecode.OpGetGlobal],
		opcodes[bytecode.OpSetGlobal],
		opcodes[bytecode.OpGetTable],
		opcodes[bytecode.OpSetTable],
		opcodes[bytecode.OpAdd],
		opcodes[bytecode.OpSub],
		opcodes[bytecode.OpMul],
		opcodes[bytecode.OpDiv],
		opcodes[bytecode.OpMod],
		opcodes[bytecode.OpConcat],
		opcodes[bytecode.OpEq],
		opcodes[bytecode.OpLt],
		opcodes[bytecode.OpLe],
		opcodes[bytecode.OpJmp],
		opcodes[bytecode.OpCall],
		opcodes[bytecode.OpReturn],
		opcodes[bytecode.OpNewTable],
		opcodes[bytecode.OpClosure],
		opcodes[bytecode.OpVararg],
		opcodes[bytecode.OpInitUpval],
		opcodes[bytecode.OpGetUpval],
		opcodes[bytecode.OpSetUpval],
		mainProto,
	)

	return watermark + vmTemplate
}
