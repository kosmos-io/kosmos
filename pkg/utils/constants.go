package utils

// cluster node
const (
	KosmosNodeLabel = "kosmos.io/node"
	KosmosNodeValue = "true"

	// KosmosResourceOwnersAnnotations on resorce (pv, configmap, secret), represents which cluster this resource belongs to
	KosmosResourceOwnersAnnotations = "kosmos-io/cluster-owners"
	// KosmosNodeOwnedByClusterAnnotations on node, represents which cluster this node belongs to
	KosmosNodeOwnedByClusterAnnotations = "kosmos-io/owned-by-cluster"
)
