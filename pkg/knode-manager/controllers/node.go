package controllers

import (
	"k8s.io/client-go/kubernetes"

	"github.com/kosmos.io/kosmos/pkg/knode-manager/adapters"
)

type NodeController struct {
	adapter adapters.NodeHandler
	client  kubernetes.Interface
}

func NewNodeController(adapter adapters.NodeHandler, client kubernetes.Interface) (*NodeController, error) {
	return &NodeController{
		adapter: adapter,
		client:  client,
	}, nil
}

func (n *NodeController) applyNode() error {
	return nil
}

func (n *NodeController) Run() error {
	err := n.applyNode()
	if err != nil {
		return err
	}

	return nil
}
