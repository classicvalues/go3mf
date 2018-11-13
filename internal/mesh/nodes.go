package mesh

import (
	"github.com/qmuntal/go3mf/internal/geometry"
	"github.com/go-gl/mathgl/mgl32"
)

// MaxNodeCount is the maximum number of nodes allowed.
const MaxNodeCount = 2147483646

// Node defines a node of a mesh.
type Node struct {
	Index    uint32     // Index of the node inside the mesh.
	Position mgl32.Vec3 // Coordinates of the node.
}

type nodeStructure struct {
	vectorTree *geometry.VectorTree
	nodes        []*Node
	maxNodeCount uint32 // If 0 MaxNodeCount will be used.
}

func (n *nodeStructure) clear() {
	n.nodes = make([]*Node, 0)
}

// NodeCount returns the number of nodes in the mesh.
func (n *nodeStructure) NodeCount() uint32 {
	return uint32(len(n.nodes))
}

// Node retrieve the node with the target index.
func (n *nodeStructure) Node(index uint32) *Node {
	return n.nodes[uint32(index)]
}

// AddNode adds a node the the mesh at the target position.
func (n *nodeStructure) AddNode(position mgl32.Vec3) *Node {
	if n.vectorTree != nil {
		if index, ok := n.vectorTree.FindVector(position); ok {
			return n.Node(index)
		}
	}
	nodeCount := n.NodeCount()
	if nodeCount >= n.getMaxNodeCount() {
		panic(new(MaxNodeError))
	}

	node := &Node{
		Index:    nodeCount,
		Position: position,
	}
	n.nodes = append(n.nodes, node)
	if n.vectorTree != nil {
		n.vectorTree.AddVector(node.Position, node.Index)
	}
	return node
}

func (n *nodeStructure) checkSanity() bool {
	nodeCount := n.NodeCount()
	if nodeCount > n.getMaxNodeCount() {
		return false
	}
	for i := 0; i < int(nodeCount); i++ {
		node := n.Node(uint32(i))
		if node.Index != uint32(i) {
			return false
		}
	}
	return true
}

func (n *nodeStructure) merge(other mergeableNodes, matrix mgl32.Mat4) []*Node {
	nodeCount := other.NodeCount()
	newNodes := make([]*Node, nodeCount)
	if nodeCount == 0 {
		return newNodes
	}

	for i := 0; i < int(nodeCount); i++ {
		node := other.Node(uint32(i))
		position := mgl32.TransformCoordinate(node.Position, matrix)
		newNodes[i] = n.AddNode(position)
	}
	return newNodes
}

func (n *nodeStructure) getMaxNodeCount() uint32 {
	if n.maxNodeCount == 0 {
		return MaxNodeCount
	}
	return n.maxNodeCount
}
