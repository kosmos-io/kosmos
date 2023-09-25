package manifest

const (
	ClusterlinkNetworkManagerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-network-manager
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-network-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-network-manager
  template:
    metadata:
      labels:
        app: clusterlink-network-manager
    spec:
      serviceAccountName: clusterlink-network-manager
      containers:
        - name: manager
          image: {{ .ImageRepository }}/clusterlink-network-manager:v{{ .Version }}
          imagePullPolicy: IfNotPresent
          command:
            - clusterlink-network-manager
            - v=4
          resources:
            limits:
              memory: 500Mi
              cpu: 500m
            requests:
              cpu: 500m
              memory: 500Mi
`

	ClusterlinkDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-operator
  namespace: clusterlink-system
  labels:
    app: operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: operator
  template:
    metadata:
      labels:
        app: operator
    spec:
      serviceAccountName: clusterlink-operator
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            - labelSelector:
                matchExpressions:
                  - key: app
                    operator: In
                    values:
                      - operator
              namespaces:
                - clusterlink-system
              topologyKey: kubernetes.io/hostname
      containers:
      - name: operator
        image: ghcr.io/kosmos-io/clusterlink-operator:v{{ .Version }}
        imagePullPolicy: IfNotPresent
        command:
          - clusterlink-operator
          - --controlpanelconfig=/etc/clusterlink/kubeconfig
        resources:
          limits:
            memory: 200Mi
            cpu: 250m
          requests:
            cpu: 100m
            memory: 200Mi
        env:
        - name: VERSION
          value: v{{ .Version }}
        - name: CLUSTER_NAME
          value: {{ .ClusterName }}
        - name: USE_PROXY
          value: "true"
        volumeMounts:
          - mountPath: /etc/clusterlink
            name: proxy-config
            readOnly: true
      volumes:
        - name: proxy-config
          secret:
            secretName: controlpanel-config

`

	ClusterRouterKnodeDeployment = `---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: knode
  namespace: {{ .Namespace }}
  labels:
    app: clusterrouter-controller-manager
spec:
  replicas: 1
  selector:
    matchLabels:
      app: knode
  template:
    metadata:
      labels:
        app: knode
    spec:
      serviceAccountName: clusterrouter-knode
      containers:
        - name: manager
          image: {{ .ImageRepository }}/knode:v{{ .Version }}
          imagePullPolicy: Always
          command:
            - /clusterrouter
            - --kube-api-qps=500
            - --kube-api-burst=1000
            - --kubeconfig=/etc/kube/config
            - --leader-elect=false
          volumeMounts:
            - mountPath: /etc/kube
              name: config-volume
              readOnly: true
      volumes:
        - configMap:
            defaultMode: 420
            items:
              - key: Kubeconfig
                path: Kubeconfig
            name: host-kubeconfig
          name: config-volume
`
)

type DeploymentReplace struct {
	Namespace       string
	ImageRepository string
	Version         string
}

type ClusterlinkDeploymentReplace struct {
	Version     string
	ClusterName string
}
