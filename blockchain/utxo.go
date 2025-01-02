package blockchain

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"

	"github.com/dgraph-io/badger/v3"
)

var (
	utxoPrefix   = []byte("utxo-") // UTXO数据的前缀
	prefixLength = len(utxoPrefix) // 前缀的长度
)

// UTXOSet 结构体表示一个UTXO集合，它与区块链相关联
type UTXOSet struct {
	Blockchain *BlockChain // 区块链
}

// FindSpendableOutputs 查找可花费的输出（UTXO）
func (u UTXOSet) FindSpendableOutputs(pubKeyHash []byte, amount int) (int, map[string][]int) {
	fmt.Printf("Finding spendable outputs for: %x\n", pubKeyHash)
	unspentOuts := make(map[string][]int) // 存储可用的UTXO
	accumulated := 0 // 累积的金额
	db := u.Blockchain.Database // 获取数据库实例

	// 使用Badger数据库的视图事务
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions // 默认迭代器选项

		it := txn.NewIterator(opts) // 创建一个迭代器
		defer it.Close()

		// 遍历所有UTXO条目
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			k := item.Key()
			var v []byte
			// 获取UTXO值
			err := item.Value(func(val []byte) error {
				fmt.Printf("Checking UTXO key: %x\n", k)
				v = val
				return nil
			})
			Handle(err)
			k = bytes.TrimPrefix(k, utxoPrefix) // 去掉前缀
			txID := hex.EncodeToString(k) // 获取交易ID
			outs := DeserializeOutputs(v) // 反序列化输出

			// 遍历每个输出并判断是否满足条件
			for outIdx, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) && accumulated < amount {
					accumulated += out.Value
					unspentOuts[txID] = append(unspentOuts[txID], outIdx)
				}
			}
		}
		return nil
	})
	Handle(err)

	fmt.Printf("Accumulated: %d, UnspentOuts: %+v\n", accumulated, unspentOuts)
	return accumulated, unspentOuts
}

// Reindex 重新索引UTXO集合
func (u UTXOSet) Reindex() {
	fmt.Println("Reindexing UTXO set...")
	db := u.Blockchain.Database

	// 删除旧的UTXO数据
	u.DeleteByPrefix(utxoPrefix)

	// 查找区块链中的UTXO
	UTXO := u.Blockchain.FindUTXO()
	fmt.Printf("Found UTXOs: %+v\n", UTXO)

	// 将UTXO数据重新保存到数据库
	err := db.Update(func(txn *badger.Txn) error {
		for txId, outs := range UTXO {
			key, err := hex.DecodeString(txId) // 解码交易ID
			Handle(err)
			key = append(utxoPrefix, key...) // 加上前缀
			fmt.Printf("Adding UTXO for txId: %s\n", txId)
			err = txn.Set(key, outs.Serialize()) // 保存UTXO
			Handle(err)
		}

		return nil
	})
	Handle(err)
	fmt.Println("UTXO set reindexed")
}

// DeleteByPrefix 根据前缀删除数据
func (u *UTXOSet) DeleteByPrefix(prefix []byte) {
	// 定义删除数据的方法
	deleteKeys := func(keysForDelete [][]byte) error {
		if err := u.Blockchain.Database.Update(func(txn *badger.Txn) error {
			for _, key := range keysForDelete {
				if err := txn.Delete(key); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}
		return nil
	}

	collectSize := 100000 // 每次删除的键的数量
	u.Blockchain.Database.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.PrefetchValues = false
		it := txn.NewIterator(opts)
		defer it.Close()

		keysForDelete := make([][]byte, 0, collectSize)
		keysCollected := 0
		// 遍历所有数据并准备删除
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {
			key := it.Item().KeyCopy(nil)
			keysForDelete = append(keysForDelete, key)
			keysCollected++
			if keysCollected == collectSize {
				// 批量删除
				if err := deleteKeys(keysForDelete); err != nil {
					log.Panic(err)
				}
				keysForDelete = make([][]byte, 0, collectSize)
				keysCollected = 0
			}
		}
		// 删除剩余的键
		if keysCollected > 0 {
			if err := deleteKeys(keysForDelete); err != nil {
				log.Panic(err)
			}
		}
		return nil
	})
}

// Update 更新UTXO集合（每次区块添加时调用）
func (u *UTXOSet) Update(block *Block) {
	db := u.Blockchain.Database

	// 更新UTXO集合
	err := db.Update(func(txn *badger.Txn) error {
		for _, tx := range block.Transactions {
			if !tx.IsCoinbase() { // 排除coinbase交易
				for _, in := range tx.Inputs {
					updatedOuts := TxOutputs{}
					inID := append(utxoPrefix, in.ID...) // 输入的UTXO ID
					item, err := txn.Get(inID)
					Handle(err)
					var v []byte
					// 获取UTXO值
					err = item.Value(func(val []byte) error {
						v = val
						return nil
					})
					Handle(err)

					outs := DeserializeOutputs(v) // 反序列化输出

					// 更新UTXO（如果输入没有被花费）
					for outIdx, out := range outs.Outputs {
						if outIdx != in.Out {
							updatedOuts.Outputs = append(updatedOuts.Outputs, out)
						}
					}

					// 如果输出为空，则删除该UTXO
					if len(updatedOuts.Outputs) == 0 {
						if err := txn.Delete(inID); err != nil {
							log.Panic(err)
						}
					} else {
						if err := txn.Set(inID, updatedOuts.Serialize()); err != nil {
							log.Panic(err)
						}
					}
				}
			}

			// 新的交易输出
			newOutputs := TxOutputs{
				Outputs: append([]TxOutput{}, tx.Outputs...),
			}

			// 将新交易的输出存入数据库
			txID := append(utxoPrefix, tx.ID...)
			if err := txn.Set(txID, newOutputs.Serialize()); err != nil {
				log.Panic(err)
			}
		}

		return nil
	})
	Handle(err)
}

// FindUnspentTransactions 查找所有未花费的交易输出
func (u UTXOSet) FindUnspentTransactions(pubKeyHash []byte) []TxOutput {
	var UTXOs []TxOutput

	db := u.Blockchain.Database

	// 使用数据库视图事务查找所有UTXO
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()

		// 遍历所有UTXO并筛选出符合条件的
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			item := it.Item()
			var v []byte
			err := item.Value(func(val []byte) error {
				v = val
				return nil
			})
			Handle(err)
			outs := DeserializeOutputs(v)

			// 筛选与给定公钥哈希匹配的输出
			for _, out := range outs.Outputs {
				if out.IsLockedWithKey(pubKeyHash) {
					UTXOs = append(UTXOs, out)
				}
			}
		}

		return nil
	})
	Handle(err)

	return UTXOs
}

// CountTransactions 计算数据库中存储的交易数量
func (u UTXOSet) CountTransactions() int {
	db := u.Blockchain.Database
	counter := 0

	// 使用数据库视图事务统计交易数量
	err := db.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions

		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Seek(utxoPrefix); it.ValidForPrefix(utxoPrefix); it.Next() {
			counter++
		}

		return nil
	})

	Handle(err)

	return counter
}
