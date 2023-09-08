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
          image: {{ .ImageRepository }}/clusterlink/clusterlink-network-manager:{{ .Version }}
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
)

type DeploymentReplace struct {
	Namespace       string
	ImageRepository string
	Version         string
}
