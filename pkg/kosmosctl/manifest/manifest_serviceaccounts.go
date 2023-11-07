package manifest

const (
	KosmosOperatorServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  kosmos-operator
  namespace: {{ .Namespace }}
`

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

	ClusterTreeServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: clustertree
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
