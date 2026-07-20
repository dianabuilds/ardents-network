package process

func New(cfg Config) NodeRuntime {
	return NewNodeRuntime(cfg)
}

func NewNodeRuntime(cfg Config) NodeRuntime {
	return NewNode(cfg)
}

func (n *Node) NodeForTesting() any { return n }
