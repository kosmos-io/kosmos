package floaterclient

const clusterlinkFloaterDaemonSet = `
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: clusterlink-floater
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-floater
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-floater
  template:
    metadata:
      labels:
        app: clusterlink-floater
    spec:
      hostNetwork: {{ .EnableHostNetwork }}
      serviceAccountName: clusterlink-floater
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kosmos.io/exclude
                operator: DoesNotExist
      containers:
      - name: floater
        image: {{ .ImageRepository }}/clusterlink-floater:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - clusterlink-floater
        env: 
          - name: "PORT"
            value: "{{ .Port }}"
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - key: CriticalAddonsOnly
        operator: Exists
      - effect: NoExecute
        operator: Exists
`

const clusterlinkClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: clusterlink-floater
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

const clusterlinkFloaterServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name:  clusterlink-floater
  namespace: {{ .Namespace }}
`

type DaemonSetReplace struct {
	Namespace       string
	ImageRepository string
	Version         string
	DaemonSetName   string
	Port            string

	EnableHostNetwork bool `default:"false"`
}

type RBACReplace struct {
	Namespace string
}
