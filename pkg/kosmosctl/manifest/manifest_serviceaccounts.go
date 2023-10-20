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

	ClusterlinkOperatorServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-operator
  namespace: {{ .Namespace }}
`

	ClusterTreeKnodeManagerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clustertree-cluster-manager
  namespace: {{ .Namespace }}
`

	CorednsServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: {{ .Namespace }}
`
)

type ServiceAccountReplace struct {
	Namespace string
}
