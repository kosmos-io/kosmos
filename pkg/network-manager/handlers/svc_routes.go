package handlers

type ServiceRoutes struct {
	Next
}

func (h *ServiceRoutes) Do(c *Context) (err error) {
	gwNodes := c.Filter.GetGatewayNodes()

	for _, target := range gwNodes {
		cluster := c.Filter.GetClusterByName(target.Spec.ClusterName)
		serviceCIDRs := cluster.Status.ServiceCIDRs

		BuildRoutes(c, target, serviceCIDRs)
	}

	return nil
}
