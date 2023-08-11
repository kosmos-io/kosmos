package utils

import (
	"fmt"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// nolint
func ContainsString(arr []string, s string) bool {
	for _, str := range arr {
		if strings.Contains(str, s) {
			return true
		}
	}
	return false
}

func CompareStringArrays(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BuildClusterConfig(configBytes []byte) (*rest.Config, error) {
	if len(configBytes) == 0 {
		return nil, fmt.Errorf("config bytes is nil")
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(configBytes)
	if err != nil {
		return nil, err
	}
	return clientConfig.ClientConfig()
}

func BuildClusterClient(configBytes []byte) (*kubernetes.Clientset, error) {
	if len(configBytes) == 0 {
		return nil, fmt.Errorf("config bytes is nil")
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(configBytes)
	if err != nil {
		return nil, err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	clusterClientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clusterClientSet, nil
}

func BuildDynamicClient(configBytes []byte) (dynamic.Interface, error) {
	if len(configBytes) == 0 {
		return nil, fmt.Errorf("config bytes is nil")
	}
	clientConfig, err := clientcmd.NewClientConfigFromBytes(configBytes)
	if err != nil {
		return nil, err
	}
	config, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

func IsIPv6(s string) bool {
	// 0.234.63.0 and 0.234.63.0/22
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.':
			return false
		case ':':
			return true
		}
	}
	return false
}
