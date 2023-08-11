package manager

const clusterlinkManagerServiceAccount = `
apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
`

type ServiceAccountReplace struct {
	Namespace string
	Name      string
}

const clusterlinkManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-controller-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-controller-manager
  template:
    metadata:
      labels:
        app: clusterlink-controller-manager
    spec:
      serviceAccountName: {{ .Name }}
      containers:
        - name: manager
          image: {{ .ImageRepository }}/clusterlink-controller-manager:{{ .Version }}
          imagePullPolicy: IfNotPresent
          command:
            - clusterlink-controller-manager
            - --controlpanelconfig=/etc/clusterlink/kubeconfig
          env:
          - name: CLUSTER_NAME
            value: "{{ .ClusterName }}"  
          resources:
            limits:
              memory: 500Mi
              cpu: 500m
            requests:
              cpu: 500m
              memory: 500Mi
          volumeMounts:
            - mountPath: /etc/clusterlink
              name: proxy-config
              readOnly: true
      volumes:
      - name: proxy-config
        secret:
          secretName: {{ .ProxyConfigMapName }}
`

type DeploymentReplace struct {
	Namespace          string
	Name               string
	ProxyConfigMapName string
	ClusterName        string
	ImageRepository    string
	Version            string
}

const clusterlinkManagerClusterRole = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: {{ .Name }}
rules:
  - apiGroups: ['*']
    resources: ['*']
    verbs: ["*"]
  - nonResourceURLs: ['*']
    verbs: ["get"]
`

type ClusterRoleReplace struct {
	Name string
}

const clusterlinkManagerClusterRoleBinding = `
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: {{ .Name }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: {{ .Name }}
subjects:
  - kind: ServiceAccount
    name: {{ .Name }}
    namespace: {{ .Namespace }}
`

type ClusterRoleBindingReplace struct {
	Name      string
	Namespace string
}
