package manifest

const (
	ClusterlinkNetworkManagerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clusterlink-network-manager
  namespace: {{ .Namespace }}
`

	ClusterlinkFloaterServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-floater
  namespace: {{ .Namespace }}
`

	ClusterlinkServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-operator
  namespace: clusterlink-system
`

	ClusterRouterKnodeServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clusterrouter-knode
  namespace: {{ .Namespace }}
`
)

type ServiceAccountReplace struct {
	Namespace string
}
