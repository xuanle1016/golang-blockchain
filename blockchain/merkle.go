package blockchain

import "crypto/sha256"

// MerkleTree 结构体表示一个 Merkle 树，其中包含树的根节点
type MerkleTree struct {
	RootNode *MerkleNode // 树的根节点
}

// MerkleNode 结构体表示 Merkle 树的节点
type MerkleNode struct {
	Left  *MerkleNode // 左子节点
	Right *MerkleNode // 右子节点
	Data  []byte      // 当前节点的数据（哈希值）
}

// NewMerkleNode 创建一个新的 MerkleNode，计算节点的哈希值
// 如果是叶子节点（没有子节点），则直接使用数据计算哈希；
// 否则，通过左右子节点的哈希值计算父节点的哈希。
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := MerkleNode{}

	if left == nil && right == nil {
		// 如果是叶子节点，直接用数据计算哈希
		hash := sha256.Sum256(data)
		node.Data = hash[:] // 存储哈希值
	} else {
		// 如果是父节点，将左右子节点的哈希值拼接后计算哈希
		prevHashes := append(left.Data, right.Data...)
		hash := sha256.Sum256(prevHashes)
		node.Data = hash[:] // 存储哈希值
	}

	// 设置左右子节点
	node.Left = left
	node.Right = right

	return &node // 返回新创建的节点
}

// NewMerkleTree 创建一个新的 MerkleTree，并返回树的根节点
// 将传入的多组数据计算成 Merkle 树
func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []MerkleNode

	// 如果数据量为奇数，重复最后一个数据项，确保每一层的节点数为偶数
	if len(data)%2 != 0 {
		data = append(data, data[len(data)-1])
	}

	// 先将所有数据创建成叶子节点
	for _, datum := range data {
		node := NewMerkleNode(nil, nil, datum) // 创建叶子节点
		nodes = append(nodes, *node)           // 将节点添加到节点切片中
	}

	// 从底层开始构建父节点，直到根节点
	for i := 0; i < len(data)/2; i++ {
		var newLevel []MerkleNode

		// 每两个节点合并成一个父节点
		for j := 0; j < len(nodes); j += 2 {
			node := NewMerkleNode(&nodes[j], &nodes[j+1], nil) // 创建父节点
			newLevel = append(newLevel, *node)                  // 添加到新的一层节点
		}

		// 更新当前节点层次
		nodes = newLevel
	}

	// 创建并返回 MerkleTree，根节点是最后一层的第一个节点
	tree := MerkleTree{&nodes[0]}

	return &tree
}
