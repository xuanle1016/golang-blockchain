package wallet

import (
	"log"

	"github.com/mr-tron/base58" // 引入 Base58 编码库
)

// Base58Encode 使用 Base58 编码算法对输入数据进行编码
// 参数: input - 需要编码的字节数组
// 返回值: 编码后的字节数组
func Base58Encode(input []byte) []byte {
	encode := base58.Encode(input) // 使用库函数进行 Base58 编码

	return []byte(encode) // 返回编码后的字节数组
}

// Base58Decode 使用 Base58 解码算法对输入数据进行解码
// 参数: input - Base58 编码的字节数组
// 返回值: 解码后的原始字节数组
// 如果解码失败，会记录错误日志并引发程序崩溃
func Base58Decode(input []byte) []byte {
	// 使用库函数进行 Base58 解码
	decode, err := base58.Decode(string(input[:]))
	if err != nil {
		log.Panic(err) // 如果解码失败，记录错误并退出程序
	}

	return decode // 返回解码后的字节数组
}


