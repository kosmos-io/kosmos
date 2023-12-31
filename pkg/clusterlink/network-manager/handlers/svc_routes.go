package handlers

type ServiceRoutes struct {
	Next
}

func (h *ServiceRoutes) Do(c *Context) (err error) {
	gwNodes := c.Filter.GetGatewayNodes()

	for _, target := range gwNodes {
		cluster := c.Filter.GetClusterByName(target.Spec.ClusterName)
		serviceCIDRs := cluster.Status.ClusterLinkStatus.ServiceCIDRs

		serviceCIDRs = FilterByIPFamily(serviceCIDRs, cluster.Spec.ClusterLinkOptions.IPFamily)
		serviceCIDRs = ConvertToGlobalCIDRs(serviceCIDRs, cluster.Spec.ClusterLinkOptions.GlobalCIDRsMap)
		BuildRoutes(c, target, serviceCIDRs)
	}

	return nil
}
