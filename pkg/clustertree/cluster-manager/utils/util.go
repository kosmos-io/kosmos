package utils

import (
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Handlers func(*rest.Config)

func NewConfigFromBytes(kubeConfig []byte, handlers ...Handlers) (*rest.Config, error) {
	var (
		config *rest.Config
		err    error
	)

	c, err := clientcmd.NewClientConfigFromBytes(kubeConfig)
	if err != nil {
		return nil, err
	}
	config, err = c.ClientConfig()
	if err != nil {
		return nil, err
	}

	for _, h := range handlers {
		if h == nil {
			continue
		}
		h(config)
	}

	return config, nil
}
