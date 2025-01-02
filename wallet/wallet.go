package wallet

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"log"
	"math/big"

	"golang.org/x/crypto/ripemd160"
)

const (
	checksumLength = 4 // 校验和的长度
	version        = byte(0x00) // 地址的版本号
)

// Wallet 结构体表示一个钱包
type Wallet struct {
	PrivateKey []byte // 私钥
	PublicKey  []byte // 公钥
}

// DeserializePrivateKey 反序列化私钥
// 输入为字节数组，返回 ECDSA 私钥对象
func DeserializePrivateKey(privateBytes []byte) *ecdsa.PrivateKey {
	curve := elliptic.P256() // 使用椭圆曲线 P-256

	private := new(ecdsa.PrivateKey)
	private.D = new(big.Int).SetBytes(privateBytes) // 设置私钥的 D 值
	private.PublicKey.Curve = curve
	// 使用私钥生成公钥
	private.PublicKey.X, private.PublicKey.Y = curve.ScalarBaseMult(privateBytes)

	return private
}

// DeserializePublicKey 反序列化公钥
// 输入为字节数组，返回 ECDSA 公钥对象
func DeserializePublicKey(publicBytes []byte) *ecdsa.PublicKey {
	curve := elliptic.P256() // 使用椭圆曲线 P-256
	// 从字节数组中解析 X 和 Y 坐标
	x := new(big.Int).SetBytes(publicBytes[:len(publicBytes)/2])
	y := new(big.Int).SetBytes(publicBytes[len(publicBytes)/2:])

	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}
}

// Address 生成钱包地址
// 地址包含公钥的哈希值、版本号和校验和
func (w Wallet) Address() []byte {
	pubHash := PublicKeyHash(w.PublicKey) // 计算公钥的哈希

	// 添加版本号
	versionedHash := append([]byte{version}, pubHash...)
	// 计算校验和
	checksum := Checksum(versionedHash)

	// 拼接完整的哈希值
	fullHash := append(versionedHash, checksum...)
	// 使用 Base58 编码生成地址
	address := Base58Encode(fullHash)

	return address
}

// ValidateAddress 验证地址是否有效
// 输入为地址字符串，返回布尔值
func ValidateAddress(address string) bool {
	pubKeyHash := Base58Decode([]byte(address))        // 解码地址
	actualChecksum := pubKeyHash[len(pubKeyHash)-checksumLength:] // 提取校验和
	version := pubKeyHash[0]                           // 提取版本号
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-checksumLength]   // 提取公钥哈希部分
	// 计算目标校验和
	targetChecksum := Checksum(append([]byte{version}, pubKeyHash...))

	// 比较实际校验和与目标校验和
	return bytes.Equal(actualChecksum, targetChecksum)
}

// NewKeyPair 生成新的公钥和私钥对
func NewKeyPair() ([]byte, []byte) {
	curve := elliptic.P256() // 使用椭圆曲线 P-256

	// 生成私钥
	private, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		log.Panic(err)
	}

	// 序列化私钥 (D 值)
	privateBytes := private.D.Bytes()

	// 序列化公钥 (X || Y)
	publicBytes := append(private.PublicKey.X.Bytes(), private.PublicKey.Y.Bytes()...)

	return privateBytes, publicBytes
}

// MakeWallet 创建一个新的钱包
func MakeWallet() *Wallet {
	private, public := NewKeyPair() // 生成公钥和私钥
	wallet := Wallet{PrivateKey: private, PublicKey: public} // 创建钱包

	return &wallet
}

// PublicKeyHash 计算公钥的哈希值
// 先进行 SHA-256 再进行 RIPEMD-160 哈希
func PublicKeyHash(pubKey []byte) []byte {
	pubHash := sha256.Sum256(pubKey) // SHA-256 哈希

	// 使用 RIPEMD-160 哈希
	hasher := ripemd160.New()
	_, err := hasher.Write(pubHash[:])
	if err != nil {
		log.Panic(err)
	}

	publicRipMD := hasher.Sum(nil) // 得到哈希值

	return publicRipMD
}

// Checksum 计算校验和
// 输入为载荷，返回校验和
func Checksum(payload []byte) []byte {
	firstHash := sha256.Sum256(payload)  // 第一次 SHA-256 哈希
	secondHash := sha256.Sum256(firstHash[:]) // 第二次 SHA-256 哈希

	return secondHash[:checksumLength] // 返回前 checksumLength 位
}
