package main

import (
	"log"

	"github.com/kosmos.io/kosmos/cmd/kubenest/node-agent/app"
)

// RootCmd.Execute() 会根据用户输入调用子命令，比如 serve。
// 如果用户输入 serve，serveCmdRun 会被调用。
// serveCmdRun 调用了 Start，启动 WebSocket 服务并创建一个 Goroutine，用于发送心跳消息。
func main() {
	if err := app.RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
