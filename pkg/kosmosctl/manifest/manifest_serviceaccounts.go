package manifest

const (
	KosmosControlServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  kosmos-control
  namespace: {{ .Namespace }}
`

	ClusterlinkOperatorServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-operator
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

	SchedulerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: kosmos-scheduler
  namespace: {{ .Namespace }}
`
)

type ServiceAccountReplace struct {
	Namespace string
}
