package blockchain

import (
	"bytes"
	"encoding/gob"
	"log"
	"time"
)

// Block 结构体表示区块链中的一个区块
type Block struct {
	Timestamp    int64          // 区块创建时间戳
	Hash         []byte         // 当前区块的哈希值
	Transactions []*Transaction // 区块中包含的交易列表
	PrevHash     []byte         // 上一个区块的哈希值
	Nonce        int            // 用于工作量证明（PoW）的随机数
	Height       int            // 区块高度（区块在区块链中的位置）
}

// HashTransactions 方法计算并返回区块中所有交易的 Merkle 树的根哈希
func (b *Block) HashTransactions() []byte {
	var txHashes [][]byte

	// 对区块中的每一笔交易进行序列化，并计算哈希
	for _, tx := range b.Transactions {
		txHashes = append(txHashes, tx.Serialize())
	}

	// 使用 Merkle 树计算所有交易的哈希
	tree := NewMerkleTree(txHashes)

	// 返回 Merkle 树根节点的哈希值
	return tree.RootNode.Data
}

// CreateBlock 创建一个新的区块，并计算该区块的哈希值
func CreateBlock(txs []*Transaction, prevHash []byte, height int) *Block {
	// 创建一个新的区块，区块的时间戳、上一个区块哈希值、交易列表、区块高度等信息
	block := &Block{time.Now().Unix(), []byte{}, txs, prevHash, 0, height}

	// 创建一个工作量证明对象并运行 PoW 算法来获取 nonce 和区块的哈希
	pow := NewProof(block)
	nonce, hash := pow.Run()

	// 设置区块的哈希和 nonce
	block.Hash = hash[:]
	block.Nonce = nonce

	// 返回创建的区块
	return block
}

// Genesis 创建创世区块（区块链的第一个区块）
func Genesis(coinbase *Transaction) *Block {
	// 创建创世区块，coinbase 是包含挖矿奖励的交易
	return CreateBlock([]*Transaction{coinbase}, []byte{}, 0)
}

// Serialize 将区块序列化为字节数组，便于存储或网络传输
func (b *Block) Serialize() []byte {
	var res bytes.Buffer
	encoder := gob.NewEncoder(&res)

	// 编码区块对象
	err := encoder.Encode(b)

	Handle(err)

	return res.Bytes()
}

// Deserialize 将字节数组反序列化为区块对象
func Deserialize(data []byte) *Block {
	var block Block

	decoder := gob.NewDecoder(bytes.NewReader(data))

	// 解码字节数组为区块对象
	err := decoder.Decode(&block)

	Handle(err)

	return &block
}

// Handle 用于处理错误，如果有错误则触发 panic
func Handle(err error) {
	if err != nil {
		log.Panic(err)
	}
}
