package heartbeat

import (
	"bytes"
	"log"
	"os/exec"
	"strings"
	"time"
)

// checkNodeAgentRunning 检查 node-agent 服务是否正在运行
func checkNodeAgentStatus() bool {
	cmd := exec.Command("systemctl", "is-active", "node-agent")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		return false
	}

	status := strings.TrimSpace(out.String())
	return status == "active"
}

// monitorHeartbeat 定时监控服务状态并输出心跳信息
func monitorHeartbeat() {
	ticker := time.NewTicker(30 * time.Second) // 每30秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if checkNodeAgentStatus() {
				log.Println("Heartbeat: node-agent is running.")
				// 将状态上报到 K8S（下一步实现）
			} else {
				log.Println("Heartbeat: node-agent is NOT running.")
				// 可能需要额外处理如重启服务等逻辑
			}
		}
	}
}
