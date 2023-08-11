package interfacepolicy

import (
	clusterlinkv1alpha1 "cnp.io/clusterlink/pkg/apis/clusterlink/v1alpha1"
	"cnp.io/clusterlink/pkg/utils"
)

func GetInterfaceName(networkInterfacePolicies []clusterlinkv1alpha1.NICNodeNames, nodeName string, defaultInterfaceName string) string {
	for _, policy := range networkInterfacePolicies {
		if len(policy.NodeName) > 0 && utils.ContainsString(policy.NodeName, nodeName) {
			return policy.InterfaceName
		}
	}
	return defaultInterfaceName
}
