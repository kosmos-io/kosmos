package host

const (
	CoreDNSSA = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: {{ .Namespace }}
`

	CoreDNSClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:coredns-{{ .Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns-{{ .Name }}
subjects:
- kind: ServiceAccount
  name: coredns
  namespace: {{ .Namespace }}
`

	CoreDNSClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:coredns-{{ .Name }}
rules:
- apiGroups:
  - ""
  resources:
  - endpoints
  - services
  - pods
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - discovery.k8s.io
  resources:
  - endpointslices
  verbs:
  - list
  - watch
`
)
