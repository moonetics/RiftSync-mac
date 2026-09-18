package compiler

import (
	"fmt"

	"riftsync/internal/obfuscator/ast"
	"riftsync/internal/obfuscator/bytecode"
)

type loopContext struct {
	breaks    []int
	continues []int
}

type Compiler struct {
	chunk    *bytecode.Chunk
	locals   map[string]int
	captured map[string]bool
	nextReg  int
	maxLocal int
	parent   *Compiler
	loops    []*loopContext
}

func (c *Compiler) isCaptured(name string) bool {
	if c.captured == nil {
		return false
	}
	return c.captured[name]
}

func (c *Compiler) hasAncestorLocal(name string) bool {
	for p := c.parent; p != nil; p = p.parent {
		if _, ok := p.locals[name]; ok {
			return true
		}
	}
	return false
}

func NewCompiler() *Compiler {
	return &Compiler{
		chunk:    bytecode.NewChunk(),
		locals:   make(map[string]int),
		nextReg:  0,
		maxLocal: 0,
	}
}

func (c *Compiler) allocReg() int {
	reg := c.nextReg
	c.nextReg++
	return reg
}

func (c *Compiler) allocLocalReg() int {
	reg := c.allocReg()
	if reg >= c.maxLocal {
		c.maxLocal = reg + 1
	}
	return reg
}

func (c *Compiler) freeTemps() {
	c.nextReg = c.maxLocal
}

func (c *Compiler) enterLoop() {
	c.loops = append(c.loops, &loopContext{})
}

func (c *Compiler) exitLoop(breakTarget, continueTarget int) {
	if len(c.loops) == 0 {
		return
	}
	ctx := c.loops[len(c.loops)-1]
	c.loops = c.loops[:len(c.loops)-1]

	for _, bIdx := range ctx.breaks {
		c.patchJump(bIdx, breakTarget)
	}
	for _, cIdx := range ctx.continues {
		c.patchJump(cIdx, continueTarget)
	}
}

func (c *Compiler) patchJump(instIdx int, targetIdx int) {
	offset := targetIdx - instIdx - 1
	c.chunk.Instructions[instIdx].SBx = offset
	c.chunk.Instructions[instIdx].Bx = offset + 32767
}

func (c *Compiler) Compile(root *ast.StatNode) (*bytecode.Chunk, error) {
	if root == nil {
		return nil, fmt.Errorf("nil AST root")
	}
	AnalyzeCaptures(root)
	c.captured = root.Captured
	if err := c.compileStat(root); err != nil {
		return nil, err
	}
	// Always emit return at end of script
	c.chunk.Emit(bytecode.OpReturn, 0, 1, 0)
	return c.chunk, nil
}

