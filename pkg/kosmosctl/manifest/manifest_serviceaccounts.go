package manifest

const (
	ClusterlinkNetworkManagerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clusterlink-network-manager
  namespace: {{ .Namespace }}
`

	ClusterTreeServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clustertree
  namespace: {{ .Namespace }}
`

	KosmosOperatorServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  kosmos-operator
  namespace: {{ .Namespace }}
`

	KosmosControlServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  kosmos-control
  namespace: {{ .Namespace }}
`

	ClusterlinkFloaterServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-floater
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
