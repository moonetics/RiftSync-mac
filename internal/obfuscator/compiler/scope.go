package compiler

import "riftsync/internal/obfuscator/ast"

type scopeInfo struct {
	parent      *scopeInfo
	locals      map[string]bool
	capturedRef *map[string]bool
}

// AnalyzeCaptures traverses the AST and marks which local variables are captured
// by child closures, so they can be managed as upvalues with shared cells.
func AnalyzeCaptures(root *ast.StatNode) {
	if root == nil {
		return
	}
	root.Captured = make(map[string]bool)
	rootScope := &scopeInfo{
		locals:      make(map[string]bool),
		capturedRef: &root.Captured,
	}
	precollectLocals(root, rootScope)
	traverseStat(root, rootScope)
}

func precollectLocals(stat *ast.StatNode, scope *scopeInfo) {
	if stat == nil {
		return
	}
	switch stat.Type {
	case "Block":
		for _, s := range stat.Body {
			precollectLocals(s, scope)
		}
	case "LocalStat":
		for _, v := range stat.Vars {
			scope.locals[v] = true
		}
	case "ForStat":
		if stat.Var != "" {
			scope.locals[stat.Var] = true
		}
		precollectLocals(stat.Block, scope)
	case "ForInStat":
		for _, v := range stat.Vars {
			scope.locals[v] = true
		}
		precollectLocals(stat.Block, scope)
	case "IfStat":
		precollectLocals(stat.Then, scope)
		precollectLocals(stat.Else, scope)
	case "WhileStat", "RepeatStat":
		precollectLocals(stat.Block, scope)
	}
}

func traverseStat(stat *ast.StatNode, scope *scopeInfo) {
	if stat == nil {
		return
	}
	switch stat.Type {
	case "Block":
		for _, s := range stat.Body {
			traverseStat(s, scope)
		}
	case "LocalStat":
		for _, v := range stat.Vars {
			scope.locals[v] = true
		}
		for _, val := range stat.Values {
			traverseExpr(val, scope)
		}
	case "AssignStat":
		for _, vn := range stat.VarNodes {
			traverseExpr(vn, scope)
		}
		for _, val := range stat.Values {
			traverseExpr(val, scope)
		}
	case "ExprStat":
		traverseExpr(stat.Expr, scope)
	case "IfStat":
		traverseExpr(stat.Condition, scope)
		traverseStat(stat.Then, scope)
		traverseStat(stat.Else, scope)
	case "WhileStat", "RepeatStat":
		traverseExpr(stat.Condition, scope)
		traverseStat(stat.Block, scope)
	case "ForStat":
		scope.locals[stat.Var] = true
		traverseExpr(stat.From, scope)
		traverseExpr(stat.To, scope)
		traverseExpr(stat.Step, scope)
		traverseStat(stat.Block, scope)
	case "ForInStat":
		for _, v := range stat.Vars {
			scope.locals[v] = true
		}
		for _, val := range stat.Values {
			traverseExpr(val, scope)
		}
		traverseStat(stat.Block, scope)
	case "ReturnStat":
		for _, val := range stat.Values {
			traverseExpr(val, scope)
		}
	}
}

func traverseExpr(expr *ast.ExprNode, scope *scopeInfo) {
	if expr == nil {
		return
	}
	switch expr.Type {
	case "LocalRef":
		name := expr.Name
		if scope.locals[name] {
			// Local to current function scope
			return
		}
		// Check ancestor function scopes
		for p := scope.parent; p != nil; p = p.parent {
			if p.locals[name] {
				if p.capturedRef != nil {
					(*p.capturedRef)[name] = true
				}
				break
			}
		}
	case "Function":
		expr.Captured = make(map[string]bool)
		fnScope := &scopeInfo{
			parent:      scope,
			locals:      make(map[string]bool),
			capturedRef: &expr.Captured,
		}
		for _, p := range expr.Params {
			fnScope.locals[p] = true
		}
		if expr.Block != nil {
			precollectLocals(expr.Block, fnScope)
			traverseStat(expr.Block, fnScope)
		}
	case "Binary":
		traverseExpr(expr.Left, scope)
		traverseExpr(expr.Right, scope)
	case "Unary":
		traverseExpr(expr.Expr, scope)
	case "Call":
		traverseExpr(expr.Func, scope)
		for _, arg := range expr.Args {
			traverseExpr(arg, scope)
		}
	case "IndexName":
		traverseExpr(expr.Expr, scope)
	case "IndexExpr":
		traverseExpr(expr.Expr, scope)
		traverseExpr(expr.IndexExpr, scope)
	case "Table":
		for _, item := range expr.Items {
			traverseExpr(item.Key, scope)
			traverseExpr(item.Val, scope)
		}
	}
}
