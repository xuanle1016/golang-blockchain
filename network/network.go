package network

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"syscall"
	"runtime"
	"os"

	"github.com/vrecan/death/v3"

	"github.com/xuanle1016/golang-blockchain/blockchain"
)

const (
	protocol      = "tcp"           // 网络协议，使用 TCP
	version       = 1               // 协议版本
	commandLength = 12              // 命令的长度
)

var (
	nodeAddress     string                      // 当前节点地址
	mineAddress     string                      // 挖矿地址
	KnownNodes      = []string{"localhost:3000"} // 已知节点列表
	blocksInTransit = [][]byte{}               // 正在传输的区块
	memoryPool      = make(map[string]blockchain.Transaction) // 存储未确认的交易
)

// Addr 类型表示节点地址列表
type Addr struct {
	AddrList []string
}

// Block 类型表示一个区块，包含发送方地址和区块数据
type Block struct {
	AddrFrom string
	Block    []byte
}

// GetBlocks 类型表示获取区块的请求
type GetBlocks struct {
	AddrFrom string
}

// GetData 类型表示获取数据（区块或交易）的请求
type GetData struct {
	AddrFrom string
	Type     string
	ID       []byte
}

// Inv 类型表示节点的库存（区块或交易）
type Inv struct {
	AddrFrom string
	Type     string
	Items    [][]byte
}

// Tx 类型表示交易数据
type Tx struct {
	AddrFrom    string
	Transaction []byte
}

// Version 类型表示协议版本及区块链的高度
type Version struct {
	Version    int
	BestHeight int
	AddrFrom   string
}

// CmdToBytes 将命令字符串转换为字节数组
func CmdToBytes(cmd string) []byte {
	var bytes [commandLength]byte

	for i, c := range cmd {
		bytes[i] = byte(c)
	}

	return bytes[:]
}

// BytesToCmd 将字节数组转换为命令字符串
func BytesToCmd(bytes []byte) string {
	var cmd []byte

	for _, b := range bytes {
		if b != 0x0 {
			cmd = append(cmd, b)
		}
	}

	return string(cmd)
}

// ExtractCmd 从请求中提取命令部分
func ExtractCmd(request []byte) []byte {
	return request[:commandLength]
}

// RequestBlocks 向已知节点请求区块
func RequestBlocks() {
	for _, node := range KnownNodes {
		SendGetBlocks(node)
	}
}

// SendAddr 发送节点地址的请求
func SendAddr(address string) {
	nodes := Addr{KnownNodes}
	nodes.AddrList = append(nodes.AddrList, nodeAddress)
	payload := GobEncode(nodes)
	request := append(CmdToBytes("addr"), payload...)

	SendData(address, request)
}

// SendBlock 发送区块数据
func SendBlock(addr string, b *blockchain.Block) {
	data := Block{nodeAddress, b.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("block"), payload...)

	SendData(addr, request)
}

// SendData 发送数据到指定地址
func SendData(addr string, data []byte) {
	conn, err := net.Dial(protocol, addr)

	if err != nil {
		fmt.Printf("%s is not available\n", addr)
		var updatedNodes []string

		for _, node := range KnownNodes {
			if node != addr {
				updatedNodes = append(updatedNodes, node)
			}
		}

		KnownNodes = updatedNodes

		return
	}

	defer conn.Close()

	_, err = io.Copy(conn, bytes.NewReader(data))
	if err != nil {
		log.Panic(err)
	}
}

// SendInv 发送库存（区块或交易）数据
func SendInv(address, kind string, items [][]byte) {
	inventory := Inv{nodeAddress, kind, items}
	payload := GobEncode(inventory)
	request := append(CmdToBytes("inv"), payload...)

	SendData(address, request)
}

// SendGetBlocks 发送获取区块请求
func SendGetBlocks(address string) {
	payload := GobEncode(GetBlocks{nodeAddress})
	request := append(CmdToBytes("getblocks"), payload...)

	SendData(address, request)
}

// SendGetData 发送获取数据请求（区块或交易）
func SendGetData(address, kind string, id []byte) {
	payload := GobEncode(GetData{nodeAddress, kind, id})
	request := append(CmdToBytes("getdata"), payload...)

	SendData(address, request)
}

// SendTx 发送交易数据
func SendTx(addr string, tnx *blockchain.Transaction) {
	data := Tx{nodeAddress, tnx.Serialize()}
	payload := GobEncode(data)
	request := append(CmdToBytes("tx"), payload...)

	SendData(addr, request)
}

