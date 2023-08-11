package ctlmaster

const clusterlinkControllerDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-controller-manager
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
      serviceAccountName: clusterlink-controller-manager
      containers:
      - name: manager
        image: {{ .Imgae }}
        command:
          - clusterlink-controller-manager
`

const clusterlinkOperatorDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: clusterlink-operator
  namespace: {{ .Namespace }}
  labels:
    app: clusterlink-operator
spec:
  replicas: 1
  selector:
    matchLabels:
      app: clusterlink-operator
  template:
    metadata:
      labels:
        app: clusterlink-operator
    spec:
      serviceAccountName: clusterlink-operator
      containers:
      - name: operator
        image: {{ .Imgae }}
        command:
          - clusterlink-operator
        env:
        - name: VERSION
          value: {{ .Version }}
`

type DeploymentReplace struct {
	Namespace      string
	Imgae          string
	Version        string
	DeploymentName string
}

type KubeResourceInfo struct {
	Name           string
	Namespace      string
	ResourceClient KubeResourceToDel
	Type           string
}
