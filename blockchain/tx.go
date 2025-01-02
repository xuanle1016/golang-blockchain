package blockchain

import (
	"bytes"
	"encoding/gob"

	"github.com/xuanle1016/golang-blockchain/wallet"
)

// TxOutput 表示交易的输出
type TxOutput struct {
	Value      int    // 输出的金额
	PubKeyHash []byte // 锁定该输出的公钥哈希
}

// TxOutputs 表示多个交易输出的集合
type TxOutputs struct {
	Outputs []TxOutput
}

// TxInput 表示交易的输入
type TxInput struct {
	ID        []byte // 引用的交易 ID
	Out       int    // 该输入引用的输出在交易中的索引
	Signature []byte // 交易的数字签名
	PubKey    []byte // 公钥
}

// UsesKey 检查输入是否使用了特定的公钥哈希进行解锁
func (in *TxInput) UsesKey(pubKeyHash []byte) bool {
	// 获取输入中公钥的哈希值
	lockingHash := wallet.PublicKeyHash(in.PubKey)

	// 比较公钥哈希是否匹配
	return bytes.Equal(lockingHash, pubKeyHash)
}

// Lock 锁定输出，使其只能由特定地址的私钥解锁
func (out *TxOutput) Lock(address []byte) {
	// 解码地址为 Base58 格式
	pubKeyHash := wallet.Base58Decode(address)
	// 移除地址中的版本和校验码，提取公钥哈希
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	// 设置输出的公钥哈希
	out.PubKeyHash = pubKeyHash
}

// IsLockedWithKey 检查输出是否被特定的公钥哈希锁定
func (out *TxOutput) IsLockedWithKey(pubKeyHash []byte) bool {
	// 比较输出的公钥哈希和输入的公钥哈希
	return bytes.Equal(out.PubKeyHash, pubKeyHash)
}

// NewTXOutput 创建一个新的交易输出并锁定到指定的地址
func NewTXOutput(value int, address string) *TxOutput {
	// 创建一个新的交易输出
	txo := &TxOutput{value, nil}
	// 锁定输出到指定地址
	txo.Lock([]byte(address))

	return txo
}

// Serialize 序列化 TxOutputs 结构体为字节数组
func (outs TxOutputs) Serialize() []byte {
	var buffer bytes.Buffer
	// 创建编码器
	encode := gob.NewEncoder(&buffer)
	// 将结构体编码到 buffer
	err := encode.Encode(outs)
	Handle(err)

	return buffer.Bytes()
}

// DeserializeOutputs 反序列化字节数组为 TxOutputs 结构体
func DeserializeOutputs(data []byte) TxOutputs {
	var outputs TxOutputs
	// 创建解码器
	decode := gob.NewDecoder(bytes.NewReader(data))
	// 将字节数组解码为结构体
	err := decode.Decode(&outputs)
	Handle(err)

	return outputs
}
