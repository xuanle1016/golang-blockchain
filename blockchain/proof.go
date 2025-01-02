package blockchain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"math/big"
)

// 挖矿的难度（目标哈希的前几位必须是 0，数字越大难度越高）
const Difficulty = 18

// ProofOfWork 结构体，用于工作量证明（PoW）算法
type ProofOfWork struct {
	Block  *Block   // 当前区块
	Target *big.Int // 目标值，用于比较哈希值
}

// NewProof 创建一个新的工作量证明
// 输入为区块，返回包含目标值和区块的 ProofOfWork 对象
func NewProof(b *Block) *ProofOfWork {
	target := big.NewInt(1)
	// 左移操作，调整目标值以满足难度要求
	target.Lsh(target, uint(256-Difficulty)) // Lsh: 左移位数

	pow := &ProofOfWork{b, target}
	return pow
}

// InitData 初始化数据，用于生成哈希值
// 包含前一区块哈希、交易数据的哈希值、随机数（nonce）和难度值
func (pow *ProofOfWork) InitData(nonce int) []byte {
	data := bytes.Join(
		[][]byte{
			pow.Block.PrevHash,           // 前一区块哈希
			pow.Block.HashTransactions(), // 当前区块交易数据的哈希
			ToHex(int64(nonce)),          // 随机数
			ToHex(int64(Difficulty)),     // 难度值
		},
		[]byte{}, // 空的分隔符
	)
	return data
}

// Run 执行工作量证明算法
// 寻找满足条件的随机数（nonce），返回随机数和对应的哈希值
func (pow *ProofOfWork) Run() (int, []byte) {
	var intHash big.Int // 用于存储哈希值的整数表示
	var hash [32]byte   // 当前计算的哈希值

	nonce := 0 // 随机数从 0 开始

	// 不断尝试随机数，直到找到满足条件的哈希值或达到最大值
	for nonce < math.MaxInt64 {
		data := pow.InitData(nonce)    // 初始化数据
		hash = sha256.Sum256(data)     // 计算哈希值
		fmt.Printf("\r%x", hash)       // 打印当前哈希值（动态更新）
		intHash.SetBytes(hash[:])      // 将哈希值转换为大整数

		// 如果当前哈希值小于目标值，说明找到合适的随机数
		if intHash.Cmp(pow.Target) == -1 {
			break
		} else {
			nonce++ // 否则增加随机数继续尝试
		}
	}
	fmt.Println() // 打印空行

	return nonce, hash[:] // 返回找到的随机数和哈希值
}

// Validate 验证区块是否满足工作量证明的条件
// 根据随机数重新计算哈希值并与目标值比较
func (pow *ProofOfWork) Validate() bool {
	var intHash big.Int

	data := pow.InitData(pow.Block.Nonce) // 根据区块的随机数初始化数据
	hash := sha256.Sum256(data)           // 计算哈希值
	intHash.SetBytes(hash[:])             // 将哈希值转换为大整数

	// 比较哈希值和目标值，小于目标值则有效
	return intHash.Cmp(pow.Target) == -1
}

// ToHex 将整数转换为字节数组（大端序）
// 用于生成哈希时的输入数据
func ToHex(num int64) []byte {
	buff := new(bytes.Buffer)                              // 创建缓冲区
	err := binary.Write(buff, binary.BigEndian, num)       // 写入整数（大端序）
	if err != nil {
		log.Panic(err) // 如果写入失败，记录日志并终止程序
	}
	return buff.Bytes() // 返回字节数组
}
