package bytecode

import (
	"math/rand"
	"time"
)

type Opcode int

const (
	OpLoadK Opcode = iota
	OpMove
	OpGetGlobal
	OpSetGlobal
	OpGetTable
	OpSetTable
	OpAdd
	OpSub
	OpMul
	OpDiv
	OpMod
	OpConcat
	OpEq
	OpLt
	OpLe
	OpJmp
	OpCall
	OpReturn
	OpNewTable
	OpClosure
	OpVararg
	OpInitUpval
	OpGetUpval
	OpSetUpval
	OpCount
)

type Instruction struct {
	Op  Opcode
	A   int
	B   int
	C   int
	Bx  int
	SBx int
}

type ConstantType byte

const (
	ConstNil ConstantType = iota
	ConstBoolFalse
	ConstBoolTrue
	ConstNumber
	ConstString
)

type Constant struct {
	Type ConstantType
	Num  float64
	Str  string
}

type Chunk struct {
	Instructions []Instruction
	Constants    []Constant
	Protos       []*Chunk
	NumParams    int
	IsVararg     bool
	OpcodeMap    map[Opcode]int // Virtual opcode IDs
	XorKey       uint32
}

func NewChunk() *Chunk {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate randomized unique opcode mapping
	opMap := make(map[Opcode]int)
	perm := r.Perm(int(OpCount))
	for i := 0; i < int(OpCount); i++ {
		opMap[Opcode(i)] = perm[i] + 1
	}

	return &Chunk{
		OpcodeMap: opMap,
		XorKey:    r.Uint32() | 0x1, // Ensure non-zero
	}
}

func NewSubChunk(parent *Chunk) *Chunk {
	return &Chunk{
		OpcodeMap: parent.OpcodeMap,
		XorKey:    (parent.XorKey + uint32(len(parent.Protos)+1)*7919) | 0x1,
	}
}

func (c *Chunk) AddProto(p *Chunk) int {
	c.Protos = append(c.Protos, p)
	return len(c.Protos) - 1
}

func (c *Chunk) AddConstant(val interface{}) int {
	switch v := val.(type) {
	case nil:
		c.Constants = append(c.Constants, Constant{Type: ConstNil})
		return len(c.Constants) - 1
	case bool:
		if v {
			c.Constants = append(c.Constants, Constant{Type: ConstBoolTrue})
		} else {
			c.Constants = append(c.Constants, Constant{Type: ConstBoolFalse})
		}
		return len(c.Constants) - 1
	case float64:
		c.Constants = append(c.Constants, Constant{Type: ConstNumber, Num: v})
		return len(c.Constants) - 1
	case string:
		c.Constants = append(c.Constants, Constant{Type: ConstString, Str: v})
		return len(c.Constants) - 1
	default:
		c.Constants = append(c.Constants, Constant{Type: ConstNil})
		return len(c.Constants) - 1
	}
}

func (c *Chunk) Emit(op Opcode, a, b, cReg int) int {
	idx := len(c.Instructions)
	c.Instructions = append(c.Instructions, Instruction{
		Op: op,
		A:  a,
		B:  b,
		C:  cReg,
	})
	return idx
}

func (c *Chunk) EmitABx(op Opcode, a, bx int) int {
	idx := len(c.Instructions)
	c.Instructions = append(c.Instructions, Instruction{
		Op: op,
		A:  a,
		Bx: bx,
	})
	return idx
}

func (c *Chunk) EmitAsBx(op Opcode, a, sbx int) int {
	idx := len(c.Instructions)
	bx := sbx + 32767
	c.Instructions = append(c.Instructions, Instruction{
		Op:  op,
		A:   a,
		Bx:  bx,
		SBx: sbx,
	})
	return idx
}
