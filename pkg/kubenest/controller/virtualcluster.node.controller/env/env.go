package util

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"

	"k8s.io/klog"
)

func GetExectorTmpPath() string {
	tmpPath := os.Getenv("EXECTOR_TMP_PATH")
	if len(tmpPath) == 0 {
		tmpPath = "/apps/conf/kosmos/tmp"
	}
	return tmpPath
}

func GetKubeletKubeConfigName() string {
	kubeletKubeConfigName := os.Getenv("KUBELET_KUBE_CONFIG_NAME")
	if len(kubeletKubeConfigName) == 0 {
		// env.sh  KUBELET_KUBE_CONFIG_NAME
		kubeletKubeConfigName = "kubelet.conf"
	}
	return kubeletKubeConfigName
}

func GetKubeletConfigName() string {
	kubeletConfigName := os.Getenv("KUBELET_CONFIG_NAME")
	if len(kubeletConfigName) == 0 {
		// env.sh  KUBELET_CONFIG_NAME
		kubeletConfigName = "config.yaml"
	}
	return kubeletConfigName
}

func GetExectorWorkerDir() string {
	exectorWorkDir := os.Getenv("EXECTOR_WORKER_PATH")
	if len(exectorWorkDir) == 0 {
		exectorWorkDir = "/etc/vc-node-dir/"
	}
	return exectorWorkDir
}

func GetExectorShellName() string {
	shellName := os.Getenv("EXECTOR_SHELL_NAME")

	if len(shellName) == 0 {
		shellName = "kubelet_node_helper.sh"
	}
	return shellName
}

func GetExectorShellEnvName() string {
	shellName := os.Getenv("EXECTOR_SHELL_ENV_NAME")

	if len(shellName) == 0 {
		shellName = "env.sh"
	}
	return shellName
}

func GetExectorShellPath() string {
	exectorWorkDir := GetExectorWorkerDir()
	shellVersion := GetExectorShellName()

	return fmt.Sprintf("%s%s", exectorWorkDir, shellVersion)
}

func GetExectorShellEnvPath() string {
	exectorWorkDir := GetExectorWorkerDir()
	shellVersion := GetExectorShellEnvName()

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
	username := os.Getenv("WEB_USER")
	password := os.Getenv("WEB_PASS")
	token := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s", username, password)))
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

func GetDrainWaitSeconds() int {
	drainWaitSeconds := os.Getenv("EXECTOR_DRAIN_WAIT_SECONDS")
	if len(drainWaitSeconds) == 0 {
		drainWaitSeconds = "60"
	}
	num, err := strconv.Atoi(drainWaitSeconds)

	if err != nil {
		klog.Fatalf("convert EXECTOR_DRAIN_WAIT_SECONDS failed, err: %s", err)
	}

	return num
}

func GetControlPlaneLabel() string {
	controllPlaneLabel := os.Getenv("CONTROL_PLANE_LABEL")
	if len(controllPlaneLabel) == 0 {
		controllPlaneLabel = "node-role.kubernetes.io/control-plane"
	}
	return controllPlaneLabel
}
