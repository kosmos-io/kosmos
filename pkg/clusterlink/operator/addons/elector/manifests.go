package elector

const clusterlinkElectorServiceAccount = `
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

const clusterlinkElectorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    app: elector
spec:
  replicas: 2
  selector:
    matchLabels:
      app: elector
  template:
    metadata:
      labels:
        app: elector
    spec:
      serviceAccountName: {{ .Name }}
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchExpressions:
              - key: app
                operator: In
                values:
                - elector
            namespaces:
            - clusterlink-system
            topologyKey: kubernetes.io/hostname
      containers:
      - name: elector
        readinessProbe:
          exec:
            command:
            - cat
            - /proc/1/cmdline
          failureThreshold: 30
          initialDelaySeconds: 3
          periodSeconds: 10
          timeoutSeconds: 5
        livenessProbe:
          failureThreshold: 30
          exec:
            command:
            - cat
            - /proc/1/cmdline
          initialDelaySeconds: 3
          periodSeconds: 10
          successThreshold: 1
          timeoutSeconds: 3
        image: {{ .ImageRepository }}/clusterlink-elector:{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - clusterlink-elector
          - --controlpanelconfig=/etc/clusterlink/kubeconfig
          - --v=3 
        env:
        - name: CLUSTER_NAME
          value: "{{ .ClusterName }}"  
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        volumeMounts:
        - mountPath: /etc/clusterlink
          name: proxy-config
          readOnly: true
      tolerations:
      - key: "key"
        operator: "Equal"
        value: "value"
        effect: "NoSchedule"
      volumes:
      - name: proxy-config
        secret:
          secretName: {{ .ProxyConfigMapName }}
`

type DeploymentReplace struct {
	Namespace          string
	Name               string
	ClusterName        string
	ImageRepository    string
	ProxyConfigMapName string
	Version            string
}

const clusterlinkElectorClusterRole = `
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

const clusterlinkElectorClusterRoleBinding = `
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
