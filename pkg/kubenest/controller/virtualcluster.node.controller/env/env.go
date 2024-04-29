package util

import (
	"fmt"
	"os"

	"k8s.io/klog"
)

func GetExectorTmpPath() string {
	tmpPath := os.Getenv("EXECTOR_TMP_PATH")
	if len(tmpPath) == 0 {
		tmpPath = "/apps/conf/kosmos/tmp"
	}
	return tmpPath
}

func GetExectorWorkerDir() string {
	exectorWorkDir := os.Getenv("EXECTOR_WORKER_PATH")
	if len(exectorWorkDir) == 0 {
		exectorWorkDir = "/etc/vc-node-dir/"
	}
	return exectorWorkDir
}

func GetExectorShellName() string {
	shellName := os.Getenv("EXECTOR_SHELL_VERSION")

	if len(shellName) == 0 {
		shellName = "kubelet_node_helper.sh"
	}
	return shellName
}

func GetExectorShellPath() string {
	exectorWorkDir := GetExectorWorkerDir()
	shellVersion := GetExectorShellName()

	return fmt.Sprintf("%s%s", exectorWorkDir, shellVersion)
}

func GetExectorHostMasterNodeIP() string {
	hostIP := os.Getenv("EXECTOR_HOST_MASTER_NODE_IP")
	if len(hostIP) == 0 {
		klog.Fatal("EXECTOR_HOST_MASTER_NODE_IP is none")
	}
	return hostIP
}

// tobke = base64(`username:password`)
func GetExectorToken() string {
	token := os.Getenv("EXECTOR_SHELL_TOKEN")
	if len(token) == 0 {
		klog.Fatal("EXECTOR_SHELL_TOKEN is none")
	}
	return token
}

func GetExectorPort() string {
	exectorPort := os.Getenv("EXECTOR_SERVER_PORT")
	if len(exectorPort) == 0 {
		exectorPort = "5678"
	}
	return exectorPort
}
