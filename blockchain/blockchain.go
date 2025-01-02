package blockchain

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/dgraph-io/badger/v3"
)

const (
	dbPath      = "./tmp/blocks_%s"         // 数据库路径模板
	genesisData = "First Transaction from Genesis" // 创世块的交易数据
)

// BlockChain 结构表示区块链
type BlockChain struct {
	LastHash []byte // 链中最后一个区块的哈希值
	Database *badger.DB // 存储区块链数据的数据库
}

// DBexists 检查数据库是否存在
func DBexists(path string) bool {
	// 检查指定路径下是否有 MANIFEST 文件，如果没有则说明数据库不存在
	if _, err := os.Stat(path + "/MANIFEST"); os.IsNotExist(err) {
		return false
	}
	return true
}

// ContinueBlockChain 连接到已存在的区块链
func ContinueBlockChain(nodeId string) *BlockChain {
	// 根据 nodeId 创建数据库路径
	path := fmt.Sprintf(dbPath, nodeId)
	// 如果数据库不存在，则打印信息并退出
	if !DBexists(path) {
		fmt.Println("No existing blockchain found, create one!")
		runtime.Goexit() // 退出程序
	}

	var lastHash []byte

	// 初始化 Badger 数据库选项
	opts := badger.DefaultOptions(path).
		WithLogger(nil).               // 禁用日志
		WithLoggingLevel(badger.ERROR) // 只显示错误日志
	opts.Dir = path
	opts.ValueDir = path

	// 打开数据库
	db, err := openDB(path, opts)
	Handle(err)

	// 从数据库中获取最后一个区块的哈希
	err = db.Update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = make([]byte, len(val))
			copy(lastHash, val)
			return nil
		})
		return err
	})
	Handle(err)

	// 返回区块链实例
	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

// InitBlockChain 初始化一个新的区块链
func InitBlockChain(address, nodeId string) *BlockChain {
	// 根据 nodeId 创建数据库路径
	path := fmt.Sprintf(dbPath, nodeId)
	var lastHash []byte

	// 如果数据库已经存在，则退出
	if DBexists(path) {
		fmt.Println("Blockchain already exists")
		runtime.Goexit()
	}

	// 初始化 Badger 数据库选项
	opts := badger.DefaultOptions(path).
		WithLogger(nil).               // 禁用日志
		WithLoggingLevel(badger.ERROR) // 只显示错误日志
	opts.Dir = path
	opts.ValueDir = path

	// 打开数据库
	db, err := openDB(path, opts)
	Handle(err)

	// 创建创世块并存储到数据库中
	err = db.Update(func(txn *badger.Txn) error {
		cbtx := CoinbaseTx(address, genesisData) // 创世块的 coinbase 交易
		genesis := Genesis(cbtx)                // 创建创世块

		// 将创世块存储到数据库
		err = txn.Set(genesis.Hash, genesis.Serialize())
		Handle(err)

		// 存储最后一个区块的哈希
		err = txn.Set([]byte("lh"), genesis.Hash)
		lastHash = genesis.Hash
		return err
	})
	Handle(err)

	// 返回区块链实例
	blockchain := BlockChain{lastHash, db}
	return &blockchain
}

// AddBlock 添加新块到区块链
func (chain *BlockChain) AddBlock(block *Block) {
	var lastHash []byte
	var lastBlockData []byte

	err := chain.Database.Update(func(txn *badger.Txn) error {
		// 如果块已存在，则返回
		if _, err := txn.Get(block.Hash); err == nil {
			return nil
		}

		// 将块数据存储到数据库中
		blockData := block.Serialize()
		err := txn.Set(block.Hash, blockData)
		Handle(err)

		// 获取链的最后一个区块的哈希
		item, err := txn.Get([]byte("lh"))
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastHash = make([]byte, len(val))
			copy(lastHash, val)
			return nil
		})
		Handle(err)

		// 获取最后一个区块数据
		item, err = txn.Get(lastHash)
		Handle(err)
		err = item.Value(func(val []byte) error {
			lastBlock := Deserialize(val)
			lastBlockData = lastBlock.Serialize()
			return nil
		})
		Handle(err)

		lastBlock := Deserialize(lastBlockData)

		// 如果新块高度高于最后一个块，则更新最后哈希
		if block.Height > lastBlock.Height {
			err = txn.Set([]byte("lh"), block.Hash)
			Handle(err)
			chain.LastHash = block.Hash
		}

		return nil
	})
	Handle(err)
}

// GetBlock 获取指定哈希的区块
func (chain *BlockChain) GetBlock(blockHash []byte) (Block, error) {
	var block Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get(blockHash)
		if err != nil {
			return errors.New("Block is not found")
		}

		return item.Value(func(val []byte) error {
			block = *Deserialize(val)
			return nil
		})
	})
	if err != nil {
		return block, err
	}

	return block, nil
}

// GetBlockHashes 获取区块链中所有区块的哈希值
func (chain *BlockChain) GetBlockHashes() [][]byte {
	var blocks [][]byte

	iter := chain.Iterator()

	for {
		block := iter.Next()

		blocks = append(blocks, block.Hash)

		if len(block.PrevHash) == 0 {
			break
		}
	}

	return blocks
}

// GetBestHeight 获取当前区块链的最大高度
func (chain *BlockChain) GetBestHeight() int {
	var lastBlock Block

	err := chain.Database.View(func(txn *badger.Txn) error {
		// 获取最后一个区块的哈希
		item, err := txn.Get([]byte("lh"))
		if err != nil {
			return err
		}
		var lastHash []byte
		err = item.Value(func(val []byte) error {
			lastHash = append([]byte{}, val...)
			return nil
		})
		if err != nil {
			return err
		}

		// 获取最后一个区块
		item, err = txn.Get(lastHash)
		if err != nil {
			return err
		}
		err = item.Value(func(val []byte) error {
			lastBlock = *Deserialize(val)
			return nil
		})
		return err
	})
	if err != nil {
		Handle(err)
	}

	return lastBlock.Height
}

