package handlers

import (
	"github.com/kosmos.io/kosmos/pkg/apis/kosmos/v1alpha1"
	"github.com/kosmos.io/kosmos/pkg/clusterlink/network-manager/helpers"
)

// Context Context
type Context struct {
	Filter *helpers.Filter

	Results map[string]*NodeConfig
}

func (c *Context) GetDeviceFromResults(nodeName string, devName string) *v1alpha1.Device {
	config, ok := c.Results[nodeName]
	if !ok {
		return nil
	}
	for i, dev := range config.Devices {
		if dev.Name == devName {
			return &config.Devices[i]
		}
	}
	return nil
}
