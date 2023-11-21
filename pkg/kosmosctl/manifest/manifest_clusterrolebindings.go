package manifest

const (
	ClusterlinkNetworkManagerClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: clusterlink-network-manager
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: clusterlink-network-manager
subjects:
  - kind: ServiceAccount
    name: clusterlink-network-manager
    namespace: {{ .Namespace }}
`

	ClusterlinkFloaterClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: clusterlink-floater
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: clusterlink-floater
subjects:
  - kind: ServiceAccount
    name: clusterlink-floater
    namespace: {{ .Namespace }}
`

	KosmosClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kosmos
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kosmos
subjects: 
  - kind: ServiceAccount
    name: kosmos-control
    namespace: {{ .Namespace }}
  - kind: ServiceAccount
    name: clusterlink-controller-manager
    namespace: {{ .Namespace }}
  - kind: ServiceAccount
    name: kosmos-operator
    namespace: {{ .Namespace }}
`

	ClusterTreeClusterRoleBinding = `
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: clustertree
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: clustertree
subjects:
  - kind: ServiceAccount
    name: clustertree
    namespace: {{ .Namespace }}
`

	CorednsClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kosmos-coredns
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: kosmos-coredns
subjects:
  - kind: ServiceAccount
    name: coredns
    namespace: {{ .Namespace }}
`
)

type ClusterRoleBindingReplace struct {
	Namespace string
}
