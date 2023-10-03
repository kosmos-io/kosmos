package handlers

type InitNodes struct {
	Next
}

func (h *InitNodes) Do(c *Context) (err error) {
	clusterNodes := c.Filter.GetClusterNodes()
	results := make(map[string]*NodeConfig)
	for _, n := range clusterNodes {
		results[n.Name] = &NodeConfig{}
	}

	c.Results = results
	return
}