// MineBlock 挖掘新块并将其添加到区块链
func (chain *BlockChain) MineBlock(txs []*Transaction) *Block {
	var lastHash []byte
	var lastHeight int
	var lastBlockData []byte

	// 获取最后一个区块的哈希
	err := chain.Database.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte("lh"))
		Handle(err)

		// 使用回调获取值
		err = item.Value(func(val []byte) error {
			lastHash = make([]byte, len(val))
			copy(lastHash, val)
			return nil
		})
		Handle(err)

		err = item.Value(func(val []byte) error {
			lastBlockData = make([]byte, len(val))
			copy(lastBlockData, val)
			return nil
		})
		lastBlock := Deserialize(lastBlockData)
		lastHeight = lastBlock.Height

		return err
	})
	Handle(err)

	// 创建一个新的区块
	newBlock := CreateBlock(txs, lastHash, lastHeight+1)

	// 更新数据库，将新块及其哈希值存储
	err = chain.Database.Update(func(txn *badger.Txn) error {
		// 存储新块
		err := txn.Set(newBlock.Hash, newBlock.Serialize())
		Handle(err)

		// 更新最后一个区块的哈希指针
		err = txn.Set([]byte("lh"), newBlock.Hash)
		Handle(err)

		// 更新内存中的最后一个哈希
		chain.LastHash = newBlock.Hash

		return nil
	})
	Handle(err)

	return newBlock
}

// FindUTXO 查找所有未花费的交易输出（UTXO）
func (chain *BlockChain) FindUTXO() map[string]TxOutputs {
	UTXO := make(map[string]TxOutputs)
	spentTXOs := make(map[string][]int)

	iter := chain.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID)

		Outputs:
			for outIdx, out := range tx.Outputs {
				// 如果该输出已被花费，则跳过
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] {
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}
				outs := UTXO[txID]
				outs.Outputs = append(outs.Outputs, out)
				UTXO[txID] = outs
			}

			// 如果不是 coinbase 交易，则标记为已花费
			if !tx.IsCoinbase() {
				for _, in := range tx.Inputs {
					inTxID := hex.EncodeToString(in.ID)
					spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Out)
				}
			}
		}

		if len(block.PrevHash) == 0 {
			break
		}
	}
	return UTXO
}

// FindTransaction 查找指定 ID 的交易
func (bc *BlockChain) FindTransaction(ID []byte) (Transaction, error) {
	iter := bc.Iterator()

	for {
		block := iter.Next()

		for _, tx := range block.Transactions {
			// 如果找到匹配的交易，则返回
			if bytes.Equal(tx.ID, ID) {
				return *tx, nil
			}
		}

		// 如果区块链结束，且未找到交易，则返回错误
		if len(block.PrevHash) == 0 {
			break
		}
	}

	return Transaction{}, errors.New("Transaction does not exist")
}

// SignTransaction 对交易进行签名
func (bc *BlockChain) SignTransaction(tx *Transaction, privKey ecdsa.PrivateKey) {
	prevTXs := make(map[string]Transaction)

	for _, in := range tx.Inputs {
		// 查找并获取输入的交易
		prevTX, err := bc.FindTransaction(in.ID)
		Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	tx.Sign(privKey, prevTXs)
}

// VerifyTransaction 验证交易的签名
func (bc *BlockChain) VerifyTransaction(tx *Transaction) bool {
	prevTXs := make(map[string]Transaction)

	// 获取交易输入的历史交易数据
	for _, in := range tx.Inputs {
		prevTX, err := bc.FindTransaction(in.ID)
		Handle(err)
		prevTXs[hex.EncodeToString(prevTX.ID)] = prevTX
	}

	return tx.Verify(prevTXs)
}


// retry 函数尝试重新打开数据库，解决 "LOCK" 锁文件导致的数据库无法打开的问题。
// 它会删除数据库目录中的 "LOCK" 文件，并重试打开数据库。
func retry(dir string, originalOpts badger.Options) (*badger.DB, error) {
	lockPath := filepath.Join(dir, "LOCK") // 构造 "LOCK" 文件的路径

	// 尝试删除 "LOCK" 文件，可能因为数据库意外关闭留下了该文件
	if err := os.Remove(lockPath); err != nil {
		return nil, fmt.Errorf(`removing "LOCK": %s`, err) // 如果删除 "LOCK" 文件失败，返回错误
	}

	retryOpts := originalOpts // 复制原始的数据库选项
	db, err := badger.Open(retryOpts) // 尝试重新打开数据库
	return db, err
}

// openDB 函数用于打开 Badger 数据库。
// 如果数据库因为 "LOCK" 锁文件而无法打开，它会尝试删除锁文件并重试打开数据库。
func openDB(dir string, opts badger.Options) (*badger.DB, error) {
	// 尝试打开数据库
	if db, err := badger.Open(opts); err != nil {
		// 如果错误信息包含 "LOCK"，说明数据库被锁定
		if strings.Contains(err.Error(), "LOCK") {
			// 尝试删除锁文件并重新打开数据库
			if db, err := retry(dir, opts); err == nil {
				log.Println("database unlocked, value log truncated") // 输出数据库已解锁的信息
				return db, nil // 成功解锁数据库并打开
			}
			log.Println("could not unlock database:", err) // 如果无法解锁，输出错误信息
		}
		return nil, err // 如果其他错误，直接返回错误
	} else {
		return db, nil // 成功打开数据库，返回数据库实例
	}
}
