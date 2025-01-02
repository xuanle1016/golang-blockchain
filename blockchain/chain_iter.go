package blockchain

import "github.com/dgraph-io/badger/v3"

// BlockChainIterator 结构体表示一个区块链的迭代器，用于从最后一个区块向创世区块迭代
type BlockChainIterator struct {
	CurrentHash []byte     // 当前区块的哈希
	Database    *badger.DB // 区块链的数据库
}

// Iterator 返回一个新的区块链迭代器，初始化为区块链的最后一个区块
func (chain *BlockChain) Iterator() *BlockChainIterator {
	iter := &BlockChainIterator{chain.LastHash, chain.Database}
	return iter
}

// Next 从当前区块向创世区块迭代，并返回下一个区块
func (iter *BlockChainIterator) Next() *Block {
	var block *Block

	// 使用数据库的视图事务获取当前区块的数据
	err := iter.Database.View(func(txn *badger.Txn) error {
		// 获取当前区块的条目
		item, err := txn.Get(iter.CurrentHash)
		Handle(err)

		// 通过回调获取区块的编码数据并反序列化为区块对象
		err = item.Value(func(val []byte) error {
			block = Deserialize(val) // 反序列化区块数据
			return nil
		})
		Handle(err)

		return nil
	})

	Handle(err)

	// 更新迭代器的当前哈希值为当前区块的前一个区块哈希
	iter.CurrentHash = block.PrevHash

	// 返回当前区块
	return block
}
