package cli

import (
	"flag"
	"fmt"
	"log"
	"os"
	"runtime"
	"strconv"

	"github.com/xuanle1016/golang-blockchain/blockchain"
	"github.com/xuanle1016/golang-blockchain/network"
	"github.com/xuanle1016/golang-blockchain/wallet"
)

// CommandLine 结构体，表示命令行接口
type CommandLine struct{}

// 打印可用命令的用法
func (cli *CommandLine) printUsage() {
	fmt.Println("Usage:")
	fmt.Println(" getbalance -address ADDRESS - 获取某地址的余额")
	fmt.Println(" createblockchain -address ADDRESS 创建区块链，并将创世奖励发送到指定地址")
	fmt.Println(" printchain - 打印区块链中的所有区块")
	fmt.Println(" send -from FROM -to TO -amount AMOUNT -mine - 发送一定金额的币。如果设置-mine标志，将在本地立即挖矿")
	fmt.Println(" createwallet - 创建一个新的钱包")
	fmt.Println(" listaddresses - 列出钱包文件中的所有地址")
	fmt.Println(" reindexutxo - 重建UTXO集合")
	fmt.Println(" startnode -miner ADDRESS - 使用指定的NODE_ID启动一个节点。-miner 启用挖矿功能并设置奖励地址")
}

// 验证命令行参数是否合法
func (cli *CommandLine) validateArgs() {
	if len(os.Args) < 2 {
		cli.printUsage()
		runtime.Goexit()
	}
}

// 启动节点，并可选择启用挖矿功能
func (cli *CommandLine) StartNode(nodeID, minerAddress string) {
	fmt.Printf("Starting node %s\n", nodeID)

	if len(minerAddress) > 0 {
		if wallet.ValidateAddress(minerAddress) {
			fmt.Println("挖矿已启用。奖励地址: ", minerAddress)
		} else {
			log.Panic("无效的矿工地址!")
		}
	}

	network.StartServer(nodeID, minerAddress)
}

// 重建UTXO集合
func (cli *CommandLine) reindexUTXO(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	count := UTXOSet.CountTransactions()
	fmt.Printf("完成! 当前UTXO集合包含 %d 笔交易.\n", count)
}

// 列出钱包文件中的所有地址
func (cli *CommandLine) listAddresses(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	addresses := wallets.GetAllAddress()

	for _, address := range addresses {
		fmt.Println(address)
	}
}

// 创建新的钱包地址
func (cli *CommandLine) createWallet(nodeID string) {
	wallets, _ := wallet.CreateWallets(nodeID)
	address := wallets.AddWallet()
	wallets.SaveFile(nodeID)

	fmt.Printf("新的地址: %s\n", address)
}

// 打印区块链中所有区块信息
func (cli *CommandLine) printChain(nodeID string) {
	chain := blockchain.ContinueBlockChain(nodeID)
	defer chain.Database.Close()
	iter := chain.Iterator()

	for {
		block := iter.Next()

		fmt.Printf("前一区块哈希: %x\n", block.PrevHash)
		fmt.Printf("当前区块哈希: %x\n", block.Hash)

		pow := blockchain.NewProof(block)
		fmt.Printf("工作量证明: %s\n", strconv.FormatBool(pow.Validate()))

		for _, tx := range block.Transactions {
			fmt.Println(tx)
		}
		fmt.Println()

		if len(block.PrevHash) == 0 {
			break
		}
	}
}

// 创建区块链并生成创世区块
func (cli *CommandLine) createBlockChain(address string, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("地址无效")
	}

	chain := blockchain.InitBlockChain(address, nodeID)
	defer chain.Database.Close()

	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	UTXOSet.Reindex()

	fmt.Println("创建完成!")
}

