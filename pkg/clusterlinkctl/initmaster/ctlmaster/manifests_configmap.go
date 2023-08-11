package ctlmaster

import (
	"fmt"
	"os"

	"k8s.io/klog/v2"
)

func ReadKubeconfigFile(fileName string) (string, error) {
	f, err := os.ReadFile(fileName)
	if err != nil {
		klog.Errorf("Read kubeconfig file failed:%v", err)
		return "", fmt.Errorf("read kubeconfig file failed: %w", err)
	}
	return string(f), nil
}
