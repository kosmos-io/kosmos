package rbac

const (
	VirtualSchedulerSA = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: virtualcluster-scheduler
  namespace: "{{ .Namespace }}"
`
	VirtualSchedulerRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: virtual-scheduler
subjects:
  - kind: ServiceAccount
    name: virtualcluster-scheduler
    namespace: "{{ .Namespace }}"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: virtual-scheduler
`
	VirtualSchedulerRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: virtual-scheduler
rules:
  - verbs:
      - create
      - patch
      - update
    apiGroups:
      - ''
      - events.k8s.io
    resources:
      - events
  - verbs:
      - create
    apiGroups:
      - coordination.k8s.io
    resources:
      - leases
  - verbs:
      - get
      - update
    apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    resourceNames:
      - virtualcluster-scheduler
  - verbs:
      - create
    apiGroups:
      - ''
    resources:
      - endpoints
  - verbs:
      - get
      - update
    apiGroups:
      - ''
    resources:
      - endpoints
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - ''
    resources:
      - nodes
  - verbs:
      - delete
      - get
      - list
      - watch
    apiGroups:
      - ''
    resources:
      - pods
  - verbs:
      - create
    apiGroups:
      - ''
    resources:
      - bindings
      - pods/binding
  - verbs:
      - patch
      - update
    apiGroups:
      - ''
    resources:
      - pods/status
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - ''
    resources:
      - replicationcontrollers
      - services
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - apps
      - extensions
    resources:
      - replicasets
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - apps
    resources:
      - statefulsets
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - policy
    resources:
      - poddisruptionbudgets
  - verbs:
      - get
      - list
      - watch
      - update
    apiGroups:
      - ''
    resources:
      - persistentvolumeclaims
      - persistentvolumes
  - verbs:
      - create
    apiGroups:
      - authentication.k8s.io
    resources:
      - tokenreviews
  - verbs:
      - create
    apiGroups:
      - authorization.k8s.io
    resources:
      - subjectaccessreviews
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - storage.k8s.io
    resources:
      - '*'
  - verbs:
      - get
      - list
      - watch
    apiGroups:
      - ''
    resources:
      - configmaps
      - namespaces
`
)