func (c *Compiler) compileStat(stat *ast.StatNode) error {
	switch stat.Type {
	case "Block":
		for _, s := range stat.Body {
			if err := c.compileStat(s); err != nil {
				return err
			}
		}
	case "LocalStat":
		for i, name := range stat.Vars {
			isUp := c.isCaptured(name)
			var targetReg int
			if !isUp {
				targetReg = c.allocLocalReg()
				c.locals[name] = targetReg
			}
			var valReg int
			if i < len(stat.Values) {
				vr, err := c.compileExpr(stat.Values[i])
				if err != nil {
					return err
				}
				valReg = vr
			} else {
				kIdx := c.chunk.AddConstant(nil)
				valReg = c.allocReg()
				c.chunk.EmitABx(bytecode.OpLoadK, valReg, kIdx)
			}
			if isUp {
				c.locals[name] = -1 // mark as local
				kIdx := c.chunk.AddConstant(name)
				c.chunk.EmitABx(bytecode.OpInitUpval, valReg, kIdx)
			} else {
				if valReg != targetReg {
					c.chunk.Emit(bytecode.OpMove, targetReg, valReg, 0)
				}
			}
		}
		c.freeTemps()
	case "AssignStat":
		for i, valExpr := range stat.Values {
			valReg, err := c.compileExpr(valExpr)
			if err != nil {
				return err
			}
			if i < len(stat.VarNodes) {
				varNode := stat.VarNodes[i]
				if varNode.Type == "LocalRef" {
					if c.isCaptured(varNode.Name) || c.hasAncestorLocal(varNode.Name) {
						kIdx := c.chunk.AddConstant(varNode.Name)
						c.chunk.EmitABx(bytecode.OpSetUpval, valReg, kIdx)
					} else if targetReg, ok := c.locals[varNode.Name]; ok {
						c.chunk.Emit(bytecode.OpMove, targetReg, valReg, 0)
					} else {
						kIdx := c.chunk.AddConstant(varNode.Name)
						c.chunk.EmitABx(bytecode.OpSetGlobal, valReg, kIdx)
					}
				} else if varNode.Type == "GlobalRef" {
					kIdx := c.chunk.AddConstant(varNode.Name)
					c.chunk.EmitABx(bytecode.OpSetGlobal, valReg, kIdx)
				} else if varNode.Type == "IndexName" {
					tableReg, err := c.compileExpr(varNode.Expr)
					if err != nil {
						return err
					}
					keyReg := c.allocReg()
					kIdx := c.chunk.AddConstant(varNode.Index)
					c.chunk.EmitABx(bytecode.OpLoadK, keyReg, kIdx)
					c.chunk.Emit(bytecode.OpSetTable, tableReg, keyReg, valReg)
				} else if varNode.Type == "IndexExpr" {
					tableReg, err := c.compileExpr(varNode.Expr)
					if err != nil {
						return err
					}
					keyReg, err := c.compileExpr(varNode.IndexExpr)
					if err != nil {
						return err
					}
					c.chunk.Emit(bytecode.OpSetTable, tableReg, keyReg, valReg)
				}
			}
		}
		c.freeTemps()
	case "ExprStat":
		if stat.Expr != nil {
			if _, err := c.compileExpr(stat.Expr); err != nil {
				return err
			}
		}
		c.freeTemps()
	case "IfStat":
		condReg, err := c.compileExpr(stat.Condition)
		if err != nil {
			return err
		}
		falseConstReg := c.allocReg()
		falseKIdx := c.chunk.AddConstant(false)
		c.chunk.EmitABx(bytecode.OpLoadK, falseConstReg, falseKIdx)

		c.chunk.Emit(bytecode.OpEq, 1, condReg, falseConstReg)
		jmpFalseIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.freeTemps()

		if stat.Then != nil {
			if err := c.compileStat(stat.Then); err != nil {
				return err
			}
		}

		if stat.Else != nil {
			jmpEndIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.patchJump(jmpFalseIdx, len(c.chunk.Instructions))
			if err := c.compileStat(stat.Else); err != nil {
				return err
			}
			c.patchJump(jmpEndIdx, len(c.chunk.Instructions))
		} else {
			c.patchJump(jmpFalseIdx, len(c.chunk.Instructions))
		}
	case "ForStat":
		fromReg, err := c.compileExpr(stat.From)
		if err != nil {
			return err
		}
		toReg, err := c.compileExpr(stat.To)
		if err != nil {
			return err
		}
		var stepReg int
		if stat.Step != nil {
			stepReg, err = c.compileExpr(stat.Step)
			if err != nil {
				return err
			}
		} else {
			stepReg = c.allocReg()
			kIdx := c.chunk.AddConstant(1.0)
			c.chunk.EmitABx(bytecode.OpLoadK, stepReg, kIdx)
		}

		varReg := c.allocLocalReg()
		c.chunk.Emit(bytecode.OpMove, varReg, fromReg, 0)
		c.locals[stat.Var] = varReg

		loopStart := len(c.chunk.Instructions)
		c.chunk.Emit(bytecode.OpLe, 0, varReg, toReg)
		jmpEnd := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)

		c.enterLoop()
		if stat.Block != nil {
			if err := c.compileStat(stat.Block); err != nil {
				return err
			}
		}
		continueTarget := len(c.chunk.Instructions)
		c.chunk.Emit(bytecode.OpAdd, varReg, varReg, stepReg)
		jmpBack := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.patchJump(jmpBack, loopStart)

		endTarget := len(c.chunk.Instructions)
		c.patchJump(jmpEnd, endTarget)
		c.exitLoop(endTarget, continueTarget)
		c.freeTemps()

	case "ForInStat":
		if len(stat.Values) == 0 {
			return fmt.Errorf("ForInStat without values")
		}
		var iterFunc, stateReg, varValReg int
		if stat.Values[0].Type == "Call" {
			funcReg, err := c.compileExpr(stat.Values[0].Func)
			if err != nil {
				return err
			}
			fBase := c.allocReg()
			c.chunk.Emit(bytecode.OpMove, fBase, funcReg, 0)
			for _, arg := range stat.Values[0].Args {
				aReg := c.allocReg()
				evReg, err := c.compileExpr(arg)
				if err != nil {
					return err
				}
				if evReg != aReg {
					c.chunk.Emit(bytecode.OpMove, aReg, evReg, 0)
				}
			}
			c.chunk.Emit(bytecode.OpCall, fBase, len(stat.Values[0].Args)+1, 4)
			iterFunc = fBase
			stateReg = fBase + 1
			varValReg = fBase + 2
		} else {
			iterReg, err := c.compileExpr(stat.Values[0])
			if err != nil {
				return err
			}
			iterFunc = iterReg
			stateReg = c.allocReg()
			c.chunk.EmitABx(bytecode.OpLoadK, stateReg, c.chunk.AddConstant(nil))
			varValReg = c.allocReg()
			c.chunk.EmitABx(bytecode.OpLoadK, varValReg, c.chunk.AddConstant(nil))
		}

		varRegs := make([]int, len(stat.Vars))
		for i, v := range stat.Vars {
			r := c.allocLocalReg()
			c.locals[v] = r
			varRegs[i] = r
		}

		loopStart := len(c.chunk.Instructions)
		runBase := c.allocReg()
		c.chunk.Emit(bytecode.OpMove, runBase, iterFunc, 0)
		arg1 := c.allocReg()
		c.chunk.Emit(bytecode.OpMove, arg1, stateReg, 0)
		arg2 := c.allocReg()
		c.chunk.Emit(bytecode.OpMove, arg2, varValReg, 0)

		numVars := len(stat.Vars)
		c.chunk.Emit(bytecode.OpCall, runBase, 3, numVars+1)

		nilK := c.chunk.AddConstant(nil)
		nilReg := c.allocReg()
		c.chunk.EmitABx(bytecode.OpLoadK, nilReg, nilK)
		c.chunk.Emit(bytecode.OpEq, 1, runBase, nilReg)
		jmpEnd := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)

		for i := 0; i < numVars; i++ {
			c.chunk.Emit(bytecode.OpMove, varRegs[i], runBase+i, 0)
		}
		c.chunk.Emit(bytecode.OpMove, varValReg, varRegs[0], 0)

		c.enterLoop()
		if stat.Block != nil {
			if err := c.compileStat(stat.Block); err != nil {
				return err
			}
		}
		continueTarget := len(c.chunk.Instructions)
		jmpBack := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.patchJump(jmpBack, loopStart)

		endTarget := len(c.chunk.Instructions)
		c.patchJump(jmpEnd, endTarget)
		c.exitLoop(endTarget, continueTarget)
		c.freeTemps()

	case "WhileStat":
		loopStart := len(c.chunk.Instructions)
		condReg, err := c.compileExpr(stat.Condition)
		if err != nil {
			return err
		}
		falseK := c.chunk.AddConstant(false)
		falseReg := c.allocReg()
		c.chunk.EmitABx(bytecode.OpLoadK, falseReg, falseK)
		c.chunk.Emit(bytecode.OpEq, 1, condReg, falseReg)
		jmpEnd := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.freeTemps()

		c.enterLoop()
		if stat.Block != nil {
			if err := c.compileStat(stat.Block); err != nil {
				return err
			}
		}
		continueTarget := len(c.chunk.Instructions)
		jmpBack := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.patchJump(jmpBack, loopStart)

		endTarget := len(c.chunk.Instructions)
		c.patchJump(jmpEnd, endTarget)
		c.exitLoop(endTarget, continueTarget)
		c.freeTemps()

	case "RepeatStat":
		loopStart := len(c.chunk.Instructions)
		c.enterLoop()
		if stat.Block != nil {
			if err := c.compileStat(stat.Block); err != nil {
				return err
			}
		}
		continueTarget := len(c.chunk.Instructions)
		condReg, err := c.compileExpr(stat.Condition)
		if err != nil {
			return err
		}
		falseK := c.chunk.AddConstant(false)
		falseReg := c.allocReg()
		c.chunk.EmitABx(bytecode.OpLoadK, falseReg, falseK)
		c.chunk.Emit(bytecode.OpEq, 1, condReg, falseReg)
		jmpBack := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
		c.patchJump(jmpBack, loopStart)

		endTarget := len(c.chunk.Instructions)
		c.exitLoop(endTarget, continueTarget)
		c.freeTemps()

	case "BreakStat":
		if len(c.loops) > 0 {
			jmpIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.loops[len(c.loops)-1].breaks = append(c.loops[len(c.loops)-1].breaks, jmpIdx)
		}

	case "ContinueStat":
		if len(c.loops) > 0 {
			jmpIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.loops[len(c.loops)-1].continues = append(c.loops[len(c.loops)-1].continues, jmpIdx)
		}

	case "ReturnStat":
		if len(stat.Values) == 0 {
			c.chunk.Emit(bytecode.OpReturn, 0, 1, 0)
		} else {
			valRegs := make([]int, len(stat.Values))
			for i, valExpr := range stat.Values {
				vr, err := c.compileExpr(valExpr)
				if err != nil {
					return err
				}
				valRegs[i] = vr
			}
			firstReg := c.allocReg()
			c.chunk.Emit(bytecode.OpMove, firstReg, valRegs[0], 0)
			for i := 1; i < len(valRegs); i++ {
				r := c.allocReg()
				c.chunk.Emit(bytecode.OpMove, r, valRegs[i], 0)
			}
			c.chunk.Emit(bytecode.OpReturn, firstReg, len(stat.Values)+1, 0)
		}
		c.freeTemps()
	}
	return nil
}

