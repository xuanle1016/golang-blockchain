package main

import (
	"os"

	"github.com/xuanle1016/golang-blockchain/cli"
)

func main() {
	defer os.Exit(0)
	cmd := cli.CommandLine{}
	cmd.Run()
}
