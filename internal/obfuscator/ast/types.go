package ast

import "encoding/json"

type ParseOutput struct {
	Error *string   `json:"error,omitempty"`
	Root  *StatNode `json:"root,omitempty"`
}

type StatNode struct {
	Type      string      `json:"type"`
	Vars      []string    `json:"vars,omitempty"`      // For LocalStat, ForInStat
	VarNodes  []*ExprNode `json:"var_nodes,omitempty"` // For AssignStat
	Values    []*ExprNode `json:"values,omitempty"`    // For LocalStat, AssignStat, ReturnStat, ForInStat
	Expr      *ExprNode   `json:"expr,omitempty"`      // For ExprStat
	Condition *ExprNode   `json:"condition,omitempty"` // For IfStat, WhileStat, RepeatStat
	Then      *StatNode   `json:"then,omitempty"`      // For IfStat
	Else      *StatNode   `json:"else,omitempty"`      // For IfStat
	Body      []*StatNode `json:"body,omitempty"`      // For Block
	Block     *StatNode   `json:"block,omitempty"`     // For ForStat, ForInStat, WhileStat, RepeatStat

	// For ForStat
	Var  string    `json:"var,omitempty"`
	From *ExprNode `json:"from,omitempty"`
	To   *ExprNode `json:"to,omitempty"`
	Step *ExprNode `json:"step,omitempty"`

	Captured map[string]bool `json:"-"`
}

type TableItem struct {
	Kind string    `json:"kind"` // "List", "Record", "General"
	Key  *ExprNode `json:"key,omitempty"`
	Val  *ExprNode `json:"val,omitempty"`
}

type ExprNode struct {
	Type  string      `json:"type"`
	Value interface{} `json:"value,omitempty"` // For Bool, Number, String
	Name  string      `json:"name,omitempty"`  // For LocalRef, GlobalRef
	Op    string      `json:"op,omitempty"`    // For Binary, Unary
	Left  *ExprNode   `json:"left,omitempty"`  // For Binary
	Right *ExprNode   `json:"right,omitempty"` // For Binary
	Expr  *ExprNode   `json:"expr,omitempty"`  // For Unary, IndexName, IndexExpr

	Index     string    `json:"index,omitempty"`      // For IndexName
	IndexExpr *ExprNode `json:"index_expr,omitempty"` // For IndexExpr

	Func *ExprNode   `json:"func,omitempty"` // For Call
	Args []*ExprNode `json:"args,omitempty"` // For Call
	Self bool        `json:"self,omitempty"` // For Call (method call with :)

	// For Table
	Items []*TableItem `json:"items,omitempty"`

	// For Function (closure)
	Params   []string  `json:"params,omitempty"`
	HasSelf  bool      `json:"has_self,omitempty"`
	Vararg   bool      `json:"vararg,omitempty"`
	Block    *StatNode `json:"block,omitempty"`
	Captured map[string]bool `json:"-"`
}

func FromJSON(data []byte) (*StatNode, error) {
	var out ParseOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out.Root, nil
}
