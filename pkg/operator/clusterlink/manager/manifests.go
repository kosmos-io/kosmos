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
              cpu: 500m
              memory: 500Mi
            requests:
              cpu: 500m
              memory: 500Mi
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
            exec:
              command:
                - cat
                - /proc/1/cmdline
            failureThreshold: 30
            initialDelaySeconds: 3
            periodSeconds: 10
            successThreshold: 1
            timeoutSeconds: 3
          volumeMounts:
            - mountPath: /etc/clusterlink
              name: proxy-config
              readOnly: true
      serviceAccountName: {{ .Name }}
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - clusterlink-controller-manager
            namespaces:
              - {{ .Namespace }}
            topologyKey: kubernetes.io/hostname
      tolerations:
        - effect: NoSchedule
          key: kosmos.io/join
          operator: Equal
          value: "true"
      volumes:
        - name: proxy-config
          secret:
            defaultMode: 420
            secretName: {{ .ProxySecretName }}
`

type DeploymentReplace struct {
	Namespace       string
	Name            string
	ProxySecretName string
	ClusterName     string
	ImageRepository string
	Version         string
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