// 查询指定地址的余额
func (cli *CommandLine) getBalance(address, nodeID string) {
	if !wallet.ValidateAddress(address) {
		log.Panic("地址无效")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	defer chain.Database.Close()

	balance := 0
	pubKeyHash := wallet.Base58Decode([]byte(address))
	pubKeyHash = pubKeyHash[1 : len(pubKeyHash)-4]
	fmt.Printf("地址的公钥哈希: %x\n", pubKeyHash)

	UTXOs := UTXOSet.FindUnspentTransactions(pubKeyHash)
	fmt.Printf("地址的UTXOs: %+v\n", UTXOs)

	for _, out := range UTXOs {
		balance += out.Value
	}

	fmt.Printf("地址 %s 的余额: %d\n", address, balance)
}

// 发送交易
func (cli *CommandLine) send(from, to string, amount int, nodeID string, mineNow bool) {
	if !wallet.ValidateAddress(to) {
		log.Panic("地址无效")
	}

	if !wallet.ValidateAddress(from) {
		log.Panic("地址无效")
	}

	chain := blockchain.ContinueBlockChain(nodeID)
	UTXOSet := blockchain.UTXOSet{Blockchain: chain}
	defer chain.Database.Close()

	wallets, err := wallet.CreateWallets(nodeID)
	if err != nil {
		log.Panic(err)
	}
	wallet := wallets.GetWallet(from)

	tx := blockchain.NewTransaction(&wallet, to, amount, &UTXOSet)
	if mineNow {
		cbTx := blockchain.CoinbaseTx(from, "")
		txs := []*blockchain.Transaction{cbTx, tx}
		block := chain.MineBlock(txs)
		UTXOSet.Update(block)
	} else {
		network.SendTx(network.KnownNodes[0], tx)
		fmt.Println("交易已发送")
	}

	fmt.Println("发送成功!")
}

// 解析命令行输入并执行对应的功能
func (cli *CommandLine) Run() {
	cli.validateArgs()

	nodeID := os.Getenv("NODE_ID")
	if nodeID == "" {
		fmt.Printf("NODE_ID 未设置!")
		runtime.Goexit()
	}

	// 定义命令和参数
	getBalanceCmd := flag.NewFlagSet("getbalance", flag.ExitOnError)
	createBlockchainCmd := flag.NewFlagSet("createblockchain", flag.ExitOnError)
	sendCmd := flag.NewFlagSet("send", flag.ExitOnError)
	printChainCmd := flag.NewFlagSet("printchain", flag.ExitOnError)
	createWalletCmd := flag.NewFlagSet("createwallet", flag.ExitOnError)
	listAddressesCmd := flag.NewFlagSet("listaddresses", flag.ExitOnError)
	reindexUTXOCmd := flag.NewFlagSet("reindexutxo", flag.ExitOnError)
	startNodeCmd := flag.NewFlagSet("startnode", flag.ExitOnError)

	// 设置命令的参数
	getBalanceAddress := getBalanceCmd.String("address", "", "获取余额的地址")
	createBlockchainAddress := createBlockchainCmd.String("address", "", "接收创世块奖励的地址")
	sendFrom := sendCmd.String("from", "", "发送方地址")
	sendTo := sendCmd.String("to", "", "接收方地址")
	sendAmount := sendCmd.Int("amount", 0, "发送金额")
	sendMine := sendCmd.Bool("mine", false, "是否在本地立即挖矿")
	startNodeMiner := startNodeCmd.String("miner", "", "启用挖矿模式并设置奖励地址")

	// 解析命令
	switch os.Args[1] {
	case "reindexutxo":
		err := reindexUTXOCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "getbalance":
		err := getBalanceCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "startnode":
		err := startNodeCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createblockchain":
		err := createBlockchainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "listaddresses":
		err := listAddressesCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "createwallet":
		err := createWalletCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "printchain":
		err := printChainCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	case "send":
		err := sendCmd.Parse(os.Args[2:])
		if err != nil {
			log.Panic(err)
		}
	default:
		cli.printUsage()
		runtime.Goexit()
	}

	// 根据解析结果执行相应命令
	if getBalanceCmd.Parsed() {
		if *getBalanceAddress == "" {
			getBalanceCmd.Usage()
			runtime.Goexit()
		}
		cli.getBalance(*getBalanceAddress, nodeID)
	}

	if createBlockchainCmd.Parsed() {
		if *createBlockchainAddress == "" {
			createBlockchainCmd.Usage()
			runtime.Goexit()
		}
		cli.createBlockChain(*createBlockchainAddress, nodeID)
	}

	if printChainCmd.Parsed() {
		cli.printChain(nodeID)
	}

	if createWalletCmd.Parsed() {
		cli.createWallet(nodeID)
	}
	if listAddressesCmd.Parsed() {
		cli.listAddresses(nodeID)
	}

	if reindexUTXOCmd.Parsed() {
		cli.reindexUTXO(nodeID)
	}

	if sendCmd.Parsed() {
		if *sendFrom == "" || *sendTo == "" || *sendAmount <= 0 {
			sendCmd.Usage()
			runtime.Goexit()
		}
		cli.send(*sendFrom, *sendTo, *sendAmount, nodeID, *sendMine)
	}

	if startNodeCmd.Parsed() {
		cli.StartNode(nodeID, *startNodeMiner)
	}
}
