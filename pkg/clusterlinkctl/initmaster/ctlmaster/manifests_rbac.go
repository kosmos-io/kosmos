package ctlmaster

const clusterlinkClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: clusterlink
rules:
- apiGroups:
  - '*'
  resources:
  - '*'
  verbs:
  - '*'
- nonResourceURLs:
  - '*'
  verbs:
  - get
`
const clusterlinkClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: clusterlink
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: clusterlink
subjects:
  - kind: ServiceAccount
    name: clusterlink-controller-manager
    namespace: {{ .Namespace }}
  - kind: ServiceAccount
    name: clusterlink-operator
    namespace: {{ .Namespace }}
`

const clusterlinkOperatorServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-operator
  namespace: {{ .Namespace }}
`

const clusterlinkControllerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-controller-manager
  namespace: {{ .Namespace }}
`

type RBACStuctNull struct {
}

type ClusterRoleBindingReplace struct {
	Namespace string
}

type ServiceAccountReplace struct {
	Namespace string
}
