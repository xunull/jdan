package calc

// Node 是 AST 节点（表达式树）。
type Node interface{ node() }

// NumNode 是字面量数字。
type NumNode struct{ Val float64 }

// ConstNode 是常量引用（pi / e）。
type ConstNode struct{ Name string }

// BinNode 是二元运算（a op b）。
type BinNode struct {
	Op   tokKind
	L, R Node
}

// UnaryNode 是一元运算（目前只有负号）。
type UnaryNode struct {
	Op tokKind
	X  Node
}

// CallNode 是函数调用 name(args...)。
type CallNode struct {
	Name string
	Args []Node
}

func (NumNode) node()   {}
func (ConstNode) node() {}
func (BinNode) node()   {}
func (UnaryNode) node() {}
func (CallNode) node()  {}
