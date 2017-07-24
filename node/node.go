package node

import (
	"gitlab.zhonganonline.com/ann/angine"
)

// Node include angine
type Node struct {
	angine *angine.Angine
}

// Node number
const NodeNum = 4

// New node
func New(config cfg.Config) *Node {
	panic("not implement yet")
}

// Run node
func (node *Node) Run() {
	panic("not implement yet")
}

// Stop an node
func (node *Node) Stop() {
	panic("not implement yet")
}
