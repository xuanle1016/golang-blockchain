package wallet

import (
	"bytes"
	"crypto/elliptic"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"log"
	"os"
)

const walletFile = "./tmp/wallets_%s.data" // 定义钱包文件的存储路径模板，%s 会替换为节点ID

// Wallets 结构体用于存储多个钱包
type Wallets struct {
	Wallets map[string]*Wallet // 使用映射存储钱包，键为钱包地址，值为对应的 Wallet 对象
}

// CreateWallets 创建一个新的 Wallets 实例，并加载与 nodeId 相关的已有钱包文件
func CreateWallets(nodeId string) (*Wallets, error) {
	wallets := Wallets{}
	wallets.Wallets = make(map[string]*Wallet) // 初始化钱包映射

	// 加载与 nodeId 相关的钱包文件
	err := wallets.LoadFile(nodeId)
	return &wallets, err
}

// AddWallet 创建一个新的钱包并将其添加到 Wallets 中，返回钱包的地址
func (ws *Wallets) AddWallet() string {
	wallet := MakeWallet()       // 创建一个新的钱包
	address := string(wallet.Address()) // 获取钱包地址并转换为字符串

	// 将钱包添加到映射中
	ws.Wallets[address] = wallet

	return address // 返回钱包地址
}

// GetAllAddress 获取所有钱包的地址并返回地址的切片
func (ws *Wallets) GetAllAddress() []string {
	var addresses []string

	// 遍历钱包映射，收集所有钱包的地址
	for address := range ws.Wallets {
		addresses = append(addresses, address)
	}

	return addresses // 返回钱包地址的切片
}

// GetWallet 根据地址获取对应的钱包
func (ws Wallets) GetWallet(address string) Wallet {
	return *ws.Wallets[address] // 返回对应地址的钱包
}

// LoadFile 从文件中加载钱包数据，如果文件不存在则返回错误
func (ws *Wallets) LoadFile(nodeID string) error {
	walletFile := fmt.Sprintf(walletFile, nodeID) // 使用 nodeID 构造钱包文件路径
	if _, err := os.Stat(walletFile); os.IsNotExist(err) {
		return err // 如果文件不存在，返回错误
	}

	var wallets Wallets // 创建 Wallets 结构体用于解码文件内容

	// 读取钱包文件内容
	fileContent, err := ioutil.ReadFile(walletFile)
	if err != nil {
		log.Panic(err) // 读取文件失败则 panic
	}

	gob.Register(elliptic.P256()) // 注册椭圆曲线算法
	decoder := gob.NewDecoder(bytes.NewReader(fileContent)) // 创建解码器
	err = decoder.Decode(&wallets) // 解码文件内容到 wallets 变量
	if err != nil {
		log.Panic(err) // 解码失败则 panic
	}

	// 将解码后的钱包数据赋值给当前 Wallets 实例
	ws.Wallets = wallets.Wallets

	return nil // 加载成功，返回 nil
}

// SaveFile 将当前 Wallets 的数据保存到文件
func (ws *Wallets) SaveFile(nodeId string) {
	var content bytes.Buffer
	walletFile := fmt.Sprintf(walletFile, nodeId) // 使用 nodeId 构造钱包文件路径

	gob.Register(elliptic.P256()) // 注册椭圆曲线算法

	encoder := gob.NewEncoder(&content) // 创建编码器
	err := encoder.Encode(ws) // 编码当前 Wallets 实例
	if err != nil {
		log.Panic(err) // 编码失败则 panic
	}

	// 将编码后的内容写入文件
	err = ioutil.WriteFile(walletFile, content.Bytes(), 0644)
	if err != nil {
		log.Panic(err) // 写入文件失败则 panic
	}
}