func (c *Compiler) compileExpr(expr *ast.ExprNode) (int, error) {
	if expr == nil {
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(nil)
		c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
		return reg, nil
	}

	switch expr.Type {
	case "Nil":
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(nil)
		c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
		return reg, nil
	case "Bool":
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Value.(bool))
		c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
		return reg, nil
	case "Number":
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Value.(float64))
		c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
		return reg, nil
	case "String":
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Value.(string))
		c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
		return reg, nil
	case "LocalRef":
		if c.isCaptured(expr.Name) || c.hasAncestorLocal(expr.Name) {
			reg := c.allocReg()
			kIdx := c.chunk.AddConstant(expr.Name)
			c.chunk.EmitABx(bytecode.OpGetUpval, reg, kIdx)
			return reg, nil
		}
		if reg, ok := c.locals[expr.Name]; ok && reg >= 0 {
			return reg, nil
		}
		// Fallback to global
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Name)
		c.chunk.EmitABx(bytecode.OpGetGlobal, reg, kIdx)
		return reg, nil
	case "GlobalRef":
		reg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Name)
		c.chunk.EmitABx(bytecode.OpGetGlobal, reg, kIdx)
		return reg, nil
	case "Unary":
		operandReg, err := c.compileExpr(expr.Expr)
		if err != nil {
			return 0, err
		}
		destReg := c.allocReg()
		switch expr.Op {
		case "-":
			zeroReg := c.allocReg()
			kIdx := c.chunk.AddConstant(0.0)
			c.chunk.EmitABx(bytecode.OpLoadK, zeroReg, kIdx)
			c.chunk.Emit(bytecode.OpSub, destReg, zeroReg, operandReg)
		case "not":
			falseReg := c.allocReg()
			falseK := c.chunk.AddConstant(false)
			trueK := c.chunk.AddConstant(true)
			c.chunk.EmitABx(bytecode.OpLoadK, falseReg, falseK)
			c.chunk.Emit(bytecode.OpEq, 1, operandReg, falseReg)
			jmpIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.chunk.EmitABx(bytecode.OpLoadK, destReg, falseK)
			jmpEnd := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.patchJump(jmpIdx, len(c.chunk.Instructions))
			c.chunk.EmitABx(bytecode.OpLoadK, destReg, trueK)
			c.patchJump(jmpEnd, len(c.chunk.Instructions))
		default:
			c.chunk.Emit(bytecode.OpMove, destReg, operandReg, 0)
		}
		return destReg, nil
	case "Binary":
		leftReg, err := c.compileExpr(expr.Left)
		if err != nil {
			return 0, err
		}
		rightReg, err := c.compileExpr(expr.Right)
		if err != nil {
			return 0, err
		}
		destReg := c.allocReg()
		var op bytecode.Opcode
		switch expr.Op {
		case "+":
			op = bytecode.OpAdd
		case "-":
			op = bytecode.OpSub
		case "*":
			op = bytecode.OpMul
		case "/":
			op = bytecode.OpDiv
		case "%":
			op = bytecode.OpMod
		case "..":
			op = bytecode.OpConcat
		case "==", "~=", "<", "<=", ">", ">=":
			var emitOp bytecode.Opcode
			aVal := 1
			l := leftReg
			r := rightReg
			switch expr.Op {
			case "==":
				emitOp = bytecode.OpEq
				aVal = 1
			case "~=":
				emitOp = bytecode.OpEq
				aVal = 0
			case "<":
				emitOp = bytecode.OpLt
				aVal = 1
			case "<=":
				emitOp = bytecode.OpLe
				aVal = 1
			case ">":
				emitOp = bytecode.OpLt
				aVal = 1
				l, r = rightReg, leftReg
			case ">=":
				emitOp = bytecode.OpLe
				aVal = 1
				l, r = rightReg, leftReg
			}
			c.chunk.Emit(emitOp, aVal, l, r)
			jmpIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			falseK := c.chunk.AddConstant(false)
			trueK := c.chunk.AddConstant(true)
			c.chunk.EmitABx(bytecode.OpLoadK, destReg, falseK)
			jmpEndIdx := c.chunk.EmitAsBx(bytecode.OpJmp, 0, 0)
			c.patchJump(jmpIdx, len(c.chunk.Instructions))
			c.chunk.EmitABx(bytecode.OpLoadK, destReg, trueK)
			c.patchJump(jmpEndIdx, len(c.chunk.Instructions))
			return destReg, nil
		default:
			op = bytecode.OpAdd
		}
		c.chunk.Emit(op, destReg, leftReg, rightReg)
		return destReg, nil
	case "Call":
		if expr.Self && expr.Func != nil && expr.Func.Type == "IndexName" {
			objReg, err := c.compileExpr(expr.Func.Expr)
			if err != nil {
				return 0, err
			}
			keyReg := c.allocReg()
			kIdx := c.chunk.AddConstant(expr.Func.Index)
			c.chunk.EmitABx(bytecode.OpLoadK, keyReg, kIdx)
			methodReg := c.allocReg()
			c.chunk.Emit(bytecode.OpGetTable, methodReg, objReg, keyReg)

			// Pre-evaluate arguments
			argRegs := make([]int, len(expr.Args))
			for i, arg := range expr.Args {
				ar, err := c.compileExpr(arg)
				if err != nil {
					return 0, err
				}
				argRegs[i] = ar
			}

			// Allocate contiguous registers: base (method), self (obj), and args
			baseReg := c.allocReg()
			c.chunk.Emit(bytecode.OpMove, baseReg, methodReg, 0)
			selfReg := c.allocReg()
			c.chunk.Emit(bytecode.OpMove, selfReg, objReg, 0)
			for _, ar := range argRegs {
				slot := c.allocReg()
				c.chunk.Emit(bytecode.OpMove, slot, ar, 0)
			}
			numArgs := len(expr.Args) + 1
			c.chunk.Emit(bytecode.OpCall, baseReg, numArgs+1, 2)
			return baseReg, nil
		}

		funcReg, err := c.compileExpr(expr.Func)
		if err != nil {
			return 0, err
		}

		// Pre-evaluate arguments
		argRegs := make([]int, len(expr.Args))
		for i, arg := range expr.Args {
			ar, err := c.compileExpr(arg)
			if err != nil {
				return 0, err
			}
			argRegs[i] = ar
		}

		// Allocate contiguous registers: base (func) and args
		baseReg := c.allocReg()
		c.chunk.Emit(bytecode.OpMove, baseReg, funcReg, 0)
		for _, ar := range argRegs {
			slot := c.allocReg()
			c.chunk.Emit(bytecode.OpMove, slot, ar, 0)
		}
		numArgs := len(expr.Args)
		c.chunk.Emit(bytecode.OpCall, baseReg, numArgs+1, 2)
		return baseReg, nil
	case "IndexName":
		tableReg, err := c.compileExpr(expr.Expr)
		if err != nil {
			return 0, err
		}
		keyReg := c.allocReg()
		kIdx := c.chunk.AddConstant(expr.Index)
		c.chunk.EmitABx(bytecode.OpLoadK, keyReg, kIdx)
		destReg := c.allocReg()
		c.chunk.Emit(bytecode.OpGetTable, destReg, tableReg, keyReg)
		return destReg, nil
	case "IndexExpr":
		tableReg, err := c.compileExpr(expr.Expr)
		if err != nil {
			return 0, err
		}
		keyReg, err := c.compileExpr(expr.IndexExpr)
		if err != nil {
			return 0, err
		}
		destReg := c.allocReg()
		c.chunk.Emit(bytecode.OpGetTable, destReg, tableReg, keyReg)
		return destReg, nil
	case "Table":
		destReg := c.allocReg()
		c.chunk.EmitABx(bytecode.OpNewTable, destReg, 0)
		listIdx := 1
		for _, item := range expr.Items {
			valReg, err := c.compileExpr(item.Val)
			if err != nil {
				return 0, err
			}
			var keyReg int
			switch item.Kind {
			case "List":
				keyReg = c.allocReg()
				kIdx := c.chunk.AddConstant(float64(listIdx))
				c.chunk.EmitABx(bytecode.OpLoadK, keyReg, kIdx)
				listIdx++
			case "Record":
				if item.Key != nil {
					keyReg, err = c.compileExpr(item.Key)
					if err != nil {
						return 0, err
					}
				}
			default:
				if item.Key != nil {
					keyReg, err = c.compileExpr(item.Key)
					if err != nil {
						return 0, err
					}
				}
			}
			c.chunk.Emit(bytecode.OpSetTable, destReg, keyReg, valReg)
		}
		return destReg, nil
	case "Function":
		subChunk := bytecode.NewSubChunk(c.chunk)
		subChunk.NumParams = len(expr.Params)
		subChunk.IsVararg = expr.Vararg
		subCompiler := &Compiler{
			chunk:    subChunk,
			locals:   make(map[string]int),
			captured: expr.Captured,
			nextReg:  0,
			parent:   c,
		}
		for _, param := range expr.Params {
			pReg := subCompiler.allocReg()
			if subCompiler.isCaptured(param) {
				subCompiler.locals[param] = -1
				kIdx := subChunk.AddConstant(param)
				subChunk.EmitABx(bytecode.OpInitUpval, pReg, kIdx)
			} else {
				subCompiler.locals[param] = pReg
			}
		}
		if expr.Block != nil {
			if err := subCompiler.compileStat(expr.Block); err != nil {
				return 0, err
			}
		}
		subChunk.Emit(bytecode.OpReturn, 0, 1, 0)

		protoIdx := c.chunk.AddProto(subChunk)
		destReg := c.allocReg()
		c.chunk.EmitABx(bytecode.OpClosure, destReg, protoIdx)
		return destReg, nil
	case "Vararg":
		destReg := c.allocReg()
		c.chunk.Emit(bytecode.OpVararg, destReg, 0, 0)
		return destReg, nil
	}

	reg := c.allocReg()
	kIdx := c.chunk.AddConstant(nil)
	c.chunk.EmitABx(bytecode.OpLoadK, reg, kIdx)
	return reg, nil
}