// SendVersion 发送版本信息
func SendVersion(addr string, chain *blockchain.BlockChain) {
	bestHeight := chain.GetBestHeight()
	payload := GobEncode(Version{version, bestHeight, nodeAddress})

	request := append(CmdToBytes("version"), payload...)

	SendData(addr, request)
}

// HandleAddr 处理节点地址请求
func HandleAddr(request []byte) {
	var buff bytes.Buffer
	var payload Addr

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)

	}

	KnownNodes = append(KnownNodes, payload.AddrList...)
	fmt.Printf("there are %d known nodes\n", len(KnownNodes))
	RequestBlocks()
}

// HandleBlock 处理区块请求
func HandleBlock(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload Block

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blockData := payload.Block
	block := blockchain.Deserialize(blockData)

	fmt.Println("Recevied a new block!")
	chain.AddBlock(block)

	fmt.Printf("Added block %x\n", block.Hash)

	if len(blocksInTransit) > 0 {
		blockHash := blocksInTransit[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		blocksInTransit = blocksInTransit[1:]
	} else {
		UTXOSet := blockchain.UTXOSet{Blockchain: chain}
		UTXOSet.Reindex()
	}
}

// HandleInv 处理库存请求（区块或交易）
func HandleInv(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload Inv

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	fmt.Printf("Recevied inventory with %d %s\n", len(payload.Items), payload.Type)

	if payload.Type == "block" {
		blocksInTransit = payload.Items

		blockHash := payload.Items[0]
		SendGetData(payload.AddrFrom, "block", blockHash)

		newInTransit := [][]byte{}
		for _, b := range blocksInTransit {
			if !bytes.Equal(b, blockHash) {
				newInTransit = append(newInTransit, b)
			}
		}
		blocksInTransit = newInTransit
	}

	if payload.Type == "tx" {
		txID := payload.Items[0]

		if memoryPool[hex.EncodeToString(txID)].ID == nil {
			SendGetData(payload.AddrFrom, "tx", txID)
		}
	}
}

// HandleGetBlocks 处理获取区块请求
func HandleGetBlocks(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload GetBlocks

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	blocks := chain.GetBlockHashes()
	SendInv(payload.AddrFrom, "block", blocks)
}

// HandleGetData 处理获取数据请求（区块或交易）
func HandleGetData(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload GetData

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	if payload.Type == "block" {
		block, err := chain.GetBlock([]byte(payload.ID))
		if err != nil {
			return
		}

		SendBlock(payload.AddrFrom, &block)
	}

	if payload.Type == "tx" {
		txID := hex.EncodeToString(payload.ID)
		tx := memoryPool[txID]

		SendTx(payload.AddrFrom, &tx)
	}
}

// HandleTx 处理交易数据请求
func HandleTx(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload Tx

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	txData := payload.Transaction
	tx := blockchain.DeserializeTransaction(txData)
	memoryPool[hex.EncodeToString(tx.ID)] = tx

	fmt.Printf("%s, %d", nodeAddress, len(memoryPool))

	if nodeAddress == KnownNodes[0] {
		for _, node := range KnownNodes {
			if node != nodeAddress && node != payload.AddrFrom {
				SendInv(node, "tx", [][]byte{tx.ID})
			}
		}
	} else {
		if len(memoryPool) >= 2 && len(mineAddress) > 0 {
			MineTx(chain)
		}
	}
}

// MineTx 挖掘新区块
func MineTx(chain *blockchain.BlockChain) {
	// 从内存池中获取有效交易并生成新区块
	var txs []*blockchain.Transaction

	for id := range memoryPool {
		fmt.Printf("tx: %s\n", memoryPool[id].ID)
		tx := memoryPool[id]
		if chain.VerifyTransaction(&tx) {
			txs = append(txs, &tx)
		}
	}

	if len(txs) == 0 {
		fmt.Println("All Transactions are invalid")
		return
	}

	cbTx := blockchain.CoinbaseTx(mineAddress, "")
	txs = append(txs, cbTx)

	newBlock := chain.MineBlock(txs)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	fmt.Println("New Block mined")

	for _, tx := range txs {
		txID := hex.EncodeToString(tx.ID)
		delete(memoryPool, txID)
	}

	for _, node := range KnownNodes {
		if node != nodeAddress {
			SendInv(node, "block", [][]byte{newBlock.Hash})
		}
	}

	if len(memoryPool) > 0 {
		MineTx(chain)
	}
}

// HandleVersion 处理版本信息请求
func HandleVersion(request []byte, chain *blockchain.BlockChain) {
	var buff bytes.Buffer
	var payload Version

	buff.Write(request[commandLength:])
	dec := gob.NewDecoder(&buff)
	err := dec.Decode(&payload)
	if err != nil {
		log.Panic(err)
	}

	bestHeight := chain.GetBestHeight()
	otherHeight := payload.BestHeight

	if bestHeight < otherHeight {
		SendGetBlocks(payload.AddrFrom)
	} else if bestHeight > otherHeight {
		SendVersion(payload.AddrFrom, chain)
	}

	if !NodeIsKnown(payload.AddrFrom) {
		KnownNodes = append(KnownNodes, payload.AddrFrom)
	}
}

// HandleConnection 处理节点之间的连接
func HandleConnection(conn net.Conn, chain *blockchain.BlockChain) {
	// 从连接中读取所有数据
	req, err := ioutil.ReadAll(conn)
	defer conn.Close() // 确保连接关闭

	if err != nil {
		log.Panic(err) // 如果发生错误，输出日志并终止程序
	}

	// 提取请求中的命令
	command := BytesToCmd(req[:commandLength])
	fmt.Printf("Received %s command\n", command)

	// 根据命令调用对应的处理函数
	switch command {
	case "addr": // 处理地址信息
		HandleAddr(req)
	case "block": // 处理区块信息
		HandleBlock(req, chain)
	case "inv": // 处理库存信息
		HandleInv(req, chain)
	case "getblocks": // 处理获取区块请求
		HandleGetBlocks(req, chain)
	case "getdata": // 处理获取数据请求
		HandleGetData(req, chain)
	case "tx": // 处理交易信息
		HandleTx(req, chain)
	case "version": // 处理版本信息
		HandleVersion(req, chain)
	default:
		fmt.Println("Unknown command") // 未知命令
	}
}

// StartServer 启动区块链节点服务器
func StartServer(nodeID, minerAddress string) {
	// 设置节点地址和矿工地址
	nodeAddress = fmt.Sprintf("localhost:%s", nodeID)
	mineAddress = minerAddress

	// 监听指定协议和地址
	ln, err := net.Listen(protocol, nodeAddress)
	if err != nil {
		log.Panic(err) // 如果监听失败，输出日志并终止程序
	}
	defer ln.Close() // 确保监听关闭

	// 加载或创建区块链
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close() // 确保区块链数据库关闭

	go CloseDB(chain) // 设置程序关闭时的清理函数

	// 如果当前节点不是主节点，发送版本信息到主节点
	if nodeAddress != KnownNodes[0] {
		SendVersion(KnownNodes[0], chain)
	}

	// 无限循环，处理传入的连接
	for {
		conn, err := ln.Accept() // 接受传入的连接
		if err != nil {
			log.Panic(err)
		}
		go HandleConnection(conn, chain) // 使用 goroutine 异步处理连接
	}
}

// GobEncode 将数据编码为 Gob 格式
func GobEncode(data interface{}) []byte {
	var buff bytes.Buffer

	enc := gob.NewEncoder(&buff)
	err := enc.Encode(data) // 编码数据
	if err != nil {
		log.Panic(err) // 如果编码失败，输出日志并终止程序
	}

	return buff.Bytes() // 返回编码后的字节数组
}

// NodeIsKnown 检查节点是否已经在已知节点列表中
func NodeIsKnown(addr string) bool {
	for _, node := range KnownNodes {
		if node == addr {
			return true // 如果地址在列表中，返回 true
		}
	}
	return false // 否则返回 false
}

// CloseDB 设置程序退出时关闭区块链数据库
func CloseDB(chain *blockchain.BlockChain) {
	// 创建 Death 对象，用于捕获退出信号
	d := death.NewDeath(syscall.SIGINT, syscall.SIGTERM, os.Interrupt)

	// 等待退出信号并执行清理操作
	d.WaitForDeathWithFunc(func() {
		defer os.Exit(1) // 确保程序退出
		defer runtime.Goexit() // 确保所有 Goroutine 退出
		chain.Database.Close() // 关闭数据库
	})
}
