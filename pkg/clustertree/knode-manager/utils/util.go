package utils

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	kosmosversioned "github.com/kosmos.io/kosmos/pkg/generated/clientset/versioned"
)

type Handlers func(*rest.Config)

func NewClientFromConfigPath(configPath string, opts ...Handlers) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func NewKosmosClientFromConfigPath(configPath string, opts ...Handlers) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)
	config, err = clientcmd.BuildConfigFromFlags("", configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to build config from configpath: %v", err)
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("could not create clientset: %v", err)
	}
	return client, nil
}

func NewClientFromBytes(kubeConfig []byte, opts ...Handlers) (kubernetes.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}

func NewKosmosClientFromBytes(kubeConfig []byte, opts ...Handlers) (kosmosversioned.Interface, error) {
	var (
		config *rest.Config
		err    error
	)

	clientConfig, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = clientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, opt := range opts {
		if opt == nil {
			continue
		}
		opt(config)
	}

	client, err := kosmosversioned.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %v", err)
	}
	return client, nil
}
